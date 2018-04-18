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

	p, err := NewPulseLimiter(2, Sec, 1)
	if err != nil {
		t.Fatalf("Pulser creation failed: %v", err)
	}
	time.Sleep(1 * time.Second)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		p.ServeTokens(ctx)
	}()

	// The following scenario should yield one success and
	// two failures, as there is only one token available in
	// the first second.  Note timing variations make these
	// tests potentially shaky, and they probably could be
	// hardened a little by using mechanisms such as channels
	// to communicate precise timings.  In the interest of time,
	// it is how it is for now, but the results are repeatable.
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
	if succ != 1 && fail != 2 {
		t.Fatalf("unexpected counts: succ: %d, fail:%d\n", succ, fail)
	}

	succ = 0
	fail = 0
	time.Sleep(1 * time.Second)
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
	if succ != 2 && fail != 1 {
		t.Fatalf("unexpected counts (second): succ: %d, fail:%d\n", succ, fail)
	}

	cancel()
	wg.Wait()
}

func TestTryAcquireToken(t *testing.T) {
	var succ, fail int64
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p, err := NewPulseLimiter(1, Sec, 1)
	if err != nil {
		t.Fatalf("Pulser creation failed: %v", err)
	}

	// Should be one token available after a second.
	time.Sleep(1 * time.Second)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		p.ServeTokens(ctx)
	}()

	// There is one token initially available, and given the refresh
	// interval, only one should succeed.
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
	if succ != 1 && fail != 1 {
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
