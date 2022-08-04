package selector

import (
	"context"
	"errors"
	"fmt"
	"hash/crc32"
	"io/ioutil"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	kubernetes "github.com/castai/k8s-client-go"
)

const (
	kubernetesNamespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	defaultNamespace        = "default"
)

var (
	ErrNoServers = errors.New("k8s-memcache-selector: no servers configured or available")
)

func WithLogger(log Logger) Option {
	return func(o *options) {
		o.log = log
	}
}

type Option func(o *options)
type options struct {
	log Logger
}

func NewServerList(ctx context.Context, addr string, opt ...Option) (*ServerList, error) {
	if addr == "" {
		return nil, errors.New("address is required")
	}

	opts := options{
		log: &defaultLogger{},
	}
	for _, o := range opt {
		o(&opts)
	}

	kc, err := kubernetes.NewInCluster()
	if err != nil {
		return nil, err
	}
	endpoints := kubernetes.NewEndpointsOperator(kc)

	ss, err := startServerList(ctx, addr, opts.log, endpoints)
	if err != nil {
		return nil, err
	}
	return ss, nil
}

func startServerList(ctx context.Context, addr string, log Logger, endpoints kubernetes.ObjectOperator[*kubernetes.Endpoints]) (*ServerList, error) {
	target, err := parseTargetInfo(addr)
	if err != nil {
		return nil, fmt.Errorf("parsing target info: %w", err)
	}
	ss := &ServerList{
		log:       log,
		endpoints: endpoints,
	}
	if err := ss.setInitialServers(ctx, target); err != nil {
		return nil, fmt.Errorf("setting initial servers: %w", err)
	}
	go ss.startDiscovery(ctx, target)

	return ss, nil
}

// ServerList implements memcache ServerSelector interface. Under the hood kubernetes endpoints are used for discovery of memcache server ips.
// See https://github.com/bradfitz/gomemcache/blob/master/memcache/selector.go
type ServerList struct {
	log       Logger
	endpoints kubernetes.ObjectOperator[*kubernetes.Endpoints]
	mu        sync.RWMutex
	addrs     []net.Addr
}

// Each iterates over each server calling the given function
func (ss *ServerList) Each(f func(net.Addr) error) error {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	for _, a := range ss.addrs {
		if err := f(a); nil != err {
			return err
		}
	}
	return nil
}

// keyBufPool returns []byte buffers for use by PickServer's call to
// crc32.ChecksumIEEE to avoid allocations. (but doesn't avoid the
// copies, which at least are bounded in size and small)
var keyBufPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 256)
		return &b
	},
}

func (ss *ServerList) PickServer(key string) (net.Addr, error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	if len(ss.addrs) == 0 {
		return nil, ErrNoServers
	}
	if len(ss.addrs) == 1 {
		return ss.addrs[0], nil
	}
	bufp := keyBufPool.Get().(*[]byte)
	n := copy(*bufp, key)
	cs := crc32.ChecksumIEEE((*bufp)[:n])
	keyBufPool.Put(bufp)

	return ss.addrs[cs%uint32(len(ss.addrs))], nil
}

func (ss *ServerList) setInitialServers(ctx context.Context, target targetInfo) error {
	endpoints, err := ss.endpoints.Get(ctx, target.namespace, target.name, kubernetes.GetOptions{})
	if err != nil {
		return fmt.Errorf("fetching endpoints: %w", err)
	}
	ss.updateServers(endpoints, target)
	return nil
}

func (ss *ServerList) startDiscovery(ctx context.Context, target targetInfo) {
	until(func() {
		events, err := ss.endpoints.Watch(ctx, target.namespace, target.name, kubernetes.ListOptions{})
		if err != nil {
			ss.log.Errorf("k8s-memcache-selector: endpoints watch failed, will retry: %v", err)
			return
		}

		for {
			select {
			case <-ctx.Done():
				return
			case e, more := <-events.ResultChan():
				if !more {
					return
				}
				ss.updateServers(e.Object, target)
			}
		}

	}, 1*time.Second, ctx.Done())
}

func (ss *ServerList) updateServers(endpoints *kubernetes.Endpoints, target targetInfo) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	ss.addrs = []net.Addr{}

	for _, s := range endpoints.Subsets {
		// Find final port.
		var addrPort int
		for _, port := range s.Ports {
			if target.namedPort {
				if port.Name == target.port {
					addrPort = port.Port
					break
				}
			} else if strconv.Itoa(port.Port) == target.port {
				addrPort = port.Port
				break
			}
		}

		for _, addr := range s.Addresses {
			ss.addrs = append(ss.addrs, &net.TCPAddr{
				IP:   net.ParseIP(addr.IP),
				Port: addrPort,
			})
		}
	}

	addrs := make([]string, len(ss.addrs))
	for i, addr := range ss.addrs {
		addrs[i] = addr.String()
	}

	ss.log.Infof("k8s-memcache-selector: updated server addresses: %s", strings.Join(addrs, ","))
}

func until(fn func(), sleep time.Duration, done <-chan struct{}) {
	select {
	case <-done:
		return
	default:
	}

	for {
		fn()
		select {
		case <-done:
			return
		case <-time.After(sleep):
		}
	}
}

type targetInfo struct {
	namespace string
	name      string
	port      string
	namedPort bool
}

func parseTargetInfo(u string) (targetInfo, error) {
	host, port, err := net.SplitHostPort(u)
	if err != nil {
		return targetInfo{}, err
	}

	hostParts := strings.SplitN(host, ".", 2)
	name := ""
	namespace := getCurrentNamespaceOrDefault()
	if len(hostParts) == 2 {
		name = hostParts[0]
		namespace = hostParts[1]
	} else {
		name = hostParts[0]
	}

	namedPort := false
	if _, err := strconv.Atoi(port); err != nil {
		namedPort = true
	}

	return targetInfo{
		namespace: namespace,
		name:      name,
		port:      port,
		namedPort: namedPort,
	}, nil
}

func getCurrentNamespaceOrDefault() string {
	ns, err := ioutil.ReadFile(kubernetesNamespaceFile)
	if err != nil {
		return defaultNamespace
	}
	return string(ns)
}
