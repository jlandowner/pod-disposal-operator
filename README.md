# Pod Disposal Operator

Pod Disposal Operator performs a simple operation that dispose some pods in Deployment regularly specified by a cron expression.

## Target Usecase

- Relocating running Pods periodically according to the behaviors/characteristics of each application.
- Performing "light" chaos engineering while managing deletion targets and timing to execute.
- Rotating Pods. For example, aiming to release Pod's output files, memories, processes and so on.

# Background

## Descheduler
[Descheduler](https://github.com/kubernetes-sigs/descheduler) is generally used to relocate running Pods.
Descheduler has an advantage that you can configure a variety of policies, strategies and options.
However, it has another aspect that is therefore not easy to configure sometimes.

Also it is common to run Descenduler by cronjob in the cluster. But in a each Job execution, it will target to all Pods in the cluster excluding some Pods.
Therefore, in the multi-tenant cluster for example, cluter administrators like platform teams sometimes have to run it understanding all of the behaviors/characteristics of applications of the service teams in the cluster.

## Chaos-Engineering
Kubernetes users are probably interested in chaos engineering more or less, but many of them might be not sure how to begin it.

For example as application containers, the levels you would like to try Chaos-Engineering may be depending on the characteristics and maturity of the application, even if it's a fault-tolerant application (e.g., you want to do it at a time of day when there are few accesses).

You would like to start with a small start from the point where problems does not effect for the entire service, even if some problems occur.

## Pod Rotation
Many applications running for long periods of time sometimes experience problems.
Legacy applications might have a maintenance job to release memories and processes by restart periodically.

Also, if your applications are outputting files per pods to NFS or like that, you might still need rotations the files or other mechanisms.
When creating files per pods, its name might be with a unique Pod name or other key.
In that case, when you delete the pods, ReplicaSet recreate new pods and the files are naturally rotated as well.
It is one of the easiest way to maintain them.

# Features
- You can configure disposal timing for each Deployment.
Pod Disposal Operator watchs custom resources called the `PodDisposalSchedule` that you can create for each Deployment.
Depending on the behavior/charactoristic of your application, you can rotate a large number of pods at once during small traffic-coming periods, or remove a small number of pods more frequently.

- Pod Disposal Operator perform just dispose old Pods by default. Recreation of Pods are dependent on K8s auto healing mechanisms. (ReplicaSet will detect that the number of Pods are not enough.)
You can check the cluster's and your application's fault-torrelant behavior is working enough.

- Based on the idea that "Pods running for a longer period of time causes the unbalanced Pod allocation", the operator removes Pods in old order by default.
This has also the aspect of a Pod rotation.

- Combined with Kubernetes wide-spreading scheduling mechanisms such as [Inter-Pod Anti-Affinity](https://kubernetes.io/ja/docs/concepts/configuration/assign-pod-node/#inter-pod-affinity-anti-affinity) and [Pod Topology Spread Constraints](https://kubernetes.io/docs/concepts/workloads/pods/pod-topology-spread-constraints/), the possibility that pods will be properly relocated in recreation after disposal may increase.

# QuickStart
```shell
kubectl apply -f https://raw.githubusercontent.com/jlandowner/pod-disposal-operator/master/deploy/quickstart.yaml
```

# Custom Resource Definition

You can cofigure a disposal schedule and strategy by a single Custom Resource Difinition `PodDisposalSchedule`.
Sample configuration is as following.

```yaml
apiVersion: operator.k8s.jlandowner.com/v1
kind: PodDisposalSchedule
metadata:
  name: poddisposalschedule-sample
  namespace: default
spec:
  # Target Pods selector
  selector:
    # Only Deployment is available
    type: Deployment
    # Deployment name
    name: sample-nginx
  # Cron format's disposal schedule
  schedule: "* */3 * * *"
  strategy:
    # Max number of pods to be deleted at the same time
    disposalConcurrency: 2
    # Pods that are living over lifespan will be deleted only
    lifespan: 12h
    # Number of pods to be kept
    minAvailable: 2
```

## CRD Details

- `selector`
Specify the target Pods to be deleted.
  - `type`:
  Sorry, only `Deployment` is allowed now.
  - `name`:
  Name of the Deployment.
- `schedule`
Timing to disposal by cron expression.
- `strategy`
Simple strategy about Disposal.
  - `order`:
  Pod's order to be deleted. Sorry but only `Old`(deleting by old order) is allowed now.
  - `disposalConcurrency`:
  Max number of Pod disposal at the same time. Basically the number of Pods are deleted, but it can be changed according to `minAvailable` value.
  - `lifespan`:
  Lifespan of Pod. The only Pods living over their lifespan will be deleted.
  - `minAvailable`:
  Minumum number of Pods to keep. The number of Pods running in the cluster will never be smaller than the value you specify.

`PodDisposalSchedule` is a Namespaced Custom Resource, so you should apply it in the same Namespace with the target Pods.

# Example
See the example.
https://github.com/jlandowner/pod-disposal-operator/tree/master/example

# License
Apache License Version 2.0