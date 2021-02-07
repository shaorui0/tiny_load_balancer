package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"
)

func TestConst(t *testing.T) {
	if Attempts != 0 {
		t.Errorf("Attempts: %d", Attempts)
	}
	if Retry != 1 {
		t.Errorf("Retry: %d", Retry)
	}
}

func TestGetRetryFromContext(t *testing.T) {
	req, err := http.NewRequest("GET", "https://www.baidu.com", nil)
	if err != nil {
		t.Errorf("%v", err)
	}

	retries := 0
	ctx := context.WithValue(req.Context(), Retry, retries+1) // set

	req = req.WithContext(ctx)
	ans := GetRetryFromContext(req)
	if ans != 1 {
		t.Errorf("req.Context().Value(Retry).(int) = %d; want 1", ans)
	}
}

func TestGetAttemptsFromContext(t *testing.T) {
	req, err := http.NewRequest("GET", "https://www.baidu.com", nil)
	if err != nil {
		t.Errorf("%v", err)
	}

	attempts := 0
	ctx := context.WithValue(req.Context(), Attempts, attempts+1) // set

	req = req.WithContext(ctx)
	ans := GetAttemptsFromContext(req)
	if ans != 1 {
		t.Errorf("req.Context().Value(Attempts).(int) = %d; want 1", ans)
	}
}

func TestBackendAlive(t *testing.T) {
	var b Backend

	b.SetAlive(true)
	if b.IsAlive() != true {
		t.Errorf("b.IsAlive() == true")
	}

	b.SetAlive(false)
	if b.IsAlive() != false {
		t.Errorf("b.IsAlive() == false")
	}
}

func TestBackendReverseProxy(t *testing.T) {
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "this call was relayed by the reverse proxy")
	}))
	defer backendServer.Close()

	rpURL, err := url.Parse(backendServer.URL)
	if err != nil {
		log.Fatal(err)
	}

	b := Backend{
		URL:          rpURL,
		Alive:        true,
		ReverseProxy: httputil.NewSingleHostReverseProxy(rpURL),
	}

	frontendProxy := httptest.NewServer(b.ReverseProxy)
	defer frontendProxy.Close()

	resp, err := http.Get(frontendProxy.URL) // run http get, get response
	if err != nil {
		log.Fatal(err)
	}

	// fmt.Println(resp.Header) // header
	// fmt.Println(frontendProxy.URL)
	body, err := ioutil.ReadAll(resp.Body) // body, ioutil.ReadAll
	if err != nil {
		log.Fatal(err)
	}

	// len(string(body)) == 43?
	if string(body)[:42] != string("this call was relayed by the reverse proxy") {
		t.Errorf("%s", string(body))
		t.Errorf("%s", string("this call was relayed by the reverse proxy"))
	}
}

func TestaddBackendToPool(t *testing.T) {
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "this call was relayed by the reverse proxy")
	}))
	defer backendServer.Close()
	URL, err := url.Parse(backendServer.URL)
	if err != nil {
		log.Fatal(err)
	}

	var serverPool ServerPool
	serverPool.AddBackend(&Backend{
		URL:          URL,
		Alive:        true,
		ReverseProxy: httputil.NewSingleHostReverseProxy(URL),
	})
	if len(serverPool.backends) != 1 {
		t.Errorf("xxx")
	}
}

func TestNextIndex(t *testing.T) {

	var serverPool ServerPool
	var backend Backend
	var backend2 Backend
	serverPool.AddBackend(&backend)
	serverPool.AddBackend(&backend2)
	fmt.Printf(">>> %d", len(serverPool.backends))
	serverPool.NextIndex()
	if serverPool.current != 1 {
		t.Errorf("%d", serverPool.current)
	}
	serverPool.NextIndex()
	if serverPool.current != 2 {
		t.Errorf("%d", serverPool.current)
	}
	serverPool.NextIndex()
	if serverPool.current != 3 {
		t.Errorf("%d", serverPool.current)
	}
}

func GetNewBackend() *url.URL {
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "this call was relayed by the reverse proxy")
	}))
	defer backendServer.Close()
	URL, err := url.Parse(backendServer.URL)
	if err != nil {
		log.Fatal(err)
	}
	return URL
}

func TestMarkBackendStatus(t *testing.T) {
	var serverPool ServerPool

	URL1 := GetNewBackend()
	URL2 := GetNewBackend()

	serverPool.AddBackend(&Backend{
		URL: URL1,
	})
	serverPool.AddBackend(&Backend{
		URL: URL2,
	})

	serverPool.MarkBackendStatus(URL1, false)
	serverPool.MarkBackendStatus(URL2, true)

	for _, b := range serverPool.backends {
		if b.URL.String() == URL1.String() {
			if b.Alive != false {
				t.Errorf("%s: %t", b.URL.String(), b.Alive)
			}
		} else if b.URL.String() == URL2.String() {
			if b.Alive != true {
				t.Errorf("%s: %t", b.URL.String(), b.Alive)
			}
		}
	}
}

func TestIsBackendAlive(t *testing.T) {
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "this call was relayed by the reverse proxy")
	}))
	defer backendServer.Close()
	URL, err := url.Parse(backendServer.URL)
	if err != nil {
		log.Fatal(err)
	}

	result := isBackendAlive(URL)
	if result != true {
		t.Errorf("Server has stopped running")
	}
}

func TestIsBackendAliveNot(t *testing.T) {
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "this call was relayed by the reverse proxy")
	}))
	// defer backendServer.Close()
	URL, err := url.Parse(backendServer.URL)
	if err != nil {
		log.Fatal(err)
	}
	backendServer.Close()
	result := isBackendAlive(URL)
	if result != false {
		t.Errorf("Server is still running")
	}
}

func TestHealthCheck(t *testing.T) {
	var serverPool ServerPool

	backendServer1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "this call was relayed by the reverse proxy")
	}))
	defer backendServer1.Close()
	URL1, err := url.Parse(backendServer1.URL)
	if err != nil {
		log.Fatal(err)
	}

	backendServer2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "this call was relayed by the reverse proxy")
	}))
	defer backendServer2.Close()
	URL2, err := url.Parse(backendServer2.URL)
	if err != nil {
		log.Fatal(err)
	}

	serverPool.AddBackend(&Backend{
		URL: URL1,
	})
	serverPool.AddBackend(&Backend{
		URL: URL2,
	})

	serverPool.HealthCheck()
	backendServer2.Close()
	serverPool.HealthCheck() // server 2 has down
}

func TestGetNextPeer(t *testing.T) {
	var serverPool ServerPool

	backendServer1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "this call was relayed by the reverse proxy")
	}))
	defer backendServer1.Close()
	URL1, err := url.Parse(backendServer1.URL)
	if err != nil {
		log.Fatal(err)
	}

	backendServer2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "this call was relayed by the reverse proxy")
	}))
	defer backendServer2.Close()
	URL2, err := url.Parse(backendServer2.URL)
	if err != nil {
		log.Fatal(err)
	}

	backendServer3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "this call was relayed by the reverse proxy")
	}))
	defer backendServer3.Close()
	URL3, err := url.Parse(backendServer3.URL)
	if err != nil {
		log.Fatal(err)
	}

	serverPool.AddBackend(&Backend{
		URL: URL1,
	})
	serverPool.AddBackend(&Backend{
		URL: URL2,
	})
	serverPool.AddBackend(&Backend{
		URL: URL3,
	})
	serverPool.HealthCheck() // init isAlive status
	println(serverPool.getNextPeer().URL.String(), serverPool.current)
	println(serverPool.getNextPeer().URL.String(), serverPool.current)
	println(serverPool.getNextPeer().URL.String(), serverPool.current)

	backendServer2.Close()
	backendServer3.Close()
	serverPool.HealthCheck() // re-init isAlive status
	if URL1.String() != serverPool.getNextPeer().URL.String() {
		t.Errorf("only server 1 is running.")
	}
	backendServer1.Close()
}

func TestGetNextPeerSRR(t *testing.T) {
	var serverPool ServerPool

	backendServer1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "this call was relayed by the reverse proxy")
	}))
	defer backendServer1.Close()
	URL1, err := url.Parse(backendServer1.URL)
	if err != nil {
		log.Fatal(err)
	}

	backendServer2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "this call was relayed by the reverse proxy")
	}))
	defer backendServer2.Close()
	URL2, err := url.Parse(backendServer2.URL)
	if err != nil {
		log.Fatal(err)
	}

	backendServer3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "this call was relayed by the reverse proxy")
	}))
	defer backendServer3.Close()
	URL3, err := url.Parse(backendServer3.URL)
	if err != nil {
		log.Fatal(err)
	}

	serverPool.AddBackend(&Backend{
		URL:             URL1,
		weight:          4,
		effectiveWeight: 4,
		currentWeight:   0,
	})
	serverPool.AddBackend(&Backend{
		URL:             URL2,
		weight:          2,
		effectiveWeight: 2,
		currentWeight:   0,
	})
	serverPool.AddBackend(&Backend{
		URL:             URL3,
		weight:          1,
		effectiveWeight: 1,
		currentWeight:   0,
	})

	expect := [...]string{"a", "b", "a", "c", "a", "b", "a"}
	m := make(map[string]string)
	m[URL1.String()] = "a"
	m[URL2.String()] = "b"
	m[URL3.String()] = "c"

	idx := 0
	for {
		if m[serverPool.getNextPeerSRR().URL.String()] != expect[idx] {
			t.Errorf("TestGetNextPeerSRR")
		}

		if serverPool.backends[0].currentWeight == 0 && serverPool.backends[1].currentWeight == 0 && serverPool.backends[2].currentWeight == 0 {
			break
		}
		idx++
	}
}

func TestLBAttemptsMoreThanThree(t *testing.T) {
	// backend downed => errorhandler => attempts==4 => http.Error
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "this call was relayed by the reverse proxy")
	}))
	URL, err := url.Parse(backendServer.URL)
	if err != nil {
		log.Fatal(err)
	}

	// defer backendServer.Close()
	backendServer.Close() // open ErrorHandler

	proxy := httputil.NewSingleHostReverseProxy(URL)
	proxy.ErrorHandler = func(writer http.ResponseWriter, request *http.Request, e error) {
		// attempts := GetAttemptsFromContext(request)
		attempts := 4
		log.Printf("%s(%s) Attempting retry %d\n", request.RemoteAddr, request.URL.Path, attempts)
		ctx := context.WithValue(request.Context(), Attempts, attempts+1)
		// attempts < 3, dead loop, recurse
		lb(writer, request.WithContext(ctx))
	}

	b := Backend{
		URL:          URL,
		Alive:        false,
		ReverseProxy: proxy,
	}
	serverPool.AddBackend(&b)

	frontendProxy := httptest.NewServer(b.ReverseProxy)
	defer frontendProxy.Close()

	req, err := http.NewRequest("GET", frontendProxy.URL, nil)
	if err != nil {
		t.Errorf("%v", err)
	}

	client := http.Client{}
	client.Do(req)
}

func TestGoHealthCheck(t *testing.T) {
	go healthCheck() // run normally
}
