package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
)

type metrics struct {
	sync.RWMutex
	hits   map[string]int
	misses int
}

func newMetrics() *metrics {
	return &metrics{
		hits: make(map[string]int),
	}
}

func (m *metrics) hit(path string) {
	m.Lock()
	defer m.Unlock()
	m.hits[path]++
}

func (m *metrics) miss() {
	m.Lock()
	defer m.Unlock()
	m.misses++
}

func (m *metrics) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.RLock()
	defer m.RUnlock()

	fmt.Fprintf(w, "File Access metrics:\n")
	for path, count := range m.hits {
		fmt.Fprintf(w, "  %s: %d\n", path, count)
	}
	fmt.Fprintf(w, "Missed Hits: %d\n", m.misses)
}

func fileServerWithMetrics(metrics *metrics, fs http.FileSystem) http.Handler {
	fsh := http.StripPrefix("/", http.FileServer(fs))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

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
