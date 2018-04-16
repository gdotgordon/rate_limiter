package server

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gdotgordon/rate_limiter/limiter"
)

const connTimeout = 30

type LimiterServer struct {
	timeout        time.Duration
	proxiedURL     string
	proxiedService *http.Client
	limiter        limiter.Limiter
}

func NewLimiterServer(limiter limiter.Limiter, timeout time.Duration,
	proxiedURL string) *LimiterServer {
	ls := &LimiterServer{timeout: timeout, proxiedURL: proxiedURL}
	ls.limiter = limiter
	ls.proxiedService = &http.Client{
		Timeout: time.Duration(connTimeout) * time.Second,
	}
	return ls
}

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
		Addr: ":8080",
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

	// This function will be invoked becuase we are using DefaultServeMux.
	// It turns out to be a great way to have access to the Context object
	// via the closure.
	http.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
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

		if r.Body == nil {
			http.Error(w, "Empty body", http.StatusBadRequest)
			return
		}

		resp, err := ls.proxiedService.Post(ls.proxiedURL+"/events", "application/json",
			r.Body)
		if err != nil {
			http.Error(w, "Service error", http.StatusInternalServerError)
			return
		} else {
			w.WriteHeader(resp.StatusCode)
			return
		}
	})

	log.Printf("Limiter server accepting requests ...\n")
	log.Println(s.ListenAndServe())
	wg.Wait()
	return err
}
