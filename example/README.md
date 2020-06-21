# Example

Create a Deployment that create 3 Pods of nginx.

```shell
kubectl apply -f https://raw.githubusercontent.com/jlandowner/pod-disposal-operator/master/example/deployment.yaml
```

Then, create a PodDisposalSchedule in the same namespace with the Deployment.

```shell
kubectl apply -f https://raw.githubusercontent.com/jlandowner/pod-disposal-operator/master/example/pod-disposal-schedule.yaml
```

1 minute later, you can see a Pod is disposed.

`DisposalConcurrerny` is 2 in the configuration file but even a single Pod is deleted.
It is because Pod replicas are 3 and `MinAvailavle` is 2.

See the status and the last disposal result.

```shell
kubectl get pds sample-nginx-pds -n default
```

See the detail. You can see the detail of the last disposal as Events.

```shell
kubectl describe pds sample-nginx-pds -n default
```

