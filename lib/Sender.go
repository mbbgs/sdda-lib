package lib 

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/mbbgs/sdds-lib/utils"
)


type Sender struct {
	listener    net.Listener
	config      *Config
	mu          sync.Mutex
	enableRTT   bool						// Control RTT calculation for server side
	listening   bool						// Track if continuous listening is active
	stopChan    chan struct{} 	// Channel to signal stop listening
	listenerWg  sync.WaitGroup	// Wait group for listener goroutine
}


func NewSender() *Sender {
	config, err := loadConfig()
	if err != nil {
		// Default values if config fails to load
		config = &Config{}
		config.Rules.RTT = 5 * time.Second
		config.Rules.MaxRetryCon = 5
		config.Rules.EnableRTT = false
	}
	
	return &Sender{
		config:    config,
		enableRTT: config.Rules.EnableRTT,
		stopChan:  make(chan struct{},1),
	}
}

// SetRTTEnabled allows runtime control of RTT calculation for sender
func (s *Sender) SetRTTEnabled(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enableRTT = enabled
}

// IsRTTEnabled returns current RTT calculation status for sender
func (s *Sender) IsRTTEnabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enableRTT
}

func (s *Sender) Listen(ctx context.Context, network, address string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var listener net.Listener
	var err error

	base := s.config.Rules.RTT
	maxBackoff := 30 * time.Second

	for attempt := 0; attempt < s.config.Rules.MaxRetryCon; attempt++ {
		listener, err = net.Listen(network, address)
		if err == nil {
			s.listener = listener
			log.Printf("Listening on %s after %d attempt(s)", address, attempt+1)
			return nil
		}

		if attempt < s.config.Rules.MaxRetryCon-1 {
			expBackoff := base * time.Duration(1<<attempt) // 2^attempt * base
			if expBackoff > maxBackoff {
				expBackoff = maxBackoff
			}
			jittered := jitter(expBackoff)

			log.Printf("Listen attempt %d failed: %v. Retrying in %s...", attempt+1, err, jittered)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(jittered):
				// Retry next
			}
		}
	}

	return fmt.Errorf("failed to listen after %d attempts: %w", s.config.Rules.MaxRetryCon, err)
}

// StartAccepting begins continuous accepting of connections
// connectionHandler: function to handle each new connection
// errorHandler: function to handle errors (optional, can be nil)
func (s *Sender) StartAccepting(ctx context.Context, connectionHandler func(net.Conn), errorHandler func(error)) error {
	s.mu.Lock()
	if s.listening {
		s.mu.Unlock()
		return fmt.Errorf("already accepting connections")
	}
	if s.listener == nil {
		s.mu.Unlock()
		return fmt.Errorf("not listening")
	}
	s.listening = true
	s.mu.Unlock()

	s.listenerWg.Add(1)
	go s.acceptLoop(ctx, connectionHandler, errorHandler)
	
	return nil
}

// StopAccepting stops continuous accepting of connections
func (s *Sender) StopAccepting() {
	s.mu.Lock()
	if !s.listening {
		s.mu.Unlock()
		return
	}
	s.listening = false
	s.mu.Unlock()

	close(s.stopChan)
	s.listenerWg.Wait()
	
	// Create new stop channel for potential future use
	s.mu.Lock()
	s.stopChan = make(chan struct{})
	s.mu.Unlock()
}

// IsAccepting returns whether continuous accepting is active
func (s *Sender) IsAccepting() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listening
}

// acceptLoop is the internal continuous accepting loop
func (s *Sender) acceptLoop(ctx context.Context, connectionHandler func(net.Conn), errorHandler func(error)) {
	defer s.listenerWg.Done()
	defer func() {
		s.mu.Lock()
		s.listening = false
		s.mu.Unlock()
	}()

	for {
		select {
		case <-ctx.Done():
			if errorHandler != nil {
				errorHandler(ctx.Err())
			}
			return
		case <-s.stopChan:
			log.Println("Stopping continuous accepting")
			return
		default:
			s.mu.Lock()
			listener := s.listener
			s.mu.Unlock()
			
			if listener == nil {
				if errorHandler != nil {
					errorHandler(fmt.Errorf("listener is nil"))
				}
				return
			}

			// Set a deadline to allow checking for stop signals
			if tcpListener, ok := listener.(*net.TCPListener); ok {
				tcpListener.SetDeadline(time.Now().Add(1 * time.Second))
			}

			conn, err := listener.Accept()
			if err != nil {
				// Check if it's a timeout
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue // Timeout is expected, continue accepting
				}

				if errorHandler != nil {
					errorHandler(err)
				} else {
					log.Printf("Accept error: %v", err)
				}
				
				// For serious errors, stop accepting
				return
			}

			// Handle connection in a separate goroutine
			go connectionHandler(conn)
		}
	}
}

func (s *Sender) Accept(ctx context.Context) (net.Conn, error) {
	s.mu.Lock()
	listener := s.listener
	s.mu.Unlock()
	
	if listener == nil {
		return nil, fmt.Errorf("not listening")
	}
	
	return listener.Accept()
}

func (s *Sender) Close() error {
	// Stop accepting first
	s.StopAccepting()
	
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.listener != nil {
		err := s.listener.Close()
		s.listener = nil
		return err
	}
	return nil
}

// ConnectionHandler wraps a net.Conn to provide RTT measurement for server-side connections
type ConnectionHandler struct {
	conn      net.Conn  
	sendTime  time.Time
	enableRTT bool
	mu        sync.Mutex
}

// NewConnectionHandler creates a new connection handler with RTT capability
func NewConnectionHandler(conn net.Conn, enableRTT bool) *ConnectionHandler {
	return &ConnectionHandler{
		conn:      conn,
		enableRTT: enableRTT,
	}
}

// Read implements RTT calculation for server-side reads
func (ch *ConnectionHandler) Read(data []byte) (int, error) {
	receiveStart := time.Now()
	
	n, err := ch.conn.Read(data)
	if err != nil {
		return n, err
	}
	
	// Calculate RTT if enabled and we have a send time
	ch.mu.Lock()
	if ch.enableRTT && !ch.sendTime.IsZero() {
		rtt := receiveStart.Sub(ch.sendTime)
		rttNano := uint64(rtt.Nanoseconds())
		
		distance := utils.DistanceEstimate(rttNano)
		formattedDistance := utils.FormatDistance(distance)
		
		log.Printf("Server RTT: %v (%d ns) - Estimated distance: %s", 
			rtt, rttNano, formattedDistance)
		
		ch.sendTime = time.Time{}
	}
	ch.mu.Unlock()
	
	return n, err
}

// Write implements send time tracking for RTT calculation
func (ch *ConnectionHandler) Write(data []byte) (int, error) {
	if ch.enableRTT {
		ch.mu.Lock()
		ch.sendTime = time.Now()
		ch.mu.Unlock()
	}
	
	n, err := ch.conn.Write(data)
	if err != nil && ch.enableRTT {
		ch.mu.Lock()
		ch.sendTime = time.Time{}
		ch.mu.Unlock()
	}
	
	return n, err
}

// SetRTTEnabled controls RTT calculation for this connection
func (ch *ConnectionHandler) SetRTTEnabled(enabled bool) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	ch.enableRTT = enabled
}

// Close closes the underlying connection
func (ch *ConnectionHandler) Close() error {
	return ch.conn.Close()
}