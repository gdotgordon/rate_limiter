// Package restclient implements (part of) a RESTful service to
// store JSON-encoded events.  The service invokes the storage
// service through the rate limiter proxy.
package restclient

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

const (
	connTimeout = 30
	resource    = "events"
)

// The EventService is used to invoke the REST APIs.
type EventService struct {
	serviceURL string
	client     *http.Client
}

// NewEventService creates a new REST service using the specified
// endpoint.  The resource type will be appended to the URL by
// the invoker for the REST call.
func NewEventService(serviceURL string) (*EventService, error) {
	es := &EventService{}
	es.client = &http.Client{
		Timeout: time.Duration(connTimeout) * time.Second,
	}
	if _, err := url.Parse(serviceURL); err != nil {
		return nil, fmt.Errorf("Invalid service URL: %s", serviceURL)
	}
	es.serviceURL = serviceURL
	if !strings.HasSuffix(serviceURL, "/") {
		es.serviceURL += "/"
	}
	return es, nil
}

// StoreEvent stores a string containing a JSON-encoded event.
// Returns an error if there was a server error (other than the
// server being too busy), and true or false as to whether the
// store was successful.  This boolean will give the app the
// option of retrying if timeout occurred.
func (es EventService) StoreEvent(event string) (bool, error) {

	// This call should return HTTP 201 if successful.
	resp, err := es.client.Post(es.serviceURL+resource, "application/json",
		bytes.NewReader([]byte(event)))
	if err == nil {
		b, _ := httputil.DumpResponse(resp, true)
		log.Println(string(b))
	}

	if err != nil {
		return false, err
	} else if resp.StatusCode == 503 {
		return false, nil
	} else if resp.StatusCode >= 300 {
		return false, fmt.Errorf("Store failed with status code %d (%s)",
			resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	return true, nil
}

// Add other "REST" services ...
