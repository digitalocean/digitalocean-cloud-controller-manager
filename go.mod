module github.com/digitalocean/digitalocean-cloud-controller-manager

go 1.15

require (
	github.com/davecgh/go-spew v1.1.1
	github.com/digitalocean/godo v1.69.0
	github.com/go-ini/ini v1.39.0 // indirect
	github.com/google/go-cmp v0.5.7
	github.com/minio/minio-go v6.0.10+incompatible
	github.com/mitchellh/copystructure v1.0.0
	github.com/prometheus/client_golang v1.11.0
	golang.org/x/lint v0.0.0-20210508222113-6edffad5e616
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	k8s.io/api v0.22.5
	k8s.io/apimachinery v0.22.5
	k8s.io/client-go v0.22.5
	k8s.io/cloud-provider v0.22.5
	k8s.io/component-base v0.22.5
	k8s.io/klog/v2 v2.9.0
	k8s.io/utils v0.0.0-20210819203725-bdf08cb9a70a
)
