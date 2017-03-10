package main

import (
	"errors"
	"net"
	"sync/atomic"
	"time"

	log "github.com/cihub/seelog"
)

// StoppableListener wraps a regular TCPListener with an exit channel so we can exit cleanly from the Serve() loop of our HTTP server
type StoppableListener struct {
	exit      chan struct{}
	connLease int32 // How many connections are available for this listener before rate-limiting kicks in
	*net.TCPListener
}

// NewStoppableListener returns a new wrapped listener, which is non-initialized
func NewStoppableListener(l net.Listener, exit chan struct{}, conns int) (*StoppableListener, error) {
	tcpL, ok := l.(*net.TCPListener)

	if !ok {
		return nil, errors.New("cannot wrap listener")
	}

	sl := &StoppableListener{exit: exit, connLease: int32(conns), TCPListener: tcpL}

	return sl, nil
}

// Refresh periodically refreshes the connection lease, and thus cancels any rate limits in place
func (sl *StoppableListener) Refresh(conns int) {
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

// Accept reimplements the regular Accept but adds a check on the exit channel and returns if needed
func (sl *StoppableListener) Accept() (net.Conn, error) {
	if atomic.LoadInt32(&sl.connLease) <= 0 {
		// we've reached our cap for this lease period, reject the request
		return nil, &RateLimitedError{}
	}

	for {
		//Wait up to 1 second for Reads and Writes to the new connection
		sl.SetDeadline(time.Now().Add(time.Second))

		newConn, err := sl.TCPListener.Accept()

		//Check for the channel being closed
		select {
		case <-sl.exit:
			log.Debug("stopping listener")
			sl.TCPListener.Close()
			return nil, errors.New("listener stopped")
		default:
			//If the channel is still open, continue as normal
		}

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
