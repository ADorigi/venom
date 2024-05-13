# Venom
This repository will house the components for Venom, a basic Kubernetes operator.

## Why controllers?

In kubernetes, controllers are reconciliation loops which keep track of the state of your cluster, and modify it when required. Each controller tries to move the current state of your cluster closer to the desired state.  
Controllers are basically trackers of a resource type. These types have a spec field which define their desired state. 

## Why Operator?

A kuberentes operator is a pattern which consists of a kubernetes controller, and some custom resource definitions. It basically is a controller with some extra domain specific knowledge. 

## Why Venom?

The Venom operator defines a resource called ClusterScan. For the time being, it is designed to run a custom kuberntes job with the specified spec, but it can be designed further. 

## How do you define this custom resource which Venom monitors?


A one-off ClusterScan reesource can be created using a similar manifest as given below:

```
apiVersion: poison.venom.gule-gulzar.com/v1
kind: ClusterScan
metadata:
  name: clusterscan-one-off
spec:
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - image: busybox
            name: testjob
          restartPolicy: Never

```

A recurring ClusterScan reesource can be created using a similar manifest as given below:

```

apiVersion: poison.venom.gule-gulzar.com/v1
kind: ClusterScan
metadata:
  name: clusterscan-cron
spec:
  schedule: "*/1 * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - image: busybox
            name: testjob
          restartPolicy: Never
  jobRetentionTime: 5

```


## Using Venom:

Since Venom is still under development, it can be deployed from source using the following steps:

1. You need access to a kubernetes cluster. The following commands automatically use the current context in your `kubeconfig` file.
2. Run `make install` command to install the CRDs into the cluster.
3. Run `make run` command to start the controller locally.
4. Use kustomize to deploy sample resources for ClusterScan resource, defined in config/samples/ of this repository. 
```
kubectl apply -k config/samples
```

### Using operator image from docker hub

Instead of `make run`, the deployed image for the operator from docker hub can also be used


<!-- 
## Steps I followed

1. Created a clean kubernetes cluster with `minikube start`
2. Downloaded kubebuilder using 


```
curl -L -o kubebuilder "https://go.kubebuilder.io/dl/latest/$(go env GOOS)/$(go env GOARCH)"
```


3. Change permission to allow execution with `chmod +x kubebuilder`
4. Move it to `/usr/local/bin/` folder with `mv kubebuilder /usr/local/bin/`. I had to use `sudo`.
5. Initilize go mod with `go mod init github.com/adorigi/venom`
6. Scaffold the operator using the following command

```
kubebuilder init --domain venom.gule-gulzar.com --repo github.com/adorigi/venom
```

7. Initilize the new api as follows:

```
kubebuilder create api --group poison --version v1 --kind ClusterScan
```

## useful coommnads

- kubectl proxy --port=8080   -> curl localhost:8080/apis
- kubebuilder create api --group ___ --version v1 --kind ClusterScan

 -->

