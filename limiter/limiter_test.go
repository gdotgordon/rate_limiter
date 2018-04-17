package limiter

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAcquireToken(t *testing.T) {
	var succ, fail int64
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p, err := NewPulseLimiter(2, Sec)
	if err != nil {
		t.Fatalf("Pulser creation failed: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		p.ServeTokens(ctx)
	}()

	// The sleep ensures the server is ready.
	time.Sleep(1 * time.Second)

	// The following scenario should yield one success and
	// two failures, as there is only one token available in
	// the first second.  Note timing variations could affect
	// these tests, but I've tried to pick timing intervals
	// that lead to repeatability despite this.
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
	if succ != 1 || fail != 2 {
		t.Fatalf("unexpected counts: succ: %d, fail:%d\n", succ, fail)
	}

	// One too many for the interval (.5 seconds), so one should fail.
	succ = 0
	fail = 0
	for i := 0; i < 5; i++ {
		wg2.Add(1)
		go func() {
			defer wg2.Done()

			// Slop the timeout to avoid a very close shave.
			res, err := p.AcquireToken(ctx, 2200*time.Millisecond)
			if err != nil || !res {
				atomic.AddInt64(&fail, 1)
			} else {
				atomic.AddInt64(&succ, 1)
			}
		}()
	}
	wg2.Wait()
	if succ != 4 || fail != 1 {
		t.Fatalf("unexpected counts (second): succ: %d, fail:%d\n", succ, fail)
	}

	cancel()
	wg.Wait()
}

func TestTryAcquireToken(t *testing.T) {
	var succ, fail int64
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p, err := NewPulseLimiter(1, Sec)
	if err != nil {
		t.Fatalf("Pulser creation failed: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		p.ServeTokens(ctx)
	}()

	// The sleep ensures the server is ready.
	time.Sleep(1 * time.Second)

	// Only one token is available to start.
	var wg2 sync.WaitGroup
	for i := 0; i < 3; i++ {
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
	if succ != 1 || fail != 2 {
		t.Fatalf("unexpected counts: succ: %d, fail:%d\n", succ, fail)
	}

	// Slop it a little for safety.
	time.Sleep(1100 * time.Millisecond)
	// Once again we expect the same result
	succ = 0
	fail = 0
	for i := 0; i < 3; i++ {
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
	if succ != 1 || fail != 2 {
		t.Fatalf("unexpected counts (second): succ: %d, fail:%d\n", succ, fail)
	}

	cancel()
	wg.Wait()
}

func TestShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	finish := make(chan struct{})
	p, err := NewPulseLimiter(10, Sec)
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
