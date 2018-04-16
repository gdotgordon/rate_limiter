// Test driver for the server-side.  It creates a backend server
// that the limiter server should proxy for, plus it creates a
// Limiter object to be used by the server.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"strings"
	"sync"
	"time"

	"github.com/gdotgordon/rate_limiter/limiter"
	"github.com/gdotgordon/rate_limiter/server"
)

var (
	timeout = flag.Duration("timeout", 500*time.Millisecond,
		"How long clients shoid block if limited by the rate limiter")
	ops      = flag.Int("ops", 600, "how many ops per specifed interval")
	interval = flag.Int("interval", int(limiter.Min), "Operations per time")
	burst    = flag.Int("burst", 1, "Burst rate for limiter")
)

func main() {
	flag.Parse()
	p, err := limiter.NewPulser(*ops, limiter.IntervalType(*interval),
		*burst)
	if err != nil {
		log.Fatal("Pulser creation failed: %v", err)
	}

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

	/*
		resp, err := http.Post(ts.URL+"/events", "application/json",
			bytes.NewBufferString("{\"foo\" : \"bar\"}"))
		if err != nil {
			log.Fatal(err)
		}
		b, _ := httputil.DumpResponse(resp, true)
		fmt.Println(string(b))
	*/

	server := server.NewLimiterServer(p, *timeout, ts.URL)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		server.Start(context.Background())
	}()
	for i := 0; i < 5; i++ {
		go func() {
			resp, err := http.Post("http://localhost:8080/events", "application/json",
				bytes.NewBufferString("{\"foo\" : \"bar\"}"))
			if err != nil {
				log.Fatal(err)
			}
			b, _ := httputil.DumpResponse(resp, true)
			fmt.Println(string(b))
		}()
	}
	wg.Wait()
}
