// Command beacon is a host registry: hosts POST their local IP to it, and other
// hosts on the same network look each other up through it instead of via DNS.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	// The server has to write its own public address into the client files it
	// serves, and it cannot reliably infer one from behind a reverse proxy.
	baseURL := strings.TrimSuffix(os.Getenv("BASE_URL"), "/")
	if baseURL == "" {
		return errors.New("BASE_URL is required, e.g. BASE_URL=https://beacon.example.com")
	}
	dataPath := envOr("DATA_PATH", "data/registry.json")
	port := envOr("PORT", "8080")

	reg, err := loadRegistry(dataPath)
	if err != nil {
		return err
	}
	log.Printf("loaded %d entries from %s", len(reg.all()), dataPath)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: routes(reg, baseURL),
		// Bound how long a slow or idle client can hold a connection. The
		// registry is reachable from the whole network and does nothing that
		// justifies a long-lived request.
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Shut down on SIGINT/SIGTERM so an in-flight registration finishes writing
	// the registry file rather than being killed partway through.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errc := make(chan error, 1)
	go func() {
		log.Printf("listening on %s for %s", srv.Addr, baseURL)
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		log.Print("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
