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

func (sl *StoppableListener) Refresh(conns int) {
	for range time.Tick(30 * time.Second) {
		atomic.StoreInt32(&sl.connLease, int32(conns))
		log.Debugf("Refreshed the connection lease: %d conns available", sl.connLease)
	}
}

type RateLimitedError struct{}

// satisfy net.Error interface
func (e *RateLimitedError) Error() string   { return "request has been rate-limited" }
func (e *RateLimitedError) Temporary() bool { return true }
func (e *RateLimitedError) Timeout() bool   { return false }

// Accept reimplements the regular Accept but adds a check on the exit channel and returns if needed
func (sl *StoppableListener) Accept() (net.Conn, error) {
	if sl.connLease <= 0 {
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
		// TODO[aaditya]: this is updated concurrently
		// but probably safe enough? we don't care about a 100% accurate value
		atomic.AddInt32(&sl.connLease, -1)

		return newConn, err
	}
}
