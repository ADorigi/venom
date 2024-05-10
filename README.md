# Venom
This repository will house the components for Venom, a basic Kubernetes operator.

## Why controllers?

In kubernetes, controllers are reconciliation loops which keep track of the state of your cluster, and modify it when required. Each controller tries to move the current state of your cluster closer to the desired state.  
Controllers are basically trackers of a resource type. These types have a spec field which define their desired state. 

## Why Operator?

A kuberentes operator is a pattern which consists of a kubernetes controller, and some custom resource definitions. It basically is a controller with some extra domain specific knowledge. 

## Why Venom?

The Venom operator defines a resource called ClusterScan, which, as you might have guessed, scans the cluster. But it also exposes a rest api, which lets you get information about the native kubernetes resources inside the cluster. In other words, it can losely be considered a wrapper to `kubectl get` for the current scope of the project. 

## How do you define this custom resource which Venom monitors?




## Steps I followed

1. Created a clean kubernetes cluster with `minikube start`
2. Downloaded kubebuilder using 


```
curl -L -o kubebuilder "https://go.kubebuilder.io/dl/latest/$(go env GOOS)/$(go env GOARCH)"
```


3. Change permission to allow execution with `chmod +x kubebuilder`
4. Move it to `/usr/local/bin/` folder with `mv kubebuilder /usr/local/bin/`. I had to use `sudo`.


