/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	poisonv1 "github.com/adorigi/venom/api/v1"
	"github.com/go-logr/logr"
	kbatch "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/robfig/cron"
)

// ClusterScanReconciler reconciles a ClusterScan object
type ClusterScanReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const (
	scheduledAtAnnotation = "scheduled-at"
	customJobLabel        = "owned-by"
	jobOwnerKey           = ".metadata.controller"
)

//+kubebuilder:rbac:groups=poison.venom.gule-gulzar.com,resources=clusterscans,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=poison.venom.gule-gulzar.com,resources=clusterscans/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=poison.venom.gule-gulzar.com,resources=clusterscans/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

func (r *ClusterScanReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	logger := log.FromContext(ctx)

	currentTime := time.Now()
	logger.Info("[running reconciler]", "time", currentTime.String())

	clusterscan := poisonv1.ClusterScan{}
	err := r.Get(ctx, req.NamespacedName, &clusterscan)
	if err != nil {
		logger.Error(err, "could not find clusterscan resource")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	//*********************** check if job already scheduled
	if clusterscan.Status.LastScheduledTime != nil {

		//*********************** check if it is a one off or recurring clusterscan resource

		if clusterscan.Spec.Schedule != nil {

			//*********************** get the list of all jobs run by our clusterscan resource

			var allJobs kbatch.JobList
			if err := r.List(ctx, &allJobs, client.InNamespace(req.Namespace)); err != nil {
				logger.Error(err, "unable to list all Jobs")
				return ctrl.Result{}, err
			}

			// var jobsToDelete []kbatch.Job

			for _, job := range allJobs.Items {
				// if job has our label
				if label, ok := job.Labels[customJobLabel]; ok {
					// if the label is for the current clusterscan resource
					if label == fmt.Sprintf("%s-%s", clusterscan.Namespace, clusterscan.Name) {

						toDelete, err := needsDeletion(&job, currentTime, clusterscan.Spec.JobRetentionTime)
						if err != nil {
							logger.Error(err, "unable to determine job's expiry")
							return ctrl.Result{}, err
						}

						if *toDelete {
							logger.Error(err, "deleting old job")
							if err := r.Delete(ctx, &job, client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
								logger.Error(err, "unable to delete expired job", "job", job)
							}
						}

					}
				}
			}

			//*********************** calculate the nexttime from the last scheduled time using helper function

			nextExecutionTime, err := getNextExecutionTime(logger, clusterscan.Status.LastScheduledTime.Time, *clusterscan.Spec.Schedule)
			if err != nil {
				logger.Error(err, "cannot calculate next execution time")

				return ctrl.Result{
					Requeue: true,
				}, nil
			}

			// current time is before next execution time
			if currentTime.Before(*nextExecutionTime) {
				logger.Info("")
				return ctrl.Result{
					RequeueAfter: nextExecutionTime.Sub(currentTime),
				}, nil
			}

		} else {

			logger.Info("job already created")
			return ctrl.Result{}, nil

		}
	}

	//*********************** create the job

	// initialize job using helper function
	job, err := createJob(&clusterscan, currentTime)
	if err != nil {
		logger.Error(err, "unable to construct job from template")

		return ctrl.Result{
			Requeue: true,
		}, nil
	}

	// set current clusterscan as owner of the above initialized job
	if err := ctrl.SetControllerReference(&clusterscan, job, r.Scheme); err != nil {
		logger.Error(err, "cannot set reference of clusterscan")
		return ctrl.Result{
			Requeue: true,
		}, nil
	}

	// finally, create the job
	if err := r.Create(ctx, job); err != nil {
		logger.Error(err, "unable to create Job for CronJob", "job", job)
		return ctrl.Result{}, err
	}

	// update last scheduled time for the clusterscan
	clusterscan.Status.LastScheduledTime = &metav1.Time{Time: currentTime}
	if err := r.Status().Update(ctx, &clusterscan); err != nil {
		logger.Error(err, "unable to update CronJob status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func getNextExecutionTime(logger logr.Logger, calculateFromTime time.Time, cronExpression string) (*time.Time, error) {

	logger.Info("[get Next Execution Time]", "current time", calculateFromTime)
	cronParser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := cronParser.Parse(cronExpression)
	if err != nil {
		fmt.Println("Error parsing cron expression:", err)
		return nil, err
	}

	nextTime := schedule.Next(calculateFromTime)
	return &nextTime, nil

}

func createJob(clusterscan *poisonv1.ClusterScan, currentTime time.Time) (*kbatch.Job, error) {

	name := fmt.Sprintf("%s-%d", clusterscan.Name, currentTime.Unix())
	job := &kbatch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
			Name:        name,
			Namespace:   clusterscan.Namespace,
		},
		Spec: *clusterscan.Spec.JobTemplate.Spec.DeepCopy(),
	}
	for k, v := range clusterscan.Spec.JobTemplate.Annotations {
		job.Annotations[k] = v
	}
	job.Annotations[scheduledAtAnnotation] = currentTime.Format(time.RFC3339)
	for k, v := range clusterscan.Spec.JobTemplate.Labels {
		job.Labels[k] = v
	}
	job.Labels[customJobLabel] = fmt.Sprintf("%s-%s", clusterscan.Namespace, clusterscan.Name)

	return job, nil

}

func needsDeletion(job *kbatch.Job, currentTime time.Time, retentionTime int32) (*bool, error) {
	timeRaw := job.Annotations[scheduledAtAnnotation]
	if len(timeRaw) == 0 {
		return nil, nil
	}

	timeParsed, err := time.Parse(time.RFC3339, timeRaw)
	if err != nil {
		return nil, err
	}

	addedTime := timeParsed.Add(time.Minute * time.Duration(retentionTime))

	returnValue := addedTime.Before(currentTime)

	return &returnValue, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterScanReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&poisonv1.ClusterScan{}).
		Owns(&kbatch.Job{}).
		Complete(r)
}
