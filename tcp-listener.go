package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

type TCPListener struct {
	Address  string
	Backends []Backend
	Interval time.Duration

	currentBackendIndex      int
	currentBackendRepetition int64

	shutdown chan struct{}

	upMutex         sync.RWMutex
	upBackendsCount int
}

func NewTCPListener(
	address string, backends []Backend, interval time.Duration,
) TCPListener {
	return TCPListener{
		Address:  address,
		Backends: backends,
		Interval: interval,

		currentBackendIndex:      0,
		currentBackendRepetition: 0,

		shutdown: make(chan struct{}),

		upMutex:         sync.RWMutex{},
		upBackendsCount: len(backends),
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
	if l.isUp() == false {
		return nil
	}

	for l.currentBackendRepetition >=
		l.Backends[l.currentBackendIndex].GetScore() {
		l.currentBackendIndex++
		l.currentBackendIndex %= len(l.Backends)
		l.currentBackendRepetition = 0
	}

	l.currentBackendRepetition++

	return &l.Backends[l.currentBackendIndex]
}

func (l *TCPListener) updateBackend(b *Backend) error {
	prevUp := b.GetScore() != 0

	err := b.UpdateScore()
	if err != nil {
		return err
	}

	currUp := b.GetScore() != 0

	l.upMutex.Lock()
	defer l.upMutex.Unlock()

	if prevUp && !currUp {
		l.upBackendsCount--
	}
	if !prevUp && currUp {
		l.upBackendsCount++
	}

	return nil
}

func (l *TCPListener) isUp() bool {
	l.upMutex.RLock()
	defer l.upMutex.RUnlock()
	return l.upBackendsCount != 0
}

func (l *TCPListener) updateBackends() error {
	prevUp := l.isUp()

	for i := range l.Backends {
		backend := &l.Backends[i]
		err := l.updateBackend(backend)
		if err != nil {
			return errors.New(
				fmt.Sprintf("backend %s: %s\n", backend.Address, err.Error()),
			)
		}
	}

	currUp := l.isUp()

	if prevUp && !currUp {
		log.Println("all backends are offline")
	}
	if !prevUp && currUp {
		log.Println("backends are online again")
	}

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
	for index := range l.Backends {
		b := &l.Backends[index]
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
