# Example Manifests

The following directory provides examples of static pod, daemonset and systemd manifests
that were used in a working Kubernetes cluster on DigitalOcean. Use the following examples as a guide
to setup your cluster correctly using the DigitalOcean Cloud Controller Manager.
Note that the examples are not meant to be copy and pasted as it contains configuration
for a specific cluster.

* [kubelet.service on master](master-kubelet.service) - systemd manifest for running kubelet on master nodes
* [kubelet.service](kubelet.service) - systemd manifest for running kubelet on Kubernetes nodes
* [kube-apiserver.yml](kube-apiserver.yml) - static pod for kube-apiserver
* [kube-controller-manager.yml](kube-controller-manager.yml) - static pod for kube-controller-manager
* [kube-scheduler.yml](kube-scheduler.yml) - static pod for kube-scheduler
* [cloud-controller-manager.yml](cloud-controller-manager.yml) - DaemonSet for DigitalOcean cloud-controller-manager

