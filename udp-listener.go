package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

type udpSession struct {
	clientAddr  *net.UDPAddr
	backendConn *net.UDPConn
	lastActive  time.Time
}

type UDPListener struct {
	Address  string
	Backends []Backend

	currentBackendIndex      int
	currentBackendRepetition int64

	shutdown chan struct{}

	upMutex         sync.RWMutex
	upBackendsCount int

	sessions   map[string]*udpSession
	sessionsMu sync.Mutex
}

func NewUDPListener(address string, backends []Backend) UDPListener {
	return UDPListener{
		Address:  address,
		Backends: backends,

		currentBackendIndex:      0,
		currentBackendRepetition: 0,

		shutdown: make(chan struct{}),

		upMutex:         sync.RWMutex{},
		upBackendsCount: len(backends),

		sessions: make(map[string]*udpSession),
	}
}

func (l *UDPListener) getBackend() *Backend {
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

func (l *UDPListener) updateBackend(b *Backend) error {
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

func (l *UDPListener) isUp() bool {
	l.upMutex.RLock()
	defer l.upMutex.RUnlock()
	return l.upBackendsCount != 0
}

func (l *UDPListener) stopWatchingBackends() {
	for i := range l.Backends {
		backend := &l.Backends[i]
		backend.StopWatchingFilesystem()
	}
}

func (l *UDPListener) watchBackends() {
	for i := range l.Backends {
		backend := &l.Backends[i]

		go backend.WatchFilesystem()

		go func() {
			for {
				select {
				case _, ok := <-backend.ScoreUpdate:
					if !ok {
						return
					}

					prevUp := l.isUp()
					err := l.updateBackend(backend)
					if err != nil {
						log.Print(
							fmt.Sprintf("backend %s: %s\n", backend.Address, err.Error()),
						)
						continue
					}

					currUp := l.isUp()

					if prevUp && !currUp {
						log.Println("all backends are offline")
					}
					if !prevUp && currUp {
						log.Println("backends are online again")
					}
				}
			}
		}()
	}
}

func (l *UDPListener) Shutdown() {
	l.sessionsMu.Lock()
	for key, session := range l.sessions {
		session.backendConn.Close()
		delete(l.sessions, key)
	}
	l.sessionsMu.Unlock()

	close(l.shutdown)
}

func (l *UDPListener) closeSession(key string, session *udpSession) {
	session.backendConn.Close()
	l.sessionsMu.Lock()
	delete(l.sessions, key)
	l.sessionsMu.Unlock()
}

func (l *UDPListener) Listen() error {
	if len(l.Backends) == 0 {
		return errors.New("no backend provided")
	}

	addr, err := net.ResolveUDPAddr("udp", l.Address)
	if err != nil {
		return err
	}

	serverConn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}

	log.Printf("listening on address %s (udp)\n", l.Address)
	forwardingMsg := "forwarding requests to following backends: "
	for index := range l.Backends {
		b := &l.Backends[index]
		forwardingMsg += b.Address
		if index != len(l.Backends)-1 {
			forwardingMsg += ", "
		}
	}
	log.Println(forwardingMsg)

	l.watchBackends()
	defer l.stopWatchingBackends()

	go l.sessionCleaner()

	go func() {
		<-l.shutdown
		serverConn.Close()
	}()

	buf := make([]byte, 65535)
	for {
		n, clientAddr, err := serverConn.ReadFromUDP(buf)
		if err != nil {
			select {
			case <-l.shutdown:
				return nil
			default:
				log.Printf("error reading from udp: %v\n", err)
				continue
			}
		}

		data := make([]byte, n)
		copy(data, buf[:n])

		l.sessionsMu.Lock()
		session, ok := l.sessions[clientAddr.String()]
		l.sessionsMu.Unlock()

		if ok {
			session.lastActive = time.Now()
			_, err = session.backendConn.Write(data)
			if err != nil {
				log.Printf("error writing to backend: %v\n", err)
			}
			continue
		}

		backend := l.getBackend()
		if backend == nil {
			continue
		}

		backendUDPAddr, err := net.ResolveUDPAddr("udp", backend.Address)
		if err != nil {
			log.Printf("error resolving backend address: %v\n", err)
			continue
		}

		backendConn, err := net.DialUDP("udp", nil, backendUDPAddr)
		if err != nil {
			log.Printf("error connecting to backend: %v\n", err)
			continue
		}

		_, err = backendConn.Write(data)
		if err != nil {
			log.Printf("error writing to backend: %v\n", err)
			backendConn.Close()
			continue
		}

		session = &udpSession{
			clientAddr:  clientAddr,
			backendConn: backendConn,
			lastActive:  time.Now(),
		}

		l.sessionsMu.Lock()
		l.sessions[clientAddr.String()] = session
		l.sessionsMu.Unlock()

		go l.readBackendResponses(session, serverConn)
	}
}

func (l *UDPListener) readBackendResponses(session *udpSession, serverConn *net.UDPConn) {
	buf := make([]byte, 65535)
	for {
		select {
		case <-l.shutdown:
			return
		default:
			session.backendConn.SetReadDeadline(time.Now().Add(5 * time.Second))
			n, err := session.backendConn.Read(buf)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				return
			}

			_, err = serverConn.WriteToUDP(buf[:n], session.clientAddr)
			if err != nil {
				log.Printf("error writing to client: %v\n", err)
				return
			}
		}
	}
}

func (l *UDPListener) sessionCleaner() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-l.shutdown:
			return
		case <-ticker.C:
			l.pruneSessions()
		}
	}
}

func (l *UDPListener) pruneSessions() {
	l.sessionsMu.Lock()
	defer l.sessionsMu.Unlock()

	now := time.Now()
	for key, session := range l.sessions {
		if now.Sub(session.lastActive) > 30*time.Second {
			session.backendConn.Close()
			delete(l.sessions, key)
		}
	}
}
