#!/bin/bash

set -e

local_img=localhost:5000/e2e:$(date +%s)
img="${IMG:-$local_img}"

if [ "$IMG" == "" ]; then
    GOOS=linux go build -ldflags "-s -w" -o bin/e2e .
    docker build -t ${img} .
    docker push ${img}
fi

function log()
{
    echo "Test failed!"
    echo "Pods:"
    kubectl get pods -owide -n e2e
    echo "Logs:"
    kubectl logs -l app=e2e -n e2e
}
trap log ERR

kubectl delete ns e2e || true
kubectl create ns e2e

kubectl apply -f memcache.yaml -n e2e
kubectl wait pods -l app.kubernetes.io/name=memcache --for condition=Ready --timeout=60s  -n e2e

kubectl apply -f e2e.yaml --dry-run=client -oyaml | sed "s/replace-img/$(echo "$img" | sed 's/\//\\\//g')/" | kubectl apply -f - -n e2e
kubectl wait job/e2e --for=condition=complete --timeout=15s  -n e2e
