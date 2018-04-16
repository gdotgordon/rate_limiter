// The limiter package has implmentations of the Limiter interface,
// which is a general-purpose token-based rate limiter.  The abstract
// model employed is that a token must successfully be acquired for
// some rate-limited code to proceed.
package limiter

import (
	"context"
	"time"
)

// Interval type constants as explained later.
const (
	Msec IntervalType = iota
	Sec
	Min
)

// Interval type are constants used when specifying a rate, as
// in X number of operations per <intervaql type>.
type IntervalType int

// The Limiter is the abstraction for a rate limiter implementation.
// This interface has methods that may or may not be used, depending
// on whether the limiter implementation has a "token server" loop.
// Some implementations may approximate number of tokens by examining
// intervals in between requests, and don't require a server loop.
// However, the algortihms that don't use a generator loop and thus do
// some sort of interpolation can suffer from a degree of inaccuracy
// due to not handling "burstiness" well.
type Limiter interface {
	AcquireToken(ctx context.Context, timeout time.Duration) (bool, error)
	TryAcquireToken(ctx context.Context) (bool, error)
	HasTokenServer() bool
	ServeTokens(ctx context.Context)
}

func intervalTypeToDuration(t IntervalType) time.Duration {
	var dur time.Duration
	switch t {
	case Msec:
		dur = 1 * time.Millisecond
	case Sec:
		dur = 1 * time.Second
	case Min:
		dur = 1 * time.Minute
	}
	return dur
}
