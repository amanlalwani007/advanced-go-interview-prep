package main

import (
	"fmt"
	"time"
)

type Server struct {
	host    string
	port    int
	timeout time.Duration
	maxConn int
}

type Option func(*Server)

func WithPort(p int) Option {
	return func(s *Server) { s.port = p }
}

func WithTimeout(t time.Duration) Option {
	return func(s *Server) { s.timeout = t }
}

func WithMaxConn(n int) Option {
	return func(s *Server) { s.maxConn = n }
}

func NewServer(host string, opts ...Option) *Server {
	s := &Server{
		host:    host,
		port:    8080,
		timeout: 30 * time.Second,
		maxConn: 100,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func main() {
	s := NewServer("0.0.0.0", WithPort(9090), WithTimeout(5*time.Second))
	fmt.Printf("Server{host=%s, port=%d, timeout=%v, maxConn=%d}\n",
		s.host, s.port, s.timeout, s.maxConn)
}
