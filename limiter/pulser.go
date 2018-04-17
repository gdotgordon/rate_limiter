package limiter

import (
	"context"
	"fmt"
	"log"
	"time"
)

// PulseLimiter implements the Limiter interface.  It keeps track
// of the number of tokens currently available, as well as the refresh
// interval, which is the reciprocal of the rate.
//
// It strives for best accuracy using the Token Bucket algorithm and
// implementing it fairly literally, in that it dispenses new tokens
// at a uniform rate, based on the configured settings.  Also, as per
// the algorithm, it doesn't issue any new tokens and the goroutine
// sleeps whenever the "bucket" is at capacity.

// The capcity of the bucket is the "burst rate", that is, it's backlog
// of unused tokens represents the number of requests that could be handled
// at peak load.  One difference from the formal algorithm is that we assume
// each item is 1 unit of work, whereas the real algorithm assumes the units
// are bytes, and weights the actual size of the requests, which we ignore.
// If the burst rate is set to 1, this should cap the rate.

// PulseLimiter works well in Go, as the semantics of a buffered channel
// fit this abstraction very well.  Note, we don't need to explicitly
// store the current token count, as the size and blocking nature of the
// channel limits the tokens appropriately.
type PulseLimiter struct {
	interval time.Duration
	source   chan (struct{})
}

// Ensure all interface methods are present.
var (
	_ Limiter = (*PulseLimiter)(nil)
)

// NewPulseLimiter creates a new timer-based Limiter.  The input
// parameters are the number of items per interval, and the
// interval type, which is one of the enumerated IntervalType,
// and finally the burst rate, which is the total capacity of
// the bucket.  The burst rate essentially says how many tokens
// will be on hand when the system is quiescent.
func NewPulseLimiter(items int, interval IntervalType,
	burst int) (*PulseLimiter, error) {
	if items <= 0 {
		return nil, fmt.Errorf("'items' must be positive")
	}
	if burst <= 0 {
		return nil, fmt.Errorf("'burst' must be positive")
	}

	dur := intervalTypeToDuration(interval)
	p := PulseLimiter{}
	p.interval = time.Duration(dur.Nanoseconds() / int64(items))
	p.source = make(chan (struct{}), burst)
	return &p, nil
}

// HasTokenServer indicates that the PulseLimiter does use a
// token server loop.
func (p PulseLimiter) HasTokenServer() bool {
	return true
}

// ServeTokens is the timer-driven token creator.  It is a
// blocking call that would likely be invoked from a goroutine.
func (p PulseLimiter) ServeTokens(ctx context.Context) {

	// We don't really need another channel variable, but making the
	// channel access unidirectional will allow the compiler
	// to help us if we misue it here.
	var sender chan<- (struct{}) = p.source

Loop:
	for {
		// If we need to finish, clean up.  Otherwise, try to add
		// another token to the channel.  The channel send (token add)
		// may block, which is fine, because this means we are in a
		// quiescent state and there's nothing to limit.
		//
		// Note if we happen to be in a quiescent state when the cancel
		// comes around, the ctx.Done() will get read in the select.
		select {
		case <-ctx.Done():
			log.Printf("Limiter cleanup successful!\n")
			close(sender)
			break Loop
		case sender <- struct{}{}:
		}

		// Sleep to regulate the rate.
		time.Sleep(p.interval)
	}
}

// AcquireToken attempts to acquire a token for the request within the
// specified timeout.  It returns a boolean specifying whether it
// successfully acquired the token.  Passing a 0 (or zero value) for
// the timeout means it will block "forever".
func (p PulseLimiter) AcquireToken(ctx context.Context,
	timeout time.Duration) (bool, error) {

	// If a timeout is not specified, we'll use a nil read channel,
	// which blocks forever.
	var ctime <-chan (time.Time)
	if timeout != 0 {
		t := time.NewTicker(timeout)
		defer t.Stop()

		ctime = t.C
	}

	select {
	case <-ctx.Done():
		return false, fmt.Errorf("context canceled")
	case <-ctime:
		return false, nil
	case _, ok := <-p.source:
		if !ok {
			return false, fmt.Errorf("channel closed")
		}
		return true, nil
	}
}

// TryAcquireToken attempts to get a bucket token, and fails if one
// Is not immediately available.  It returns a boolean indicating whether
// it was able to acquire the token.
func (p PulseLimiter) TryAcquireToken(ctx context.Context) (bool, error) {
	select {
	case <-ctx.Done():
		return false, fmt.Errorf("context canceled")
	case _, ok := <-p.source:
		if !ok {
			return false, fmt.Errorf("channel closed")
		}
		return true, nil
	default:
		return false, nil
	}
}
