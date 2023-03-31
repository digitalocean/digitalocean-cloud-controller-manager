module github.com/digitalocean/digitalocean-cloud-controller-manager

go 1.19

require (
	github.com/davecgh/go-spew v1.1.1
	github.com/digitalocean/godo v1.97.1-0.20230308191258-ca2138d84446
	github.com/go-logr/logr v1.2.3
	github.com/google/go-cmp v0.5.9
	github.com/minio/minio-go v6.0.14+incompatible
	github.com/mitchellh/copystructure v1.2.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.14.0
	golang.org/x/lint v0.0.0-20210508222113-6edffad5e616
	golang.org/x/oauth2 v0.0.0-20220411215720-9780585627b5
	golang.org/x/sync v0.1.0
	k8s.io/api v0.26.2
	k8s.io/apimachinery v0.26.2
	k8s.io/client-go v0.26.2
	k8s.io/cloud-provider v0.26.2
	k8s.io/component-base v0.26.2
	k8s.io/klog/v2 v2.90.1
	k8s.io/utils v0.0.0-20221128185143-99ec85e7a448
	sigs.k8s.io/controller-runtime v0.14.5
)

require (
	github.com/go-ini/ini v1.39.0 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/smartystreets/goconvey v1.7.2 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	k8s.io/apiextensions-apiserver v0.26.2 // indirect
)
