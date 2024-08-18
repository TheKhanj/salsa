package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

type TCPListener struct {
	Address  string
	Backends []Backend
	Interval time.Duration

	shutdown            chan struct{}
	anyBackendAvailable bool
}

func NewTCPListener(
	address string, backends []Backend, interval time.Duration,
) TCPListener {
	return TCPListener{
		Address:  address,
		Backends: backends,
		Interval: interval,

		shutdown:            make(chan struct{}),
		anyBackendAvailable: false,
	}
}

func handleConnection(clientConn net.Conn, targetAddr string) {
	defer clientConn.Close()

	targetConn, err := net.Dial("tcp", targetAddr)
	if err != nil {
		log.Printf("error connecting to target server: %v\n", err)
		return
	}
	defer targetConn.Close()

	go io.Copy(targetConn, clientConn)
	io.Copy(clientConn, targetConn)
}

func (l *TCPListener) getBackend() *Backend {
	var candidate *Backend = nil

	for _, backend := range l.Backends {
		if backend.Score != 0 &&
			(candidate == nil || candidate.Score > backend.Score) {
			candidate = &backend
		}
	}

	return candidate
}

func (l *TCPListener) updateBackends() error {
	anyAvailable := false
	for _, backend := range l.Backends {
		err := backend.UpdateScore()
		if err != nil {
			return errors.New(
				fmt.Sprintf("backend %s: %s\n", backend.Address, err.Error()),
			)
		}

		anyAvailable = anyAvailable || backend.Score != 0
	}

	if l.anyBackendAvailable && !anyAvailable {
		log.Println("all backends are offline")
	}
	if !l.anyBackendAvailable && anyAvailable {
		log.Println("backends are online again")
	}
	l.anyBackendAvailable = anyAvailable

	time.Sleep(l.Interval)
	return nil
}

func (l *TCPListener) Listen() error {
	if len(l.Backends) == 0 {
		return errors.New("no backend provided")
	}
	listener, err := net.Listen("tcp", l.Address)
	if err != nil {
		return err
	}
	defer listener.Close()

	log.Printf("listening on address %s\n", l.Address)
	forwardingMsg := "forwarding requests to following backends: "
	for index, b := range l.Backends {
		forwardingMsg += b.Address
		if index != len(l.Backends)-1 {
			forwardingMsg += ", "
		}
	}
	log.Println(forwardingMsg)

	err = l.updateBackends()
	if err != nil {
		return err
	}

	go func() {
		for {
			time.Sleep(l.Interval)
			err := l.updateBackends()
			if err != nil {
				log.Printf("error updating backends: %s\n", err.Error())
			}
		}
	}()

	for {
		select {
		case <-l.shutdown:
			return nil
		default:
			clientConn, err := listener.Accept()
			if err != nil {
				log.Printf("error accepting connection: %v\n", err)
				continue
			}

			backend := l.getBackend()

			if backend == nil {
				clientConn.Close()
				continue
			}

			go handleConnection(clientConn, backend.Address)
		}
	}
}

func (l *TCPListener) Shutdown() {
	close(l.shutdown)
}
