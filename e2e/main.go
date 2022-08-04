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

	if err := cache.Set(&memcache.Item{
		Key:   "key1",
		Value: []byte("val"),
	}); err != nil {
		log.Fatalf("adding cache item: %v", err)
	}

	_, err = cache.Get("key1")
	if err != nil {
		log.Fatalf("getting item: %v", err)
	}
	log.Println("done")
}
