module github.com/digitalocean/digitalocean-cloud-controller-manager

go 1.14

require (
	github.com/NYTimes/gziphandler v1.0.1 // indirect
	github.com/coreos/pkg v0.0.0-20180928190104-399ea9e2e55f // indirect
	github.com/davecgh/go-spew v1.1.1
	github.com/digitalocean/godo v1.35.1
	github.com/go-ini/ini v1.39.0 // indirect
	github.com/golang/groupcache v0.0.0-20180513044358-24b0969c4cb7 // indirect
	github.com/google/go-cmp v0.5.0
	github.com/googleapis/gnostic v0.2.0 // indirect
	github.com/minio/minio-go v6.0.10+incompatible
	github.com/spf13/pflag v1.0.5
	github.com/tmc/grpc-websocket-proxy v0.0.0-20190109142713-0ad062ec5ee5 // indirect
	golang.org/x/lint v0.0.0-20190409202823-959b441ac422
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	gopkg.in/ini.v1 v1.42.0 // indirect
	k8s.io/api v0.18.3
	k8s.io/apimachinery v0.18.3
	k8s.io/apiserver v0.18.3
	k8s.io/client-go v0.18.3
	k8s.io/cloud-provider v0.18.3
	k8s.io/component-base v0.18.3
	k8s.io/klog v1.0.0
	k8s.io/kubernetes v1.17.5
)

replace k8s.io/api => k8s.io/api v0.18.3

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.18.3

replace k8s.io/apimachinery => k8s.io/apimachinery v0.18.4-rc.0

replace k8s.io/apiserver => k8s.io/apiserver v0.18.3

replace k8s.io/cli-runtime => k8s.io/cli-runtime v0.18.3

replace k8s.io/client-go => k8s.io/client-go v0.18.3

replace k8s.io/cloud-provider => k8s.io/cloud-provider v0.18.3

replace k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.18.3

replace k8s.io/code-generator => k8s.io/code-generator v0.18.4-rc.0

replace k8s.io/component-base => k8s.io/component-base v0.18.3

replace k8s.io/cri-api => k8s.io/cri-api v0.18.4-rc.0

replace k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.18.3

replace k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.18.3

replace k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.18.3

replace k8s.io/kube-proxy => k8s.io/kube-proxy v0.18.3

replace k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.18.3

replace k8s.io/kubelet => k8s.io/kubelet v0.18.3

replace k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.18.3

replace k8s.io/metrics => k8s.io/metrics v0.18.3

replace k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.18.3

replace k8s.io/kubectl => k8s.io/kubectl v0.18.3

replace k8s.io/klog => k8s.io/klog v0.4.0

replace k8s.io/kubernetes => k8s.io/kubernetes v1.18.3
