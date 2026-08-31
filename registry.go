package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

type entry struct {
	Host      string    `json:"host"`
	IP        string    `json:"ip"`
	UpdatedAt time.Time `json:"updated_at"`
}

// registry is the set of known hosts, persisted to a JSON file on every change.
type registry struct {
	mu      sync.RWMutex
	entries map[string]entry
	path    string
}

// loadRegistry reads the registry at path, creating its directory if needed.
// A missing file is not an error: it is how a fresh deployment starts.
func loadRegistry(path string) (*registry, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("creating data dir %s: %w", dir, err)
		}
	}

	r := &registry{path: path, entries: make(map[string]entry)}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return r, nil
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if err := json.Unmarshal(data, &r.entries); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return r, nil
}

// upsert records host at ip, overwriting any previous address for it.
func (r *registry) upsert(host, ip string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[host] = entry{Host: host, IP: ip, UpdatedAt: time.Now().UTC()}

	data, err := json.MarshalIndent(r.entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling registry: %w", err)
	}
	return replaceFile(r.path, append(data, '\n'))
}

// all returns every entry, most recently seen first.
func (r *registry) all() []entry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]entry, 0, len(r.entries))
	for _, e := range r.entries {
		out = append(out, e)
	}
	slices.SortFunc(out, func(a, b entry) int { return b.UpdatedAt.Compare(a.UpdatedAt) })
	return out
}

// replaceFile writes data to path via a temporary file and a rename, so an
// interrupted write cannot truncate the registry that is already on disk.
func replaceFile(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".registry-*.json")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(tmp.Name()) // no-op once the rename below succeeds

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}
	// Flush before renaming, so a crash cannot leave the registry pointing at a
	// file whose contents never landed.
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("syncing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("replacing %s: %w", path, err)
	}
	return nil
}
