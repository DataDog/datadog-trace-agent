package main

import (
	"errors"
	"net"
	"time"

	log "github.com/cihub/seelog"
)

// StoppableListener wraps a regular TCPListener with an exit channel so we can exit cleanly from the Serve() loop of our HTTP server
type StoppableListener struct {
	exit chan struct{}
	// How many connections are available for this listener before rate-limiting kicks in
	connLease int
	*net.TCPListener
}

// NewStoppableListener returns a new wrapped listener, which is non-initialized
func NewStoppableListener(l net.Listener, exit chan struct{}, conns int) (*StoppableListener, error) {
	tcpL, ok := l.(*net.TCPListener)

	if !ok {
		return nil, errors.New("cannot wrap listener")
	}

	sl := &StoppableListener{exit: exit, connLease: conns, TCPListener: tcpL}

	return sl, nil
}

func (sl *StoppableListener) Meter(conns int) {
	for range time.Tick(10 * time.Second) {
		sl.connLease = conns
		log.Infof("Refreshed the connLease %d", sl.connLease)
	}
}

// Accept reimplements the regular Accept but adds a check on the exit channel and returns if needed
func (sl *StoppableListener) Accept() (net.Conn, error) {
	if sl.connLease <= 0 {
		log.Infof("This is the connLease %d : rate-limiting is in effect", sl.connLease)
		return nil, errors.New("receiver has been rate-limited, new conns will be rejected")
	}

	for {
		//Wait up to one second for a new connection
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

		// TODO[aaditya]: is this safe for concurrent access?
		sl.connLease--

		return newConn, err
	}
}
