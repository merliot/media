package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/merliot/hub/pkg/ratelimit"
)

var (
	rl = ratelimit.New(ratelimit.Config{
		RateLimitWindow: 100 * time.Millisecond,
		MaxRequests:     30,
		BurstSize:       30,
		CleanupInterval: 1 * time.Minute,
	})
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

var page = `
<!DOCTYPE html>
<html>
	<head>
		<title>Metrics</title>
		<meta http-equiv="refresh" content="1">
	</head>
	<body>
		<h1>Metrics</h1>

		<p>Misses: {{.misses}}</p>

		<h2>Hits:</h2>
		<ul>
			{{ range $path, $count := .hits }}
				<li>{{$path}} {{$count}}</li>
			{{ end }}
		</ul>

		<h2>Clients:</h2>
		<ul>
			{{ range $id, $tokens := .stats }}
				<li>{{$id}} {{$tokens}}</li>
			{{ end }}
		</ul>
	</body>
</html>
`

func (m *metrics) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	m.RLock()
	defer m.RUnlock()

	data := make(map[string]any)

	data["misses"] = m.misses
	data["hits"] = m.hits
	data["stats"] = rl.Stats()

	tmpl, err := template.New("metrics").Parse(page)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
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
		f.Close()

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
	log.Fatal(http.ListenAndServe(":8000", rl.RateLimit(http.DefaultServeMux)))
}
