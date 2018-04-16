package client

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	connTimeout = 30
	action      = "events"
)

type EventService struct {
	serviceURL string
	client     *http.Client
}

func NewEventService(serviceURL string) (error, *EventService) {
	es := &EventService{}
	es.client = &http.Client{
		Timeout: time.Duration(connTimeout) * time.Second,
	}
	if _, err := url.Parse(serviceURL); err != nil {
		return fmt.Errorf("Invalid service URL: %s", serviceURL), nil
	}
	es.serviceURL = serviceURL
	if !strings.HasSuffix(serviceURL, "/") {
		es.serviceURL += "/"
	}
	return nil, es
}

func (es EventService) StoreEvent(event string) error {
	// This call should return HTTP 201 if successful
	resp, err := es.client.Post(es.serviceURL+action, "application/json",
		bytes.NewReader([]byte(event)))
	if err != nil {
		return err
	} else if resp.StatusCode >= 300 {
		return fmt.Errorf("Store failed with status code %d (%s)",
			resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	return nil
}

// Add other "REST" services ...
