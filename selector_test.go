package selector

import (
	"context"
	"testing"
	"time"

	kubernetes "github.com/castai/k8s-client-go"
)

func TestServerList(t *testing.T) {
	endpoints := &mockEndpoints{
		get: &kubernetes.Endpoints{
			Subsets: []kubernetes.Subset{
				{
					Addresses: []kubernetes.Address{
						{
							IP: "10.10.0.15",
						},
					},
					Ports: []kubernetes.Port{
						{
							Port: 11211,
						},
					},
				},
			},
		},
		watch: &endpointsWatch{
			ch: make(chan kubernetes.Event[*kubernetes.Endpoints]),
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	s, err := startServerList(ctx, "memcache:11211", &defaultLogger{}, endpoints)
	if err != nil {
		t.Fatal(err)
	}

	addr, err := s.PickServer("key1")
	if err != nil {
		t.Fatal(err)
	}
	expectedAddr := "10.10.0.15:11211"
	if addr.String() != expectedAddr {
		t.Fatalf("expected addr %s, actual %s", expectedAddr, addr.String())
	}
}

func TestParseTargetInfo(t *testing.T) {
	tests := []struct {
		in  string
		out targetInfo
	}{
		{
			in: "memcache:11211",
			out: targetInfo{
				namespace: getCurrentNamespaceOrDefault(),
				name:      "memcache",
				port:      "11211",
				namedPort: false,
			},
		},
		{
			in: "memcache.my-namespace:11211",
			out: targetInfo{
				namespace: "my-namespace",
				name:      "memcache",
				port:      "11211",
				namedPort: false,
			},
		},
		{
			in: "memcache.my-namespace:named-port",
			out: targetInfo{
				namespace: "my-namespace",
				name:      "memcache",
				port:      "named-port",
				namedPort: true,
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.in, func(t *testing.T) {
			out, err := parseTargetInfo(test.in)
			if err != nil {
				t.Fatal(err)
			}
			if test.out != out {
				t.Fatalf("expected %+v, actual %+v", test.out, out)
			}
		})
	}
}

type mockEndpoints struct {
	get   *kubernetes.Endpoints
	watch *endpointsWatch
}

func (m *mockEndpoints) Get(ctx context.Context, namespace, name string, _ kubernetes.GetOptions) (*kubernetes.Endpoints, error) {
	return m.get, nil
}

func (m *mockEndpoints) Watch(ctx context.Context, namespace, name string, _ kubernetes.ListOptions) (kubernetes.WatchInterface[*kubernetes.Endpoints], error) {
	return m.watch, nil
}

type endpointsWatch struct {
	ch chan kubernetes.Event[*kubernetes.Endpoints]
}

func (e endpointsWatch) Stop() {
	close(e.ch)
}

func (e endpointsWatch) ResultChan() <-chan kubernetes.Event[*kubernetes.Endpoints] {
	return e.ch
}
