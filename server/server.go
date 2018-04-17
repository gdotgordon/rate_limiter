// Package server implements the limiter server.  It's main
// type is the LimiterServer, whose sole purpose is to rate-limit
// incoming requests and reject those that exceed the rate quota.
// The server is built in such a way that the rate enforcement is a
// composable filter that wraps the actual storage service.  The pattern
// uses a chain of delegation approach such that other functionality, such
// that additional services, such as logging, could be inserted into
// the chain.
//
// The strategy we've chosen for the rate limiter is to use a
// server-configurable timeout, which will reject a particular request
// if it waits too long, due to the server load being too high. This
// allows us to configure a balance between reliable service and acceptable
// load, which in practice could be performance-tuned at runtime.
package server

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/gdotgordon/rate_limiter/limiter"
)

const connTimeout = 30

// The LimiterServer is the type implementing the rate limiting service.
// As explained above, it could easily be extended to cover other functions
// besides rate limiting with regard to the service it proxies.
type LimiterServer struct {
	port           int
	timeout        time.Duration
	proxiedURL     string
	proxiedService *http.Client
	limiter        limiter.Limiter
}

// NewLimiterServer creates a server that runs on the specified port,
// and applies the provided Limiter to filter incoming requests.  The
// timeout refers to the client timeout in trying to get through the
// rate limiter.  The proxied URL is the URL of the backend storage
// service that requests are forwarded to.
func NewLimiterServer(port int, limiter limiter.Limiter,
	timeout time.Duration, proxiedURL string) *LimiterServer {
	ls := &LimiterServer{port: port, timeout: timeout, proxiedURL: proxiedURL}
	ls.limiter = limiter
	ls.proxiedService = &http.Client{
		Timeout: time.Duration(connTimeout) * time.Second,
	}
	return ls
}

// Start starts the token generator loop.  It is blocking, so
// it should be started in a goroutine.
func (ls *LimiterServer) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start producing tokens for the bucket.
	var err error
	var wg sync.WaitGroup
	if ls.limiter.HasTokenServer() {
		wg.Add(1)
		go func() {
			defer wg.Done()

			ls.limiter.ServeTokens(ctx)
		}()
	}

	// Setup the clean shutdown.
	wg.Add(1)
	s := http.Server{
		Addr: ":" + strconv.Itoa(ls.port),
	}
	go func() {
		defer wg.Done()

		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, syscall.SIGINT, syscall.SIGTERM)
		<-sigint

		// We received an interrupt signal, shut down.
		if err := s.Shutdown(ctx); err != nil {
			// Error from closing listeners, or context timeout:
			log.Printf("HTTP server Shutdown: %v", err)
		}

		// Invoking the cancel function should cleanup the limiter.
		cancel()
	}()

	// Encapsulate event storer inside limit checker.
	http.Handle("/events", ls.enforceLimits(ctx,
		http.HandlerFunc(ls.eventHandler)))

	log.Printf("Limiter server accepting requests on port %d ...\n", ls.port)
	log.Println(s.ListenAndServe())
	wg.Wait()
	return err
}

// enforceLimits is a "middleware" pattern that allows us to
// inject additional functionality (here, enforcing rate limiting)
// to the base functionality (posting an event).
func (ls *LimiterServer) enforceLimits(ctx context.Context,
	next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res, err := ls.limiter.AcquireToken(ctx, ls.timeout)
		if err != nil {
			http.Error(w, "Token error", http.StatusInternalServerError)
			return
		}
		if !res {
			// Could not acquire token in time.
			http.Error(w, "System too busy", http.StatusServiceUnavailable)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// eventHandler will be invoked to store the event if
func (ls *LimiterServer) eventHandler(w http.ResponseWriter,
	r *http.Request) {
	if r.Body == nil {
		http.Error(w, "Empty body", http.StatusBadRequest)
		return
	}

	// Invoke the proxied service and capture the result.
	resp, err := ls.proxiedService.Post(ls.proxiedURL+"/events",
		"application/json", r.Body)
	if err != nil {
		http.Error(w, "Service error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(resp.StatusCode)
	return
}
