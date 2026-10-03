package client

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestNewRejectsInsecureOrCredentialedEndpoints(t *testing.T) {
	for _, endpoint := range []string{"http://example.com/api", "https://user:pass@example.com/api"} {
		if _, err := New(endpoint, "ac_live_abcdefghijklmnopqrstuvwxyz"); err == nil {
			t.Fatalf("expected endpoint %q to be rejected", endpoint)
		}
	}
}

func TestIdempotencyKeyIsRandomAndLong(t *testing.T) {
	first, err := idempotencyKey()
	if err != nil {
		t.Fatal(err)
	}
	second, err := idempotencyKey()
	if err != nil {
		t.Fatal(err)
	}
	if len(first) < 16 || first == second {
		t.Fatal("idempotency key generation is unsafe")
	}
}

func TestEndpointPreservesEscapedResourceIdentifier(t *testing.T) {
	parsed, err := url.Parse("https://console.example/api/public/v1")
	if err != nil {
		t.Fatal(err)
	}
	client := &Client{baseURL: parsed}
	endpoint, err := client.endpoint("/instances/" + url.PathEscape("unexpected/path?admin=true"))
	if err != nil {
		t.Fatal(err)
	}
	expected := "https://console.example/api/public/v1/instances/unexpected%2Fpath%3Fadmin=true"
	if endpoint != expected {
		t.Fatalf("unexpected endpoint: got %q want %q", endpoint, expected)
	}
}

func TestRetryReusesMutationIdempotencyKey(t *testing.T) {
	parsed, err := url.Parse("https://console.example/api/public/v1")
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	attempt := 0
	client := &Client{
		baseURL: parsed,
		apiKey:  "ac_live_abcdefghijklmnopqrstuvwxyz",
		http: &http.Client{Timeout: time.Second, Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			attempt++
			keys = append(keys, request.Header.Get("Idempotency-Key"))
			if attempt == 1 {
				return &http.Response{
					StatusCode: http.StatusServiceUnavailable,
					Header: http.Header{
						"Content-Type": {"application/json"},
						"Retry-After":  {"0.001"},
					},
					Body: io.NopCloser(strings.NewReader(`{"error":{"message":"retry","retryable":true}}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": {"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"id":"one"}`)),
			}, nil
		})},
	}
	var result map[string]any
	if err := client.request(context.Background(), http.MethodPost, "/instances", map[string]string{"name": "one"}, &result); err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 || keys[0] == "" || keys[0] != keys[1] {
		t.Fatalf("mutation retry changed its idempotency key: %#v", keys)
	}
}
