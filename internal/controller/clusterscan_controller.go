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

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ClusterScan object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *ClusterScanReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	logger := log.FromContext(ctx)

	currentTime := time.Now()
	logger.Info("reconcile running at", "time", currentTime.String())

	//                    logger.Info("reconcile running at", currentTime.Unix())

	// TODO(user): your logic here
	clusterscan := poisonv1.ClusterScan{}
	err := r.Get(ctx, req.NamespacedName, &clusterscan)
	if err != nil {
		logger.Error(err, "could not find clusterscan resource")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// logger.Info("running reconcile for", "clusterscan", clusterscan)

	logger.Info("running reconciliation loop")

	//*********************** get the list of all jobs run by our clusterscan resource

	var ourJobs kbatch.JobList
	if err := r.List(ctx, &ourJobs, client.InNamespace(req.Namespace)); err != nil {
		logger.Error(err, "unable to list child Jobs")
		return ctrl.Result{}, err
	}

	//*********************** get the time at which the last one was run

	if clusterscan.Status.LastScheduledTime != nil {

		//*********************** calculate the nexttime from the last scheduled time using helper function

		nextExecutionTime, err := getNextExecutionTime(logger, clusterscan.Status.LastScheduledTime.Time, clusterscan.Spec.Schedule)
		if err != nil {
			logger.Error(err, "cannot calculate next execution time")

			return ctrl.Result{
				Requeue: true,
			}, nil
		}

		// current time is before next execution time
		if time.Now().Before(*nextExecutionTime) {
			logger.Info("")
			return ctrl.Result{
				Requeue: true,
			}, nil
		}

	}

	//*********************** create the job
	job, err := createJob(&clusterscan, currentTime)
	if err != nil {
		logger.Error(err, "unable to construct job from template")

		return ctrl.Result{
			Requeue: true,
		}, nil
	}
	if err := ctrl.SetControllerReference(&clusterscan, job, r.Scheme); err != nil {
		logger.Error(err, "cannot set reference of clusterscan")
		return ctrl.Result{
			Requeue: true,
		}, nil
	}

	if err := r.Create(ctx, job); err != nil {
		logger.Error(err, "unable to create Job for CronJob", "job", job)
		return ctrl.Result{}, err
	}

	clusterscan.Status.LastScheduledTime = &metav1.Time{Time: currentTime}
	if err := r.Status().Update(ctx, &clusterscan); err != nil {
		logger.Error(err, "unable to update CronJob status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// func getScheduledTimeForJobHelper(job *kbatch.Job) (*time.Time, error) {
// 	timeRaw := job.Annotations[scheduledAtAnnotation]
// 	if len(timeRaw) == 0 {
// 		return nil, nil
// 	}

// 	timeParsed, err := time.Parse(time.RFC3339, timeRaw)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return &timeParsed, nil
// }

func getNextExecutionTime(logger logr.Logger, calculateFromTime time.Time, cronExpression string) (*time.Time, error) {

	//                   logger.Info("calculating next execution time")
	cronParser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := cronParser.Parse(cronExpression)
	if err != nil {
		fmt.Println("Error parsing cron expression:", err)
		return nil, err
	}

	nextTime := schedule.Next(calculateFromTime)
	return &nextTime, nil

}

func getScheduledResult(currentTime time.Time) ctrl.Result {

	return ctrl.Result{
		Requeue: true,
	}

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

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterScanReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&poisonv1.ClusterScan{}).
		Owns(&kbatch.Job{}).
		Complete(r)
}
