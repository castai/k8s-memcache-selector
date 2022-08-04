## About

Memcache server discovery based on Kubernetes endpoints for [Go memcache client](https://github.com/bradfitz/gomemcache)

How it works:
* Watches endpoints containing memcache pod ips using [tiny k8s client](https://github.com/castai/k8s-client-go)
* Implements ServerPicker interface to dynamically update memcache ip addresses.

## Use cases

In dynamic environments like Kubernetes both clients (pods calling memcache) and memcache pods could change.
There are few issues:
1. Calling memcache using clusterIP service is quite useless (unless you have one replica) as kernel will perform round-robin across service endpoints.
2. You can enable sessionAffinity. This will ensure that client pod always goes to the same memcache instance. But what happens if pods are redeployed?
3. Using StatefulSet with headless services and passing each endpoint is kind of static too.

## Installing

```
go get github.com/castai/k8s-memcache-selector
```

## Usage

```go
package main

import (
	"context"
	"log"

	"github.com/bradfitz/gomemcache/memcache"

	selector "github.com/castai/k8s-memcache-selector"
)

func main() {
	ss, err := selector.NewServerList(context.Background(), "memcache-headless:11211")
	if err != nil {
		log.Fatalf("creating server selector: %v", err)
	}
	cache := memcache.NewFromSelector(ss)

	// Use cache..
}
```

## Permissions

Your application needs permission to get and watch endpoints. Example manifest:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: endpoints-watcher
  namespace: e2e
---
kind: Role
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: endpoints-watcher
rules:
  - apiGroups: [ "" ]
    resources: [ "endpoints" ]
    verbs: [ "get", "watch" ]
---
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: endpoints-watcher
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: endpoints-watcher
subjects:
  - kind: ServiceAccount
    name: endpoints-watcher
    namespace: e2e
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-service
  namespace: e2e
spec:
  template:
    metadata:
      labels:
        app: my-service
    spec:
      serviceAccount: endpoints-watcher
      containers:
        - name: nginx
          image: nginx
      restartPolicy: Never
  backoffLimit: 0
```
