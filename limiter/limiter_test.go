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
	go func() {
		p.ServeTokens(ctx)
	}()

	// The following scenario should yield one success and
	// two failures, as there is only one token available in
	// the first second.  Note timing variations make these
	// tests potentially shaky, and they probably could be
	// hardened a little by using mechanisms such as channels
	// to communicate precise timings.  In the interest of time,
	// it is how it is for now, but the results are repeatable.
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			res, err := p.AcquireToken(ctx, 10*time.Millisecond)
			if err != nil || !res {
				atomic.AddInt64(&fail, 1)
			} else {
				atomic.AddInt64(&succ, 1)
			}
		}()
	}
	wg.Wait()
	if succ != 1 && fail != 2 {
		t.Fatalf("unexpected counts: succ: %d, fail:%d\n", succ, fail)
	}

	succ = 0
	fail = 0

	time.Sleep(1 * time.Second)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			res, err := p.AcquireToken(ctx, 10*time.Millisecond)
			if err != nil || !res {
				atomic.AddInt64(&fail, 1)
			} else {
				atomic.AddInt64(&succ, 1)
			}
		}()
	}
	wg.Wait()
	if succ != 2 && fail != 1 {
		t.Fatalf("unexpected counts: succ: %d, fail:%d\n", succ, fail)
	}

	cancel()
}

func TestTryAcquireToken(t *testing.T) {
	var succ, fail int64
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p, err := NewPulseLimiter(2, Sec, 1)
	if err != nil {
		t.Fatalf("Pulser creation failed: %v", err)
	}
	time.Sleep(1 * time.Second)
	var wg sync.WaitGroup
	go func() {
		p.ServeTokens(ctx)
	}()

	// The following scenario should yield one success and
	// two failures, as there is only one token available in
	// the first second.  Note timing variations make these
	// tests potentially shaky, so I'm still considering whether
	// there's something more foolproof.
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			res, err := p.TryAcquireToken(ctx)
			if err != nil || !res {
				atomic.AddInt64(&fail, 1)
			} else {
				atomic.AddInt64(&succ, 1)
			}
		}()
	}
	wg.Wait()
	if succ != 1 && fail != 2 {
		t.Fatalf("unexpected counts: succ: %d, fail:%d\n", succ, fail)
	}

	succ = 0
	fail = 0

	time.Sleep(1 * time.Second)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			res, err := p.TryAcquireToken(ctx)
			if err != nil || !res {
				atomic.AddInt64(&fail, 1)
			} else {
				atomic.AddInt64(&succ, 1)
			}
		}()
	}
	wg.Wait()
	if succ != 1 && fail != 2 {
		t.Fatalf("unexpected counts: succ: %d, fail:%d\n", succ, fail)
	}

	cancel()
}

func TestShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	finish := make(chan struct{})
	p, err := NewPulseLimiter(10, Sec, 1)
	if err != nil {
		t.Fatalf("Pulse Limiter creation failed: %v", err)
	}
	time.Sleep(500 * time.Millisecond)
	go func() {
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
}
