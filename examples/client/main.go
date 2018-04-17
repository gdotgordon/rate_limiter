// Test client.  Launches a configurable number of goroutines that
// invoke the store API and sleep a random interval.
package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gdotgordon/rate_limiter/restclient"
)

var (
	concurrency = flag.Int("concurrency", 5, "number of goroutines")
	port        = flag.Int("port", 8080, "port limiter server is listening on")
)

func main() {
	flag.Parse()
	cli, err := restclient.NewEventService("http://localhost:" +
		strconv.Itoa(*port))
	if err != nil {
		log.Fatalf("Creating rest client, error: %v\n", err)
	}

	fmt.Printf("Type interrupt (Ctrl-C) to terminate.")
	var succ, fail, errors int64

	done := false
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, syscall.SIGINT, syscall.SIGTERM)
		<-sigint
		fmt.Printf("Interrupted, draining loop...\n")
		done = true
	}()

	rand.Seed(time.Now().Unix())
	n := *concurrency
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				res, err := cli.StoreEvent(
					"{\"type\" : \"earthquake\", \"magnitude\" : 5}")
				if err != nil {
					atomic.AddInt64(&errors, 1)
				} else if res {
					atomic.AddInt64(&succ, 1)
				} else {
					atomic.AddInt64(&fail, 1)
				}
				if done {
					break
				}
				time.Sleep(
					time.Duration(int64(rand.Intn(2000)) * int64(time.Millisecond)))
			}
		}()
	}
	wg.Wait()

	fmt.Printf("%d successs, %d failures (timeout), %d errors\n",
		succ, fail, errors)
}
