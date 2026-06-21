package transport

import (
	"fmt"
	"net"
	"sync"
)

// Server is a TCP listener that hands each accepted connection (wrapped as a
// *Conn) to a handler goroutine.
type Server struct {
	ln      net.Listener
	handler func(*Conn)
	wg      sync.WaitGroup

	mu     sync.Mutex
	closed bool
	active map[*Conn]struct{}
}

// Listen binds addr and serves accepted connections with handler until Close.
func Listen(addr string, handler func(*Conn)) (*Server, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listening on %s: %w", addr, err)
	}
	s := &Server{ln: ln, handler: handler, active: make(map[*Conn]struct{})}
	s.wg.Add(1)
	go s.accept()
	return s, nil
}

// Addr returns the actual listen address (useful when binding :0 in tests).
func (s *Server) Addr() string { return s.ln.Addr().String() }

func (s *Server) accept() {
	defer s.wg.Done()
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if closed {
				return
			}
			continue
		}
		c := NewConn(conn)

		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			c.Close()
			return
		}
		s.active[c] = struct{}{}
		s.wg.Add(1)
		s.mu.Unlock()

		go func() {
			defer s.wg.Done()
			defer func() {
				s.mu.Lock()
				delete(s.active, c)
				s.mu.Unlock()
				c.Close()
			}()
			s.handler(c)
		}()
	}
}

// Close stops accepting, closes all active connections, and waits for in-flight
// handlers to return.
func (s *Server) Close() error {
	s.mu.Lock()
	s.closed = true
	for c := range s.active {
		c.Close()
	}
	s.mu.Unlock()

	err := s.ln.Close()
	s.wg.Wait()
	return err
}

// Dial connects to a transport server and returns a framed connection.
func Dial(addr string) (*Conn, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dialing %s: %w", addr, err)
	}
	return NewConn(conn), nil
}
