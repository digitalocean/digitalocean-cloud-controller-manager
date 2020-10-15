module github.com/digitalocean/digitalocean-cloud-controller-manager

go 1.15

require (
	github.com/NYTimes/gziphandler v1.0.1 // indirect
	github.com/davecgh/go-spew v1.1.1
	github.com/digitalocean/godo v1.35.1
	github.com/go-ini/ini v1.39.0 // indirect
	github.com/google/go-cmp v0.5.0
	github.com/minio/minio-go v6.0.10+incompatible
	github.com/mitchellh/copystructure v1.0.0
	github.com/prometheus/client_golang v1.7.1
	github.com/spf13/pflag v1.0.5
	golang.org/x/lint v0.0.0-20191125180803-fdd1cda4f05f
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	golang.org/x/sync v0.0.0-20201008141435-b3e1573b7520
	gopkg.in/ini.v1 v1.42.0 // indirect
	k8s.io/api v0.19.2
	k8s.io/apimachinery v0.19.2
	k8s.io/client-go v0.19.2
	k8s.io/cloud-provider v0.19.2
	k8s.io/component-base v0.19.2
	k8s.io/klog/v2 v2.2.0
	k8s.io/kubernetes v1.17.5
	k8s.io/utils v0.0.0-20200729134348-d5654de09c73
)

replace k8s.io/api => k8s.io/api v0.19.2

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.19.2

replace k8s.io/apimachinery => k8s.io/apimachinery v0.19.3-rc.0

replace k8s.io/apiserver => k8s.io/apiserver v0.19.2

replace k8s.io/cli-runtime => k8s.io/cli-runtime v0.19.2

replace k8s.io/client-go => k8s.io/client-go v0.19.2

replace k8s.io/cloud-provider => k8s.io/cloud-provider v0.19.2

replace k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.19.2

replace k8s.io/code-generator => k8s.io/code-generator v0.19.3-rc.0

replace k8s.io/component-base => k8s.io/component-base v0.19.2

replace k8s.io/cri-api => k8s.io/cri-api v0.19.3-rc.0

replace k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.19.2

replace k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.19.2

replace k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.19.2

replace k8s.io/kube-proxy => k8s.io/kube-proxy v0.19.2

replace k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.19.2

replace k8s.io/kubelet => k8s.io/kubelet v0.19.2

replace k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.19.2

replace k8s.io/metrics => k8s.io/metrics v0.19.2

replace k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.19.2

replace k8s.io/kubectl => k8s.io/kubectl v0.19.2

replace k8s.io/kubernetes => k8s.io/kubernetes v1.19.2

replace k8s.io/gengo => k8s.io/gengo v0.0.0-20200428234225-8167cfdcfc14

replace k8s.io/heapster => k8s.io/heapster v1.2.0-beta.1

replace k8s.io/klog/v2 => k8s.io/klog/v2 v2.2.0

replace k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20200805222855-6aeccd4b50c6

replace k8s.io/system-validators => k8s.io/system-validators v1.1.2

replace k8s.io/utils => k8s.io/utils v0.0.0-20200729134348-d5654de09c73
