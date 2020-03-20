module github.com/digitalocean/digitalocean-cloud-controller-manager

go 1.12

require (
	github.com/NYTimes/gziphandler v1.0.1 // indirect
	github.com/coreos/bbolt v1.3.2 // indirect
	github.com/coreos/pkg v0.0.0-20180928190104-399ea9e2e55f // indirect
	github.com/davecgh/go-spew v1.1.1
	github.com/digitalocean/godo v1.35.1
	github.com/go-ini/ini v1.39.0 // indirect
	github.com/golang/groupcache v0.0.0-20180513044358-24b0969c4cb7 // indirect
	github.com/googleapis/gnostic v0.2.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.8.5 // indirect
	github.com/minio/minio-go v6.0.10+incompatible
	github.com/prometheus/client_golang v0.9.3-0.20190127221311-3c4408c8b829 // indirect
	github.com/soheilhy/cmux v0.1.4 // indirect
	github.com/spf13/pflag v1.0.3
	github.com/tmc/grpc-websocket-proxy v0.0.0-20190109142713-0ad062ec5ee5 // indirect
	github.com/xiang90/probing v0.0.0-20190116061207-43a291ad63a2 // indirect
	go.etcd.io/bbolt v1.3.2 // indirect
	golang.org/x/lint v0.0.0-20190409202823-959b441ac422
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/ini.v1 v1.42.0 // indirect
	k8s.io/api v0.0.0
	k8s.io/apimachinery v0.0.0
	k8s.io/apiserver v0.0.0
	k8s.io/client-go v0.0.0
	k8s.io/cloud-provider v0.0.0
	k8s.io/component-base v0.0.0
	k8s.io/klog v0.4.0
	k8s.io/kubernetes v1.16.2
)

// replace k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190404173353-6a84e37a896d
replace k8s.io/api => k8s.io/api v0.0.0-20191016110408-35e52d86657a

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20191016113550-5357c4baaf65

replace k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20191004115801-a2eda9f80ab8

replace k8s.io/apiserver => k8s.io/apiserver v0.0.0-20191016112112-5190913f932d

replace k8s.io/cli-runtime => k8s.io/cli-runtime v0.0.0-20191016114015-74ad18325ed5

replace k8s.io/client-go => k8s.io/client-go v0.0.0-20191016111102-bec269661e48

replace k8s.io/cloud-provider => k8s.io/cloud-provider v0.0.0-20191016115326-20453efc2458

replace k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.0.0-20191016115129-c07a134afb42

replace k8s.io/code-generator => k8s.io/code-generator v0.0.0-20191004115455-8e001e5d1894

replace k8s.io/component-base => k8s.io/component-base v0.0.0-20191016111319-039242c015a9

replace k8s.io/cri-api => k8s.io/cri-api v0.0.0-20190828162817-608eb1dad4ac

replace k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.0.0-20191016115521-756ffa5af0bd

replace k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.0.0-20191016112429-9587704a8ad4

replace k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.0.0-20191016114939-2b2b218dc1df

replace k8s.io/kube-proxy => k8s.io/kube-proxy v0.0.0-20191016114407-2e83b6f20229

replace k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.0.0-20191016114748-65049c67a58b

replace k8s.io/kubelet => k8s.io/kubelet v0.0.0-20191016114556-7841ed97f1b2

replace k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.0.0-20191016115753-cf0698c3a16b

replace k8s.io/metrics => k8s.io/metrics v0.0.0-20191016113814-3b1a734dba6e

replace k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.0.0-20191016112829-06bb3c9d77c9

replace k8s.io/kubectl => k8s.io/kubectl v0.0.0-20191016120415-2ed914427d51
