package lib

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/mbbgs/sdds-lib/utils"
)

// MessageHandler defines the function type for processing received messages.
type MessageHandler func([]byte) error

// Receiver manages the connection, message receiving, and optional RTT logging.
type Receiver struct {
	conn       net.Conn
	config     *Config
	mu         sync.RWMutex
	sendTime   time.Time
	enableRTT  bool
	listening  bool
	stopChan   chan struct{}
	listenerWg sync.WaitGroup
}

func NewReceiver() *Receiver {
	config, err := loadConfig()
	if err != nil {
		config = &Config{
			Rules: Rules{
				RTT:         5 * time.Second,
				MaxRetryCon: 3,
				EnableRTT:   false,
			},
		}
	}
	return &Receiver{
		config:    config,
		enableRTT: config.Rules.EnableRTT,
		stopChan:  make(chan struct{}, 1),
	}
}

func (r *Receiver) SetRTTEnabled(enabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.enableRTT = enabled
}

func (r *Receiver) IsRTTEnabled() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.enableRTT
}

func (r *Receiver) Connect(ctx context.Context, network, address string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var conn net.Conn
	var err error
	base := r.config.Rules.RTT

	for attempt := 0; attempt < r.config.Rules.MaxRetryCon; attempt++ {
		dialer := &net.Dialer{
			Timeout: base,
		}

		conn, err = dialer.DialContext(ctx, network, address)
		if err == nil {
			r.conn = conn
			return nil
		}

		if attempt < r.config.Rules.MaxRetryCon-1 {
			expBackoff := base * (1 << attempt) // RTT * 2^attempt
			jitteredBackoff := utils.Jitter(expBackoff)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(jitteredBackoff):
				// Continue to next attempt
			}
		}
	}

	return fmt.Errorf("failed to connect after %d attempts: %w", r.config.Rules.MaxRetryCon, err)
}


func (r *Receiver) StartListening(ctx context.Context, handler MessageHandler, errorHandler func(error)) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.listening {
		return fmt.Errorf("already listening")
	}
	if r.conn == nil {
		return fmt.Errorf("not connected")
	}
	r.listening = true

	r.listenerWg.Add(1)
	go r.listenLoop(ctx, handler, errorHandler)
	return nil
}

func (r *Receiver) StopListening() {
	r.mu.Lock()
	if !r.listening {
		r.mu.Unlock()
		return
	}
	r.listening = false
	close(r.stopChan)
	r.mu.Unlock()

	r.listenerWg.Wait()

	r.mu.Lock()
	r.stopChan = make(chan struct{}, 1)
	r.mu.Unlock()
}

func (r *Receiver) IsListening() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.listening
}

func (r *Receiver) listenLoop(ctx context.Context, handler MessageHandler, errorHandler func(error)) {
	defer r.listenerWg.Done()
	defer func() {
		r.mu.Lock()
		r.listening = false
		r.mu.Unlock()
	}()

	for {
		select {
		case <-ctx.Done():
			if errorHandler != nil {
				errorHandler(ctx.Err())
			}
			return
		case <-r.stopChan:
			log.Println("Stopping listening")
			return
		default:
			msg, err := r.receiveWithTimeout()
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				if errorHandler != nil {
					errorHandler(err)
				} else {
					log.Printf("Receive error: %v", err)
				}
				if !r.isRecoverableError(err) {
					return
				}
				continue
			}

			go func(m []byte) {
				if err := handler(m); err != nil && errorHandler != nil {
					errorHandler(fmt.Errorf("message handler error: %w", err))
				}
			}(msg)
		}
	}
}

func (r *Receiver) receiveWithTimeout() ([]byte, error) {
	r.mu.RLock()
	conn := r.conn
	enableRTT := r.enableRTT
	sendTime := r.sendTime
	r.mu.RUnlock()

	if conn == nil {
		return nil, fmt.Errorf("not connected")
	}
	conn.SetReadDeadline(time.Now().Add(1 * time.Second))

	buf := make([]byte, 4096)
	receiveStart := time.Now()

	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}

	r.logRTT(enableRTT, sendTime, receiveStart)
	return buf[:n], nil
}

func (r *Receiver) Receive(ctx context.Context) ([]byte, error) {
	r.mu.RLock()
	conn := r.conn
	enableRTT := r.enableRTT
	sendTime := r.sendTime
	r.mu.RUnlock()

	if conn == nil {
		return nil, fmt.Errorf("not connected")
	}
	conn.SetReadDeadline(time.Now().Add(r.config.Rules.RTT))

	buf := make([]byte, 4096)
	receiveStart := time.Now()

	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}

	r.logRTT(enableRTT, sendTime, receiveStart)
	return buf[:n], nil
}

func (r *Receiver) Send(ctx context.Context, data []byte) error {
	r.mu.RLock()
	conn := r.conn
	enableRTT := r.enableRTT
	r.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	if enableRTT {
		r.mu.Lock()
		r.sendTime = time.Now()
		r.mu.Unlock()
	}

	conn.SetWriteDeadline(time.Now().Add(r.config.Rules.RTT))
	_, err := conn.Write(data)

	if err != nil && enableRTT {
		r.mu.Lock()
		r.sendTime = time.Time{}
		r.mu.Unlock()
	}

	return err
}

func (r *Receiver) Close() error {
	r.StopListening()

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.conn != nil {
		err := r.conn.Close()
		r.conn = nil
		return err
	}
	return nil
}

func (r *Receiver) isRecoverableError(err error) bool {
	if err == nil {
		return true
	}
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}
	if errors.Is(err, io.EOF) ||
		errors.Is(err, net.ErrClosed) ||
		err.Error() == "connection reset by peer" {
		return false
	}
	return true
}

// logRTT logs the RTT and distance if enabled.
func (r *Receiver) logRTT(enabled bool, sendTime, receiveTime time.Time) {
	if enabled && !sendTime.IsZero() {
		rtt := receiveTime.Sub(sendTime)
		rttNano := uint64(rtt.Nanoseconds())
		distance := utils.DistanceEstimate(rttNano)
		log.Printf("RTT: %v (%d ns) - Estimated distance: %s",
			rtt, rttNano, utils.FormatDistance(distance))
		r.mu.Lock()
		r.sendTime = time.Time{}
		r.mu.Unlock()
	}
}