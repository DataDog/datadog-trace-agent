package main

import (
	"errors"
	"net"
	"sync/atomic"
	"time"

	log "github.com/cihub/seelog"
)

// RateLimitedListener wraps a regular TCPListener with rate limiting.
type RateLimitedListener struct {
	connLease int32 // How many connections are available for this listener before rate-limiting kicks in
	*net.TCPListener
}

// NewRateLimitedListener returns a new wrapped listener, which is non-initialized
func NewRateLimitedListener(l net.Listener, conns int) (*RateLimitedListener, error) {
	tcpL, ok := l.(*net.TCPListener)

	if !ok {
		return nil, errors.New("cannot wrap listener")
	}

	sl := &RateLimitedListener{connLease: int32(conns), TCPListener: tcpL}

	return sl, nil
}

// Refresh periodically refreshes the connection lease, and thus cancels any rate limits in place
func (sl *RateLimitedListener) Refresh(conns int) {
	for range time.Tick(30 * time.Second) {
		atomic.StoreInt32(&sl.connLease, int32(conns))
		log.Debugf("Refreshed the connection lease: %d conns available", conns)
	}
}

// RateLimitedError  indicates a user request being blocked by our rate limit
// It satisfies the net.Error interface
type RateLimitedError struct{}

// Error returns an error string
func (e *RateLimitedError) Error() string { return "request has been rate-limited" }

// Temporary tells the HTTP server loop that this error is temporary and recoverable
func (e *RateLimitedError) Temporary() bool { return true }

// Timeout tells the HTTP server loop that this error is not a timeout
func (e *RateLimitedError) Timeout() bool { return false }

// Accept reimplements the regular Accept but adds rate limiting.
func (sl *RateLimitedListener) Accept() (net.Conn, error) {
	if atomic.LoadInt32(&sl.connLease) <= 0 {
		// we've reached our cap for this lease period, reject the request
		return nil, &RateLimitedError{}
	}

	for {
		//Wait up to 1 second for Reads and Writes to the new connection
		sl.SetDeadline(time.Now().Add(time.Second))

		newConn, err := sl.TCPListener.Accept()

		if err != nil {
			netErr, ok := err.(net.Error)

			//If this is a timeout, then continue to wait for
			//new connections
			if ok && netErr.Timeout() && netErr.Temporary() {
				continue
			}
		}

		// decrement available conns
		atomic.AddInt32(&sl.connLease, -1)

		return newConn, err
	}
}
