# Example

Create a Deployment that create 3 Pods of nginx.

```shell
kubectl apply -f https://raw.githubusercontent.com/jlandowner/pod-disposal-operator/master/example/deployment.yaml
```

Then, create a PodDisposalSchedule in the same namespace with the Deployment.

```shell
kubectl apply -f https://raw.githubusercontent.com/jlandowner/pod-disposal-operator/master/example/pod-disposal-schedule.yaml
```

```shell
while true; do kubectl get po -n default -l deploy=sample-nginx; sleep 10; done
```

1 minute later, you can see a Pod is disposed.

`DisposalConcurrerny` is 2 in the configuration file but even a single Pod is deleted.
It is because Pod replicas are 3 and `MinAvailavle` is 2.

See the status and the last disposal result.

```shell
kubectl get pds sample-nginx-pds -n default
```

```
NAME               TARGETTYPE   TARGETNAME     LASTDISPOSALCOUNTS   LASTDISPOSALTIME       NEXTDISPOSALTIME
sample-nginx-pds   Deployment   sample-nginx   1                    2020-06-22T09:56:00Z   2020-06-22T09:57:00Z
```

See the detail. You can see the detail of the last disposal as Events.

```shell
kubectl describe pds sample-nginx-pds -n default
```

```
...

Spec:
  Schedule:  */1 * * * *
  Selector:
    Name:       sample-nginx
    Namespase:  default
    Type:       Deployment
  Strategy:
    Disposal Concurrency:  2
    Lifespan:              1m
    Min Available:         2
    Order:                 Old
Status:
  Last Disposal Counts:  1
  Last Disposal Time:    2020-06-22T09:56:00Z
  Next Disposal Time:    2020-06-22T09:57:00Z
Events:
  Type    Reason    Age   From                   Message
  ----    ------    ----  ----                   -------
  Normal  Init      92s   pod-disposal-operator  Successfully initalized status
  Normal  Starting  40s   pod-disposal-operator  Pod disposal is starting
  Normal  Disposed  40s   pod-disposal-operator  Successfully delete pod sample-nginx-cf5b9fd8c-g589t created at 2020-06-22 09:54:22 +0000 UTC
  Normal  Finished  40s   pod-disposal-operator  Pod disposal is successfully finished

```