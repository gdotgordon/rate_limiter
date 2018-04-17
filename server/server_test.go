package server

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gdotgordon/rate_limiter/limiter"
)

type placeHolder struct {
	v *int64
}

func (p placeHolder) eventHandler(w http.ResponseWriter,
	r *http.Request) {
	atomic.AddInt64(p.v, 1)
}

//http.ResponseWriter interface
func (p placeHolder) Header() http.Header {
	return make(map[string][]string)
}
func (p placeHolder) Write([]byte) (int, error) {
	return 0, nil
}
func (p placeHolder) WriteHeader(statusCode int) {
}

func TestEnforceLimits(t *testing.T) {
	p, err := limiter.NewPulser(250, limiter.Min, 1)
	if err != nil {
		t.Fatalf("Pulser creation failed: %v\n", err)
	}
	server := NewLimiterServer(p, 500*time.Millisecond, "http://dummy")
	var x int64
	ph := placeHolder{&x}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg, wg2 sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		p.ServeTokens(ctx)
	}()

	for i := 0; i < 5; i++ {
		wg2.Add(1)
		go func() {
			defer wg2.Done()

			ha := &http.Request{}
			server.enforceLimits(context.Background(),
				http.HandlerFunc(ph.eventHandler)).ServeHTTP(ph, ha)
		}()
	}
	wg2.Wait()
	cancel()
	wg.Wait()
	if *ph.v != 3 {
		t.Fatalf("Expected count = 3, got %d", *ph.v)
	}
}
