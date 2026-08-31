package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"regexp"
	"strings"
	"text/tabwriter"
)

// Hostnames are used as registry keys and shown in the host list, so keep them
// to something that could plausibly be one.
var validHost = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9.-]*[a-zA-Z0-9])?$`)

const (
	maxHostLen  = 253
	maxBodySize = 4096
)

//go:embed client
var clientFS embed.FS

// routes wires up the service. baseURL is this server's public address, which
// is substituted into the client files it hands out.
func routes(reg *registry, baseURL string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /register", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Host string `json:"host"`
			IP   string `json:"ip"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Host == "" || body.IP == "" {
			http.Error(w, "need host and ip", http.StatusBadRequest)
			return
		}
		if len(body.Host) > maxHostLen || !validHost.MatchString(body.Host) {
			http.Error(w, "invalid host", http.StatusBadRequest)
			return
		}
		if net.ParseIP(body.IP) == nil {
			http.Error(w, "invalid ip", http.StatusBadRequest)
			return
		}
		if err := reg.upsert(body.Host, body.IP); err != nil {
			log.Printf("registering %s: %v", body.Host, err)
			http.Error(w, "could not persist registration", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /hosts", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(reg.all()); err != nil {
			log.Printf("writing /hosts: %v", err)
		}
	})

	// Client files are templated on the way out so they point back at this
	// server; nothing downstream has to work out where the registry lives.
	mux.HandleFunc("GET /client/{file}", func(w http.ResponseWriter, r *http.Request) {
		body, err := fs.ReadFile(clientFS, "client/"+r.PathValue("file"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte(strings.ReplaceAll(string(body), "@@BASE_URL@@", baseURL)))
	})

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
		fmt.Fprint(tw, "HOST\tIP\tLAST SEEN\n")
		for _, e := range reg.all() {
			fmt.Fprintf(tw, "%s\t%s\t%s\n", e.Host, e.IP, e.UpdatedAt.Format("2006-01-02 15:04:05 UTC"))
		}
		tw.Flush()
		fmt.Fprintf(w, "\nRegister a host:\n  curl -fsSL %s/client/install.sh | sudo sh\n", baseURL)
	})

	return mux
}
