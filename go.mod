module github.com/digitalocean/digitalocean-cloud-controller-manager

go 1.15

require (
	github.com/davecgh/go-spew v1.1.1
	github.com/digitalocean/godo v1.67.0
	github.com/fsnotify/fsnotify v1.4.9 // indirect
	github.com/go-ini/ini v1.39.0 // indirect
	github.com/google/go-cmp v0.5.4
	github.com/minio/minio-go v6.0.10+incompatible
	github.com/mitchellh/copystructure v1.0.0
	github.com/prometheus/client_golang v1.7.1
	golang.org/x/lint v0.0.0-20200302205851-738671d3881b
	golang.org/x/mod v0.3.1-0.20200828183125-ce943fd02449 // indirect
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	golang.org/x/sync v0.0.0-20201020160332-67f06af15bc9
	golang.org/x/tools v0.1.0 // indirect
	k8s.io/api v0.21.3
	k8s.io/apimachinery v0.21.3
	k8s.io/client-go v0.21.3
	k8s.io/cloud-provider v0.21.3
	k8s.io/component-base v0.21.3
	k8s.io/klog/v2 v2.8.0
	k8s.io/utils v0.0.0-20201110183641-67b214c5f920
)
