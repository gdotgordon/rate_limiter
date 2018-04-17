# rate_limiter
Rate limiter demo using the Token Bucket algorithm.

The task is to stand up a service that applies a rate limiting algorithm on incoming requests, forwarding requests that are not rate-limited to a destination server.  The proxied service is a REST-like service that manages records for some type of events that occur.

The solution provided here implements the Token Bucket algorithm, as per https://en.wikipedia.org/wiki/Token_bucket.

## Road Map

The source code contains all the client and server packages, plus sample client and server binaries that run and demonstrate the behavior.  There are a few log statements, such as producing a token, that should be removed if the service is run at high capacity.  They are left in for now so we can verify the behavior.

The source directories are laid out as follows:
* limiter: contains the `Limiter` interface and the first implemented interface `PulseLimiter`.
* server: contains the `LimiterServer` that use the rate limiter and forwards requests to the storage service.
* restclient: contains the client API, in particular the "StoreEvent()" call.
* examples/server: doing a *go install* on this builds a binary that can re run for the server.  It uses a built-in dummy test-server for the backend proxied service.  It is a blocking service, so run it in the background, or a separate window. 
* examples/client: doing a *go install* on this builds a running client that sends multiple concurrent requests in a loop with random sleeps in between.

Note the server and rate limiter are fully configurable, and there are flags defined in the client and server binaries that will configure these settings as requested.  The defaults are fully operational.

I found that running the server as `bin/server -ops=60 -timeout=4s &` alongside with the default client yields about a 10-15% rate of requests getting limited.

## Implementation Notes

### Limiter
Taken from the code comments, here is a description of the implementation:

The PulseLimiter implements the Limiter interface.  It keeps track of the number of tokens currently available, as well as the refresh interval, which is the reciprocal of the rate.

It strives for best accuracy using the Token Bucket algorithm and implementing it fairly literally, in that it dispenses new tokens at a uniform rate, based on the configured settings.  Also, as per the algorithm, it doesn't issue any new tokens and the goroutine sleeps whenever the "bucket" is at capacity.  It uses a loop that runs in it's own goroutine.

The capacity of the bucket is the "burst rate", that is, it's backlog of unused tokens represents the number of requests that could be handled at peak load.  One difference from the formal algorithm is that we assume each item is 1 unit of work, whereas the real algorithm assumes the units are bytes, and weights the actual size of the requests, which we ignore. If the burst rate is set to 1, this should cap the rate.

All of this works well in Go, as the semantics of a buffered channel fit this abstraction very well.  Note, we don't need to explicitly store the current token count as the blocking nature of the channel limits the tokens appropriately.

Note if I had more time, I'd implement another version of the limiter that timestamps the previous and current event and extrapolates an approximate token value from that for comparison of performance.  The algorithms that don't use a generator loop do some sort of interpolation and can suffer from a degree of inaccuracy due to not handling "burstiness" well if not written properly.

### Server
The server forwards requests that are accepted by the rate limiter to the storage service.  The strategy we've chosen for the rate limiter is to use a server-configurable timeout, which will reject a particular request if it waits too long, due to the server load being too high.  This allows us to configure a balance between reliable service and acceptable load, which in practice could be performance-tuned at runtime.
