// Test driver for the server-side.  It creates an in-proc
// backend server that the limiter server should proxy for,
// plus it creates a Limiter object to be used by the server.
// It then launches the server and waits for cancellation.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"

	"github.com/gdotgordon/rate_limiter/limiter"
	"github.com/gdotgordon/rate_limiter/server"
)

var (
	port    = flag.Int("port", 8080, "port limiter server is listening on")
	timeout = flag.Duration("timeout", 500*time.Millisecond,
		"How long clients shoid block if limited by the rate limiter")
	ops      = flag.Int("ops", 600, "how many ops per specifed interval")
	interval = flag.Int("interval", int(limiter.Min), "Operations per time")
)

func main() {
	flag.Parse()
	p, err := limiter.NewPulseLimiter(*ops, limiter.IntervalType(*interval))
	if err != nil {
		log.Fatal("Pulser creation failed: %v\n", err)
	}

	// Simple proxied server that the limiter server will talk to.
	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				http.Error(w, "Unsupported method", http.StatusNotImplemented)
				return
			}

			surl := r.URL.String()
			if !strings.HasSuffix(surl, "events") {
				http.Error(w, "Unknown URL", http.StatusBadRequest)
				return
			}

			ev := make(map[string]interface{})
			dec := json.NewDecoder(r.Body)
			err := dec.Decode(&ev)
			if err != nil {
				http.Error(w, "Invalid JSON in payload", http.StatusBadRequest)
				return
			}
			w.Header().Add("Location", surl+"/12345")
			w.WriteHeader(http.StatusCreated)
		}))
	defer ts.Close()

	server := server.NewLimiterServer(*port, p, *timeout, ts.URL)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		server.Start(context.Background())
	}()
	wg.Wait()
}
