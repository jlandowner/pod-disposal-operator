# Example

Create a Deployment that create 3 Pods of nginx.

```shell
kubectl apply -f https://raw.githubusercontent.com/jlandowner/pod-disposal-operator/master/samples/deployment.yaml
```

Then, create a PodDisposalSchedule in the same namespace with the Deployment.

```shell
kubectl apply -f https://raw.githubusercontent.com/jlandowner/pod-disposal-operator/master/samples/pod-disposal-schedule.yaml
```

1 minute later, you can see a pod is disposed.

See the status.

```shell
kubectl get pds sample-nginx-pds -n default
```

See the detail.

```shell
kubectl describe pds sample-nginx-pds -n default
```

