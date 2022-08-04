module e2e

go 1.18

require (
	github.com/bradfitz/gomemcache v0.0.0-20220106215444-fb4bf637b56d
	github.com/castai/k8s-memcache-selector v0.0.0
)

require (
	github.com/castai/k8s-client-go v0.3.0 // indirect
	github.com/fsnotify/fsnotify v1.5.4 // indirect
	golang.org/x/sys v0.0.0-20220412211240-33da011f77ad // indirect
)

replace github.com/castai/k8s-memcache-selector v0.0.0 => ../
