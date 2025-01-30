package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sort"
	"sync"
)

type metrics struct {
	sync.RWMutex
	hits       map[string]int
	misses     int
	clientIPs  map[string]int
	maxPathLen int
}

func newMetrics() *metrics {
	return &metrics{
		hits:      make(map[string]int),
		clientIPs: make(map[string]int),
	}
}

func (m *metrics) hit(path string) {
	m.Lock()
	defer m.Unlock()
	m.hits[path]++
	if len(path) > m.maxPathLen {
		m.maxPathLen = len(path)
	}

}

func (m *metrics) miss() {
	m.Lock()
	defer m.Unlock()
	m.misses++
}

func (m *metrics) clients(ip string) {
	m.Lock()
	defer m.Unlock()
	m.clientIPs[ip]++
}

func (m *metrics) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.RLock()
	defer m.RUnlock()

	paths := make([]string, 0, len(m.hits))
	for path := range m.hits {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	fmt.Fprintf(w, "Misses: %d\n\n", m.misses)

	fmt.Fprintf(w, "Hits:\n\n")
	for _, path := range paths {
		count := m.hits[path]
		fmt.Fprintf(w, "  %-*s     %d\n", m.maxPathLen, path, count)
	}

	ips := make([]string, 0, len(m.clientIPs))
	for ip := range m.clientIPs {
		ips = append(ips, ip)
	}
	sort.Strings(ips)

	fmt.Fprintf(w, "\nClients:\n\n")
	for _, ip := range ips {
		count := m.clientIPs[ip]
		fmt.Fprintf(w, "  %-40s     %d\n", ip, count)
	}
}

func fileServerWithMetrics(metrics *metrics, fs http.FileSystem) http.Handler {
	fsh := http.StripPrefix("/", http.FileServer(fs))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		metrics.clients(ip)

		f, err := fs.Open(path)
		if err != nil {
			if os.IsNotExist(err) {
				metrics.miss()
				http.Error(w, "File not found", http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		defer f.Close()

		metrics.hit(path)
		fsh.ServeHTTP(w, r)
	})
}

func main() {
	fs := http.Dir("./assets")
	metrics := newMetrics()
	fileServer := fileServerWithMetrics(metrics, fs)
	http.Handle("/", fileServer)
	http.Handle("/metrics", metrics)
	fmt.Println("Starting server on :8000")
	log.Fatal(http.ListenAndServe(":8000", nil))
}
