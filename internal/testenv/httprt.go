package testenv

import "net/http"

// RoundTripFunc adapts a plain function into an http.RoundTripper. Use it
// in tests that need to capture requests and synthesize a response
// without a live server:
//
//	http.Client{Transport: testenv.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
//	    return &http.Response{...}, nil
//	})}
//
// The exported type means call sites can be grep'd for `testenv.RoundTripFunc`
// to find every test transport adapter in the repo.
type RoundTripFunc func(*http.Request) (*http.Response, error)

// RoundTrip implements http.RoundTripper.
func (f RoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
