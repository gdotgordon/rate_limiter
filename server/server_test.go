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

// This test ensures that the Limiter is properly wired
// into the server.
func TestEnforceLimits(t *testing.T) {
	p, err := limiter.NewPulseLimiter(250, limiter.Min)
	if err != nil {
		t.Fatalf("Pulser creation failed: %v\n", err)
	}
	server := NewLimiterServer(8080, p, 500*time.Millisecond, "http://dummy")
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

	// Given the rate for the limiter and the timeout of .5 sec,
	// we expect 3 of 5 to work.
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
