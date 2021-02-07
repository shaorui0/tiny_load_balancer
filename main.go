package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	Attempts int = iota
	Retry
)

var serverPool ServerPool // global

func GetRetryFromContext(r *http.Request) int {
	if retry, ok := r.Context().Value(Retry).(int); ok {
		return retry
	}
	return 0
}
func GetAttemptsFromContext(r *http.Request) int {
	if attempts, ok := r.Context().Value(Attempts).(int); ok {
		return attempts
	}
	return 1
}

type Backend struct {
	URL             *url.URL
	Alive           bool
	mux             sync.RWMutex
	ReverseProxy    *httputil.ReverseProxy
	weight          int
	currentWeight   int
	effectiveWeight int
}

func (b *Backend) SetAlive(alive bool) {
	b.mux.Lock()
	b.Alive = alive
	b.mux.Unlock()
}

func (b *Backend) IsAlive() (alive bool) {
	b.mux.RLock()
	alive = b.Alive
	b.mux.RUnlock()
	return
}

type ServerPool struct {
	backends []*Backend
	current  uint64
}

func (pool *ServerPool) AddBackend(b *Backend) {
	pool.backends = append(pool.backends, b)
}

func (s *ServerPool) NextIndex() int {
	return int(atomic.AddUint64(&s.current, uint64(1)) % uint64(len(s.backends)))
}

func (s *ServerPool) MarkBackendStatus(backendUrl *url.URL, alive bool) {
	for _, b := range s.backends {
		if b.URL.String() == backendUrl.String() {
			b.SetAlive(alive)
			break
		}
	}
}

func isBackendAlive(backendUrl *url.URL) bool {
	timeout := 2 * time.Second
	conn, err := net.DialTimeout("tcp", backendUrl.Host, timeout)
	if err != nil {
		log.Println("Site unreachable, error: ", err)
		return false
	}
	_ = conn.Close()
	return true
}

func (s *ServerPool) HealthCheck() {
	for _, b := range s.backends {
		isAlive := isBackendAlive(b.URL)
		b.SetAlive(isAlive)
		if isAlive == false {
			status := "down"
			log.Printf("%s [%s]\n", b.URL, status)
		}
	}
}

func healthCheck() {
	t := time.NewTicker(time.Minute * 2)
	for {
		select {
		case <-t.C:
			log.Println("Starting health check...")
			serverPool.HealthCheck()
			log.Println("Health check completed")
		}
	}
}

// simple RR
func (s *ServerPool) getNextPeer() *Backend {
	next := int(s.NextIndex())
	numBackends := len(s.backends)
	l := numBackends + next
	for i := next; i < l; i++ {
		idx := i % numBackends
		if s.backends[idx].IsAlive() {
			if i != next {
				atomic.StoreUint64(&s.current, uint64(idx))
			}
			return s.backends[idx]
		}
	}
	return nil
}

// smooth RR
func (s *ServerPool) getNextPeerSRR() *Backend {
	total := 0
	var best *Backend
	best = nil

	for idx := 0; idx < len(s.backends); idx++ {
		if s.backends[idx].IsAlive() == false {
			continue
		}

		s.backends[idx].currentWeight += s.backends[idx].effectiveWeight
		total += s.backends[idx].effectiveWeight

		if s.backends[idx].effectiveWeight < s.backends[idx].weight {
			s.backends[idx].effectiveWeight++
		}

		if best == nil || s.backends[idx].currentWeight > best.currentWeight {
			best = s.backends[idx]
		}
	}

	if best == nil {
		return nil
	}
	best.currentWeight -= total

	logWeightRecord(s, best)

	return best
}

func logWeightRecord(s *ServerPool, best *Backend) {
	for idx := 0; idx < len(s.backends); idx++ {
		log.Printf(" weight: %d", s.backends[idx].weight)
	}
	for idx := 0; idx < len(s.backends); idx++ {
		log.Printf(" currentWeight: %d", s.backends[idx].currentWeight)
	}
	for idx := 0; idx < len(s.backends); idx++ {
		log.Printf(" effectiveWeight: %d", s.backends[idx].effectiveWeight)
	}
	log.Printf("Current url of backend: %s", best.URL.String())
}

func (backend *Backend) SRRFailHandle() {
	// 在连接后端时，如果发现和后端的通信过程中发生了错误，则减小 effectiveWeight
	// 此后有新的请求过来时，在选取后端的过程中，再逐步增加 effectiveWeight ，最终又恢复到 weight。
	backend.effectiveWeight -= backend.weight
	if backend.effectiveWeight <= 0 {
		backend.effectiveWeight = 0
	}
}

func (s *ServerPool) MarkBackendFail(backendUrl *url.URL) {
	for _, b := range s.backends {
		if b.URL.String() == backendUrl.String() {
			b.SetAlive(false)
			b.SRRFailHandle()
			break
		}
	}
}

func lb(w http.ResponseWriter, r *http.Request) {
	attempts := GetAttemptsFromContext(r)
	if attempts > 3 {
		log.Printf("%s(%s) Max attempts reached, terminating\n", r.RemoteAddr, r.URL.Path)
		http.Error(w, "Service not available", http.StatusServiceUnavailable)
		return
	}

	peer := serverPool.getNextPeerSRR()
	if peer != nil {
		peer.ReverseProxy.ServeHTTP(w, r)
		return
	}
	http.Error(w, "Service not available", http.StatusServiceUnavailable)
}

func main() {
	// TODO get server url from argv
	backendServer1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "this call was relayed by the reverse proxy1")
	}))
	backendServer2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "this call was relayed by the reverse proxy2")
	}))
	backendServer3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "this call was relayed by the reverse proxy3")
	}))

	serverList := fmt.Sprintf("%s,%s,%s", backendServer1.URL, backendServer2.URL, backendServer3.URL)

	// TODO config, test case
	weightMap := make(map[string]int)
	weightMap[backendServer1.URL] = 4
	weightMap[backendServer2.URL] = 2
	weightMap[backendServer3.URL] = 1

	tokens := strings.Split(serverList, ",")
	for _, tok := range tokens {
		serverUrl, err := url.Parse(tok)
		if err != nil {
			log.Fatal(err)
		}

		// init proxy
		proxy := httputil.NewSingleHostReverseProxy(serverUrl)
		proxy.ErrorHandler = func(writer http.ResponseWriter, request *http.Request, e error) {
			// retry, same backend, run three times
			log.Printf("[%s] %s\n", serverUrl.Host, e.Error())
			retries := GetRetryFromContext(request)
			if retries < 3 {
				select {
				case <-time.After(10 * time.Millisecond):
					ctx := context.WithValue(request.Context(), Retry, retries+1)
					proxy.ServeHTTP(writer, request.WithContext(ctx))
				}
				return
			}

			// set server down
			// serverPool.MarkBackendStatus(serverUrl, false)
			serverPool.MarkBackendFail(serverUrl)

			// attempts, next backend
			attempts := GetAttemptsFromContext(request)
			log.Printf("%s(%s) Attempting retry %d\n", request.RemoteAddr, request.URL.Path, attempts)
			ctx := context.WithValue(request.Context(), Attempts, attempts+1)
			lb(writer, request.WithContext(ctx))
		}

		serverPool.AddBackend(&Backend{
			URL:             serverUrl,
			Alive:           true,
			ReverseProxy:    proxy,
			weight:          weightMap[tok],
			effectiveWeight: weightMap[tok],
			currentWeight:   0,
		})
		log.Printf("Configured server: %s\n", serverUrl)
	}

	port := 15000
	server := http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: http.HandlerFunc(lb),
	}

	go healthCheck()

	log.Printf("Load Balancer started at :%d\n", port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
