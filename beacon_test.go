package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

const testBaseURL = "https://beacon.example.com"

func newTestServer(t *testing.T) (http.Handler, *registry) {
	t.Helper()
	reg := newTestRegistry(t)
	return routes(reg, testBaseURL), reg
}

func newTestRegistry(t *testing.T) *registry {
	t.Helper()
	reg, err := loadRegistry(filepath.Join(t.TempDir(), "sub", "registry.json"))
	if err != nil {
		t.Fatalf("loadRegistry: %v", err)
	}
	return reg
}

func do(t *testing.T, h http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestRegisterValidation(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
	}{
		{"valid ipv4", `{"host":"nas","ip":"10.0.0.5"}`, http.StatusNoContent},
		{"valid ipv6", `{"host":"nas","ip":"fe80::1"}`, http.StatusNoContent},
		{"dotted host", `{"host":"nas.local","ip":"10.0.0.5"}`, http.StatusNoContent},
		{"hyphenated host", `{"host":"my-nas","ip":"10.0.0.5"}`, http.StatusNoContent},
		{"malformed json", `{`, http.StatusBadRequest},
		{"empty body", ``, http.StatusBadRequest},
		{"missing host", `{"ip":"10.0.0.5"}`, http.StatusBadRequest},
		{"missing ip", `{"host":"nas"}`, http.StatusBadRequest},
		{"host too long", `{"host":"` + strings.Repeat("a", 254) + `","ip":"10.0.0.5"}`, http.StatusBadRequest},
		{"host with slash", `{"host":"a/b","ip":"10.0.0.5"}`, http.StatusBadRequest},
		{"host with space", `{"host":"a b","ip":"10.0.0.5"}`, http.StatusBadRequest},
		{"host leading hyphen", `{"host":"-nas","ip":"10.0.0.5"}`, http.StatusBadRequest},
		{"not an ip", `{"host":"nas","ip":"not-an-ip"}`, http.StatusBadRequest},
		{"hostname as ip", `{"host":"nas","ip":"nas.local"}`, http.StatusBadRequest},
		{"oversized body", `{"host":"nas","ip":"10.0.0.5","pad":"` + strings.Repeat("x", maxBodySize*2) + `"}`, http.StatusBadRequest},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := newTestServer(t)
			if got := do(t, h, "POST", "/register", tc.body).Code; got != tc.want {
				t.Errorf("status = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestRegisterThenListRoundTrip(t *testing.T) {
	h, _ := newTestServer(t)
	do(t, h, "POST", "/register", `{"host":"nas","ip":"10.0.0.5"}`)
	do(t, h, "POST", "/register", `{"host":"nas","ip":"10.0.0.9"}`)

	var got []entry
	if err := json.Unmarshal(do(t, h, "GET", "/hosts", "").Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding /hosts: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1 (re-registering must update, not duplicate)", len(got))
	}
	if got[0].IP != "10.0.0.9" {
		t.Errorf("IP = %q, want the most recent 10.0.0.9", got[0].IP)
	}
}

func TestHostsAreSortedMostRecentFirst(t *testing.T) {
	h, _ := newTestServer(t)
	for _, host := range []string{"first", "second", "third"} {
		do(t, h, "POST", "/register", fmt.Sprintf(`{"host":%q,"ip":"10.0.0.1"}`, host))
	}

	var got []entry
	json.Unmarshal(do(t, h, "GET", "/hosts", "").Body.Bytes(), &got)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	for i := 1; i < len(got); i++ {
		if got[i-1].UpdatedAt.Before(got[i].UpdatedAt) {
			t.Errorf("entry %d is newer than entry %d; want most recent first", i, i-1)
		}
	}
}

func TestRoutingConstraints(t *testing.T) {
	h, _ := newTestServer(t)
	for _, tc := range []struct {
		method, target string
		want           int
	}{
		{"GET", "/register", http.StatusMethodNotAllowed},
		{"POST", "/hosts", http.StatusMethodNotAllowed},
		// The index is registered at /{$}, so it must not swallow unknown paths.
		{"GET", "/nope", http.StatusNotFound},
	} {
		if got := do(t, h, tc.method, tc.target, "").Code; got != tc.want {
			t.Errorf("%s %s = %d, want %d", tc.method, tc.target, got, tc.want)
		}
	}
}

func TestIndexListsHostsAndInstallCommand(t *testing.T) {
	h, _ := newTestServer(t)
	do(t, h, "POST", "/register", `{"host":"nas","ip":"10.0.0.5"}`)

	w := do(t, h, "GET", "/", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{"nas", "10.0.0.5", testBaseURL + "/client/install.sh"} {
		if !strings.Contains(body, want) {
			t.Errorf("index does not mention %q:\n%s", want, body)
		}
	}
}

// Every placeholder must be substituted before a file leaves the server; an
// unsubstituted one reaches the client as a literal @@BASE_URL@@ and breaks it.
// The scripts' own "@@*@@" guard globs are deliberately not matched here: they
// exist to catch a file run straight from a clone rather than fetched.
func TestClientFilesAreTemplated(t *testing.T) {
	h, _ := newTestServer(t)
	for _, name := range []string{"install.sh", "register.sh", "beacon.service", "beacon.timer"} {
		t.Run(name, func(t *testing.T) {
			w := do(t, h, "GET", "/client/"+name, "")
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", w.Code)
			}
			if strings.Contains(w.Body.String(), "@@BASE_URL@@") {
				t.Errorf("%s still contains @@BASE_URL@@", name)
			}
		})
	}

	if !strings.Contains(do(t, h, "GET", "/client/install.sh", "").Body.String(), testBaseURL) {
		t.Error("install.sh does not point back at this server")
	}
	if !strings.Contains(do(t, h, "GET", "/client/beacon.service", "").Body.String(),
		"Environment=REGISTRY_URL="+testBaseURL) {
		t.Error("beacon.service does not carry this server's address")
	}
}

// Only the embedded client/ directory is reachable under /client/.
func TestClientServesNothingElse(t *testing.T) {
	h, _ := newTestServer(t)
	for _, p := range []string{"/client/nope.sh", "/client/../go.mod", "/client/%2e%2e", "/client/", "/client"} {
		if got := do(t, h, "GET", p, "").Code; got == http.StatusOK {
			t.Errorf("GET %s returned 200, want no content", p)
		}
	}
}

func TestRegistryPersistsAcrossReload(t *testing.T) {
	reg := newTestRegistry(t)
	if err := reg.upsert("alpha", "10.0.0.1"); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	reloaded, err := loadRegistry(reg.path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	got := reloaded.all()
	if len(got) != 1 || got[0].Host != "alpha" || got[0].IP != "10.0.0.1" {
		t.Fatalf("after reload got %+v, want one alpha/10.0.0.1 entry", got)
	}
	if got[0].UpdatedAt.IsZero() {
		t.Error("UpdatedAt was not recorded")
	}
}

func TestLoadRegistryRejectsCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadRegistry(path); err == nil {
		t.Error("loadRegistry succeeded on a corrupt file; want an error rather than silently starting empty")
	}
}

// The point of the temp-file-and-rename in replaceFile is that what is on disk
// is only ever a complete document, never a partial write or a stray temp file.
func TestSaveLeavesNoTempFilesBehind(t *testing.T) {
	reg := newTestRegistry(t)
	reg.upsert("alpha", "10.0.0.1")
	reg.upsert("beta", "10.0.0.2")

	entries, err := os.ReadDir(filepath.Dir(reg.path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != filepath.Base(reg.path) {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("data dir contains %v, want only %s", names, filepath.Base(reg.path))
	}
}

// upsert and all are called concurrently by the handlers, so the locking has to
// hold up under -race.
func TestConcurrentAccessIsSerialised(t *testing.T) {
	reg := newTestRegistry(t)

	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := reg.upsert(fmt.Sprintf("host%d", i), "10.0.0.1"); err != nil {
				t.Errorf("upsert: %v", err)
			}
			reg.all()
		}()
	}
	wg.Wait()

	if n := len(reg.all()); n != 8 {
		t.Errorf("got %d entries, want 8", n)
	}
}
