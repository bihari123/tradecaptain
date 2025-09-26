package network

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/iceber/iouring-go"
)

// IOUringServer provides ultra-fast network I/O using io_uring
type IOUringServer struct {
	ring     *iouring.IOURing
	listener net.Listener
	handler  func([]byte) []byte
	done     chan struct{}
	wg       sync.WaitGroup
}

// NewIOUringServer creates a new io_uring based server
func NewIOUringServer(addr string, handler func([]byte) []byte) (*IOUringServer, error) {
	// Create io_uring instance with optimal parameters for financial data
	ring, err := iouring.New(1024) // 1024 submission queue entries
	if err != nil {
		return nil, fmt.Errorf("failed to create io_uring: %w", err)
	}

	// Create TCP listener
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		ring.Close()
		return nil, fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	return &IOUringServer{
		ring:     ring,
		listener: listener,
		handler:  handler,
		done:     make(chan struct{}),
	}, nil
}

// Start begins accepting connections with io_uring optimization
func (s *IOUringServer) Start(ctx context.Context) error {
	log.Printf("Starting io_uring server on %s", s.listener.Addr())

	// Start the main event loop
	s.wg.Add(1)
	go s.eventLoop(ctx)

	// Accept connections
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.done:
			return nil
		default:
			conn, err := s.listener.Accept()
			if err != nil {
				select {
				case <-s.done:
					return nil
				default:
					log.Printf("Accept error: %v", err)
					continue
				}
			}

			// Handle connection with io_uring
			s.wg.Add(1)
			go s.handleConnection(conn)
		}
	}
}

// eventLoop runs the main io_uring event processing loop
func (s *IOUringServer) eventLoop(ctx context.Context) {
	defer s.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.done:
			return
		default:
			// Submit pending operations
			submitted, err := s.ring.Submit()
			if err != nil {
				log.Printf("Submit error: %v", err)
				continue
			}

			if submitted > 0 {
				// Wait for completions with timeout
				cqe, err := s.ring.WaitCQE()
				if err != nil {
					log.Printf("WaitCQE error: %v", err)
					continue
				}

				// Process completion
				s.processCompletion(cqe)
				s.ring.SeenCQE(cqe)
			}

			// Small delay to prevent busy waiting
			time.Sleep(100 * time.Microsecond)
		}
	}
}

// handleConnection processes a single connection using io_uring
func (s *IOUringServer) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	// Set connection options for low latency
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetNoDelay(true)                    // Disable Nagle's algorithm
		tcpConn.SetKeepAlive(true)                  // Enable keep-alive
		tcpConn.SetKeepAlivePeriod(30 * time.Second) // Keep-alive period
	}

	buffer := make([]byte, 64*1024) // 64KB buffer

	for {
		select {
		case <-s.done:
			return
		default:
			// Read data using io_uring
			n, err := s.readWithIOUring(conn, buffer)
			if err != nil {
				if err != iouring.ErrWantMoreData {
					log.Printf("Read error: %v", err)
				}
				return
			}

			if n > 0 {
				// Process the data
				request := buffer[:n]
				response := s.handler(request)

				// Write response using io_uring
				if err := s.writeWithIOUring(conn, response); err != nil {
					log.Printf("Write error: %v", err)
					return
				}
			}
		}
	}
}

// readWithIOUring performs zero-copy read using io_uring
func (s *IOUringServer) readWithIOUring(conn net.Conn, buffer []byte) (int, error) {
	// Get file descriptor from connection
	fd, err := s.getConnFD(conn)
	if err != nil {
		return 0, err
	}

	// Prepare read operation
	sqe := s.ring.GetSQE()
	if sqe == nil {
		return 0, fmt.Errorf("no SQE available")
	}

	// Set up read operation
	sqe.PrepareRead(int(fd), buffer, 0)
	sqe.SetUserData(1) // Mark as read operation

	// Submit and wait for completion
	submitted, err := s.ring.Submit()
	if err != nil {
		return 0, err
	}

	if submitted == 0 {
		return 0, iouring.ErrWantMoreData
	}

	// Wait for completion
	cqe, err := s.ring.WaitCQE()
	if err != nil {
		return 0, err
	}
	defer s.ring.SeenCQE(cqe)

	// Check result
	result := cqe.GetRes()
	if result < 0 {
		return 0, fmt.Errorf("read failed with result: %d", result)
	}

	return int(result), nil
}

// writeWithIOUring performs zero-copy write using io_uring
func (s *IOUringServer) writeWithIOUring(conn net.Conn, data []byte) error {
	if len(data) == 0 {
		return nil
	}

	// Get file descriptor from connection
	fd, err := s.getConnFD(conn)
	if err != nil {
		return err
	}

	// Prepare write operation
	sqe := s.ring.GetSQE()
	if sqe == nil {
		return fmt.Errorf("no SQE available")
	}

	// Set up write operation
	sqe.PrepareWrite(int(fd), data, 0)
	sqe.SetUserData(2) // Mark as write operation

	// Submit and wait for completion
	submitted, err := s.ring.Submit()
	if err != nil {
		return err
	}

	if submitted == 0 {
		return fmt.Errorf("failed to submit write operation")
	}

	// Wait for completion
	cqe, err := s.ring.WaitCQE()
	if err != nil {
		return err
	}
	defer s.ring.SeenCQE(cqe)

	// Check result
	result := cqe.GetRes()
	if result < 0 {
		return fmt.Errorf("write failed with result: %d", result)
	}

	return nil
}

// processCompletion handles io_uring completion events
func (s *IOUringServer) processCompletion(cqe *iouring.CompletionQueueEvent) {
	userData := cqe.GetUserData()
	result := cqe.GetRes()

	switch userData {
	case 1: // Read operation
		if result > 0 {
			// Read completed successfully
		} else {
			log.Printf("Read operation failed: %d", result)
		}
	case 2: // Write operation
		if result > 0 {
			// Write completed successfully
		} else {
			log.Printf("Write operation failed: %d", result)
		}
	default:
		log.Printf("Unknown operation completed: %d", userData)
	}
}

// getConnFD extracts file descriptor from net.Conn
func (s *IOUringServer) getConnFD(conn net.Conn) (uintptr, error) {
	// This is a simplified version - in practice, you'd need to handle
	// different connection types and extract the underlying file descriptor
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return 0, fmt.Errorf("connection is not TCP")
	}

	// Get the underlying file
	file, err := tcpConn.File()
	if err != nil {
		return 0, err
	}
	defer file.Close()

	return file.Fd(), nil
}

// Stop gracefully shuts down the server
func (s *IOUringServer) Stop() error {
	close(s.done)

	// Close listener
	if err := s.listener.Close(); err != nil {
		log.Printf("Error closing listener: %v", err)
	}

	// Wait for all goroutines to finish
	s.wg.Wait()

	// Close io_uring
	if err := s.ring.Close(); err != nil {
		return fmt.Errorf("failed to close io_uring: %w", err)
	}

	return nil
}

// GetStats returns server statistics
func (s *IOUringServer) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"ring_fd": s.ring.Fd(),
		"address": s.listener.Addr().String(),
	}
}