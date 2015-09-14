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
	*net.TCPListener
}

// NewStoppableListener returns a new wrapped listener, which is non-initialized
func NewStoppableListener(l net.Listener, exit chan struct{}) (*StoppableListener, error) {
	tcpL, ok := l.(*net.TCPListener)

	if !ok {
		return nil, errors.New("Cannot wrap listener")
	}

	sl := &StoppableListener{exit: exit, TCPListener: tcpL}

	return sl, nil
}

// Accept reimplements the regular Accept but adds a check on the exit channel and returns if needed
func (sl *StoppableListener) Accept() (net.Conn, error) {
	for {
		//Wait up to one second for a new connection
		sl.SetDeadline(time.Now().Add(time.Second))

		newConn, err := sl.TCPListener.Accept()

		//Check for the channel being closed
		select {
		case <-sl.exit:
			log.Info("Stopping Listener")
			return nil, errors.New("Listener stopped")
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

		return newConn, err
	}
}
