package sse

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
)

// JWTAuth returns an OnConnect handler that validates a JWT Bearer token
// from the Authorization header or a token query parameter.
//
// The validateFunc receives the raw token string and should return the
// claims as a map. Return an error to reject the connection.
func JWTAuth(validateFunc func(token string) (map[string]string, error)) func(fiber.Ctx, *Connection) error {
	return func(c fiber.Ctx, conn *Connection) error {
		token := ""

		const bearerPrefix = "Bearer "
		auth := c.Get("Authorization")
		if len(auth) > len(bearerPrefix) && strings.EqualFold(auth[:len(bearerPrefix)], bearerPrefix) {
			token = auth[len(bearerPrefix):]
		}

		if token == "" {
			token = c.Query("token")
		}

		if token == "" {
			return errors.New("missing authentication token")
		}

		claims, err := validateFunc(token)
		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}

		maps.Copy(conn.Metadata, claims)

		return nil
	}
}

// TicketStore is the interface for ticket-based SSE authentication.
// Implement this with Redis, in-memory, or any key-value store.
type TicketStore interface {
	// Set stores a ticket with the given value and TTL.
	Set(ticket, value string, ttl time.Duration) error

	// GetDel atomically retrieves and deletes a ticket (one-time use).
	// Returns empty string and nil error if not found.
	GetDel(ticket string) (string, error)
}

// MemoryTicketStore is an in-memory TicketStore for development and testing.
// Call Close to stop the background cleanup goroutine.
type MemoryTicketStore struct {
	tickets   map[string]memTicket
	done      chan struct{}
	mu        sync.Mutex
	closeOnce sync.Once
}

type memTicket struct {
	expires time.Time
	value   string
}

// NewMemoryTicketStore creates an in-memory ticket store with a background
// cleanup goroutine that evicts expired tickets every 30 seconds.
func NewMemoryTicketStore() *MemoryTicketStore {
	s := &MemoryTicketStore{
		tickets: make(map[string]memTicket),
		done:    make(chan struct{}),
	}
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.mu.Lock()
				now := time.Now()
				for k, v := range s.tickets {
					if now.After(v.expires) {
						delete(s.tickets, k)
					}
				}
				s.mu.Unlock()
			case <-s.done:
				return
			}
		}
	}()

	// Prevent goroutine leak if caller forgets to call Close.
	runtime.SetFinalizer(s, func(s *MemoryTicketStore) {
		s.Close()
	})

	return s
}

// Close stops the background cleanup goroutine. Safe to call multiple times.
func (s *MemoryTicketStore) Close() {
	s.closeOnce.Do(func() {
		close(s.done)
	})
}

// Set stores a ticket with the given value and TTL.
func (s *MemoryTicketStore) Set(ticket, value string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tickets[ticket] = memTicket{value: value, expires: time.Now().Add(ttl)}
	return nil
}

// GetDel atomically retrieves and deletes a ticket (one-time use).
func (s *MemoryTicketStore) GetDel(ticket string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tickets[ticket]
	if !ok {
		return "", nil
	}
	delete(s.tickets, ticket)
	if time.Now().After(t.expires) {
		return "", nil
	}
	return t.value, nil
}

// TicketAuth returns an OnConnect handler that validates a one-time ticket
// from the ticket query parameter.
func TicketAuth(
	store TicketStore,
	parseValue func(value string) (metadata map[string]string, topics []string, err error),
) func(fiber.Ctx, *Connection) error {
	return func(c fiber.Ctx, conn *Connection) error {
		ticket := c.Query("ticket")
		if ticket == "" {
			return errors.New("missing ticket parameter")
		}

		value, err := store.GetDel(ticket)
		if err != nil {
			return fmt.Errorf("ticket validation error: %w", err)
		}
		if value == "" {
			return errors.New("invalid or expired ticket")
		}

		metadata, topics, err := parseValue(value)
		if err != nil {
			return fmt.Errorf("ticket parse error: %w", err)
		}

		maps.Copy(conn.Metadata, metadata)
		if len(topics) > 0 {
			conn.Topics = topics
		}

		return nil
	}
}

// IssueTicket creates a one-time ticket and stores it. Returns the
// ticket string that the client should pass as ?ticket=<value>.
func IssueTicket(store TicketStore, value string, ttl time.Duration) (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate ticket: %w", err)
	}
	ticket := hex.EncodeToString(b)
	if err := store.Set(ticket, value, ttl); err != nil {
		return "", err
	}
	return ticket, nil
}
