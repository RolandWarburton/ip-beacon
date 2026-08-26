package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Entry struct {
	Host      string    `json:"host"`
	IP        string    `json:"ip"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Registry struct {
	mu      sync.RWMutex
	Entries map[string]Entry `json:"entries"`
	path    string
}

func loadRegistry(path string) *Registry {
	r := &Registry{path: path, Entries: make(map[string]Entry)}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("warning: could not read %s: %v", path, err)
		}
		return r
	}
	if err := json.Unmarshal(data, &r.Entries); err != nil {
		log.Printf("warning: could not parse %s: %v", path, err)
	}
	return r
}

func (r *Registry) save() {
	data, err := json.MarshalIndent(r.Entries, "", "  ")
	if err != nil {
		log.Printf("error marshalling registry: %v", err)
		return
	}
	if err := os.WriteFile(r.path, data, 0644); err != nil {
		log.Printf("error saving registry: %v", err)
	}
}

func (r *Registry) upsert(host, ip string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Entries[host] = Entry{Host: host, IP: ip, UpdatedAt: time.Now().UTC()}
	r.save()
}

func (r *Registry) all() []Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Entry, 0, len(r.Entries))
	for _, e := range r.Entries {
		out = append(out, e)
	}
	return out
}

var indexTmpl = template.Must(template.New("index").Funcs(template.FuncMap{
	"fmtTime": func(t time.Time) string {
		return t.Format("2006-01-02 15:04:05 UTC")
	},
}).Parse(`<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>IP Registry</title>
  <style>
    body { font-family: monospace; max-width: 800px; margin: 40px auto; padding: 0 20px; background: #111; color: #eee; }
    h1 { color: #7af; }
    table { width: 100%; border-collapse: collapse; margin-top: 20px; }
    th { text-align: left; border-bottom: 2px solid #444; padding: 8px 12px; color: #7af; }
    td { padding: 8px 12px; border-bottom: 1px solid #333; }
    tr:hover td { background: #1a1a1a; }
    .empty { color: #666; margin-top: 20px; }
  </style>
</head>
<body>
  <h1>IP Registry</h1>
  {{if .}}
  <table>
    <thead><tr><th>Host</th><th>IP</th><th>Last Seen</th></tr></thead>
    <tbody>
    {{range .}}
      <tr><td>{{.Host}}</td><td>{{.IP}}</td><td>{{fmtTime .UpdatedAt}}</td></tr>
    {{end}}
    </tbody>
  </table>
  {{else}}
  <p class="empty">No hosts registered yet.</p>
  {{end}}
</body>
</html>`))

func main() {
	dataPath := os.Getenv("DATA_PATH")
	if dataPath == "" {
		dataPath = "data/registry.json"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	if dir := filepath.Dir(dataPath); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("could not create data dir %s: %v", dir, err)
		}
	}

	reg := loadRegistry(dataPath)
	log.Printf("loaded %d entries from %s", len(reg.Entries), dataPath)

	http.HandleFunc("POST /register", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Host string `json:"host"`
			IP   string `json:"ip"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Host == "" || body.IP == "" {
			http.Error(w, `{"error":"invalid body, need host and ip"}`, http.StatusBadRequest)
			return
		}
		reg.upsert(body.Host, body.IP)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"ok":true}`)
	})

	http.HandleFunc("GET /hosts", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(reg.all())
	})

	http.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := indexTmpl.Execute(w, reg.all()); err != nil {
			log.Printf("template error: %v", err)
		}
	})

	log.Printf("listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
