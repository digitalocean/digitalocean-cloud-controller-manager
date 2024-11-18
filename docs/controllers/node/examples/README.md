# Node Features with Digitalocean Cloud Controller Manager

## Labels and Addresses

DigitalOcean cloud controller manager, applies DigitalOcean specific annotations and node labels for your nodes. It also provides nodes with the correct address types.

Here's a node before digitalocean cloud controller manager was running in a cluster:

```bash
$ kubectl get no k8s-worker-01 -o yaml
apiVersion: v1
kind: Node
metadata:
  annotations:
    node.alpha.kubernetes.io/ttl: "0"
  creationTimestamp: 2017-08-04T21:37:10Z
  labels:
    beta.kubernetes.io/arch: amd64
    beta.kubernetes.io/os: linux
    kubernetes.io/hostname: k8s-worker-01
  name: k8s-worker-01
spec:
  externalID: k8s-worker-01
status:
  addresses:
  - address: 138.197.174.87
    type: InternalIP
  - address: k8s-worker-01
    type: Hostname
  allocatable:
    cpu: "4"
    memory: 6012708Ki
    pods: "110"
  capacity:
    cpu: "4"
    memory: 6115108Ki
    pods: "110"
    ...
```

This node is treated as if it was "bare metal", only OS level details of the node are known such as its architecture and its operating system.
It's also reporting the nodes public IP as the `InternalIP` which should not be the case.

Now here's a node in a cluster where digitalocean cloud controller manager is running:

```bash
$ kubectl get no k8s-worker-02 -o yaml
apiVersion: v1
kind: Node
metadata:
  annotations:
    node.alpha.kubernetes.io/ttl: "0"
  creationTimestamp: 2017-07-19T18:51:51Z
  labels:
    beta.kubernetes.io/arch: amd64
    beta.kubernetes.io/instance-type: c-4
    beta.kubernetes.io/os: linux
    failure-domain.beta.kubernetes.io/region: nyc1
    kubernetes.io/hostname: k8s-worker-02
  name: k8s-worker-02
spec:
  externalID: k8s-worker-02
status:
  addresses:
  - address: k8s-worker-02
    type: Hostname
  - address: 10.137.112.202
    type: InternalIP
  - address: 138.197.174.81
    type: ExternalIP
  - address: 2a03:b0c0:3:d0::e68:a001
    type: ExternalIP
  allocatable:
    cpu: "4"
    memory: 6012700Ki
    pods: "110"
  capacity:
    cpu: "4"
    memory: 6115100Ki
    pods: "110"
```

DigitalOcean cloud controller manager has made the cluster aware of the size of the node, in this case c-4 (4 core high CPU droplet). It has also assigned the node
a failure domain which the scheduler can use for region failovers. Note also that the correct addresses were assigned to the node. The `InternalIP` now represents
the private IP of the droplet, and the `ExternalIP` is it's public IP. The order and IP families depends on the env variable `DO_IP_ADDR_FAMILIES`.

## Node clean up

When deleting a node in a Kubernetes cluster, deleting droplets would leave the corresponding Kubernetes node in a `NotReady` state. It was the responsibility
of the cluster admin to delete the node from Kubernetes afterwards. DigitalOcean cloud controller manager will automatically delete nodes in a Kubernetes cluster
when the associated droplet was also deleted.
