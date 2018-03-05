package main

import (
	"errors"
	"fmt"
	"net"
	"time"

	"golang.org/x/time/rate"
)

// timePeriod specifies the time period to which the rate limiting applies.
const timePeriod = 30 * time.Second

// rateLimitedListener is a listener that implements a rate limiter.
type rateLimitedListener struct {
	*rate.Limiter
	net.Listener
}

// newRateLimitedListener returns a net.Listener which listens on the given
// address and limits the rate of connections for a given period of time
// to conns.
func newRateLimitedListener(addr string, conns int) (net.Listener, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("cannot listen on %s: %v", addr, err)
	}
	return &rateLimitedListener{
		Listener: ln,
		Limiter:  rate.NewLimiter(rate.Limit(timePeriod), conns),
	}, nil
}

// ErrRateLimited is returned when a request can not be accepted due
// to the rate limit being reached.
var ErrRateLimited = errors.New("request has been rate limited")

// Accept wraps the standard net.Listener with rate limiting.
func (rl *rateLimitedListener) Accept() (net.Conn, error) {
	if !rl.Allow() {
		return nil, ErrRateLimited
	}
	return rl.Listener.Accept()
}
