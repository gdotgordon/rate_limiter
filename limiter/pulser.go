package limiter

import (
	"context"
	"fmt"
	"log"
	"time"
)

// The Pulser is the item implmentating the Limiter interface.  It
// keeps track of the number of tokens currently available, as well
// as the refresh interval, which is the reciprocal of the rate.
//
// It strives for best accuracy using the Token Bucket algorithm.
// It implements the algorithm fairly literally, in that it
// dispenses new tokens at a uniform rate, based on the configured
// settings.  Also, as per the algorithm, it doesn't issue any new tokens
// if the "bucket" is at capacity.

// The capcity of the bucket is the "burst rate", that is, it's backlog
// of unused tokens represents the number of requests that could be handled
// at peak load.  One difference from the formal algorithm is that we assume
// each item is 1 unit of work, whereas the real algorithm assumes the units
// are bytes, and weights the actual size of the requests, which we ignore.
// If the burst rate is set to 1, this should cap the rate.

// All of this works well in Go, as the semantics of a buffered channel
// fit this abstraction very well.  Note, we don't need to explicitly
// store the current token count, as it is represented as the length of
// the buffered channel.
type Pulser struct {
	interval time.Duration
	source   chan (struct{})
}

// Ensure all interface methods are present.
var (
	_ Limiter = (*Pulser)(nil)
)

// NewPulser creates a new timer-based Limiter.  The input
// parameters are the number of items per interval, and the
// interval type, which is one of the enumerated IntervalType,
// and finally the burst rate, which is the total capacity of
// the bucket.  The burst rate essentially says how many tokens
// will be on hand when the system is quiescent.
func NewPulser(items int, interval IntervalType,
	burst int) (*Pulser, error) {
	if items <= 0 {
		return nil, fmt.Errorf("'items' must be positive")
	}
	if burst <= 0 {
		return nil, fmt.Errorf("'burst' must be positive")
	}

	dur := intervalTypeToDuration(interval)
	p := Pulser{}
	p.interval = time.Duration(dur.Nanoseconds() / int64(items))
	p.source = make(chan (struct{}), burst)
	return &p, nil
}

// HasTokenServer indicates that the Pulser does use a
// token server loop.
func (p Pulser) HasTokenServer() bool {
	return true
}

// ServeTokens is the timer-driven token creator.  It is a
// blocking call that would likely be invoked from a goroutine.
func (p Pulser) ServeTokens(ctx context.Context) {

	// We don't really need another channel variable, but making the
	// channel access unidirectional will allow the compiler
	// to help us if we misue it here.
	var sender chan<- (struct{}) = p.source

Loop:
	for {
		//fmt.Printf("looping %v, %v\n", p.interval, time.Now())

		// If we need to finish, clean up.  Otherwise, try to add
		// another token to the channel.  The token add may block,
		// which is fine, because this means we are in a quiescent
		// state and there's nothing to limit.
		//
		// Note if we happen to be in a quiescent state when the cancel
		// comes around, the ctx.Done() will get read.
		select {
		case <-ctx.Done():
			log.Printf("Limiter cleanup successful!\n")
			close(sender)
			break Loop
		case sender <- struct{}{}:
			fmt.Printf("appended to channel! %v\n", time.Now())
		}

		// Sleep to regulate the rate.
		time.Sleep(p.interval)
	}
}

// AcquireToken gets a bucket token to allow it to proceed,
// and optionally blocks until it acquires a token.  Passing a 0
// {or zero value) for the timeout means it will block forever.
func (p Pulser) AcquireToken(ctx context.Context,
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
		return false, fmt.Errorf("Context canceled!")
	case <-ctime:
		fmt.Printf("timer fired!\n")
		return false, nil
	case _, ok := <-p.source:
		if !ok {
			return false, fmt.Errorf("Channel closed!")
		}
		fmt.Printf("%v: got token...\n", time.Now())
		return true, nil
	}
}

// TryAcquireToken attempts to get a bucket token, and fails if one
// Is not immediately available.  It returns a boolean indicating whether
// it was able to acquire the token.
func (p Pulser) TryAcquireToken(ctx context.Context) (bool, error) {
	select {
	case <-ctx.Done():
		return false, fmt.Errorf("Context canceled!")
	case _, ok := <-p.source:
		if !ok {
			return false, fmt.Errorf("Channel closed!")
		} else {
			return true, nil
		}
	default:
		return false, nil
	}
}
