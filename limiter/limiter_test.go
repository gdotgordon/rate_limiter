package limiter

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Test blocking token acqusition.
func TestAcquireToken(t *testing.T) {
	var succ, fail int64
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p, err := NewPulseLimiter(2, Sec, 1)
	if err != nil {
		t.Fatalf("Pulser creation failed: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		p.ServeTokens(ctx)
	}()

	<-p.tokens
	time.Sleep(1500 * time.Millisecond)
	// The following scenario should yield two successes and
	// one failures, as new tokens come twice per second.
	var wg2 sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg2.Add(1)
		go func() {
			defer wg2.Done()

			res, err := p.AcquireToken(ctx, 10*time.Millisecond)
			if err != nil || !res {
				atomic.AddInt64(&fail, 1)
			} else {
				atomic.AddInt64(&succ, 1)
			}
		}()
	}
	wg2.Wait()
	if succ != 2 || fail != 1 {
		t.Fatalf("unexpected counts: succ: %d, fail:%d\n", succ, fail)
	}

	succ = 0
	fail = 0
	// Given the token rate, only one should succeed as they all
	// start at virtually the same time.
	for i := 0; i < 3; i++ {
		wg2.Add(1)
		go func() {
			defer wg2.Done()

			res, err := p.AcquireToken(ctx, 750*time.Millisecond)
			if err != nil || !res {
				atomic.AddInt64(&fail, 1)
			} else {
				atomic.AddInt64(&succ, 1)
			}
		}()
	}
	wg2.Wait()
	if succ != 1 || fail != 2 {
		t.Fatalf("unexpected counts (second): succ: %d, fail:%d\n", succ, fail)
	}

	cancel()
	wg.Wait()
}

// Test non-blocking token acquisiton.
func TestTryAcquireToken(t *testing.T) {
	var succ, fail int64
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p, err := NewPulseLimiter(30, Min, 1)
	if err != nil {
		t.Fatalf("Pulser creation failed: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		p.ServeTokens(ctx)
	}()

	// After the fist token is read, the next one won't be available
	// for two seconds, so add some slop before trying.
	<-p.tokens
	time.Sleep(2500 * time.Millisecond)

	// Given the refresh interval, only one should succeed, as both
	// are trying at "virtually" the same time.
	var wg2 sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg2.Add(1)
		go func() {
			defer wg2.Done()
			res, err := p.TryAcquireToken(ctx)
			if err != nil || !res {
				atomic.AddInt64(&fail, 1)
			} else {
				atomic.AddInt64(&succ, 1)
			}
		}()
	}
	wg2.Wait()
	if succ != 1 || fail != 1 {
		t.Fatalf("unexpected counts: succ: %d, fail:%d\n", succ, fail)
	}

	cancel()
	wg.Wait()
}

func TestShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	finish := make(chan struct{})
	p, err := NewPulseLimiter(10, Sec, 1)
	if err != nil {
		t.Fatalf("Pulse Limiter creation failed: %v", err)
	}
	time.Sleep(500 * time.Millisecond)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		p.ServeTokens(ctx)
		finish <- struct{}{}
	}()
	cancel()
	tick := time.NewTicker(5 * time.Second)
	select {
	case <-tick.C:
		t.Fatalf("Server did not close!")
	case <-finish:
	}
	wg.Wait()
}
