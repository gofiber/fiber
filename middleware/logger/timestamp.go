package logger

import (
	"sync"
	"sync/atomic"
	"time"
)

type timestampKey struct {
	format   string
	timeZone string
	interval time.Duration
}

type sharedTimestamp struct {
	next     time.Time
	location *time.Location
	value    atomic.Pointer[string]
	format   string
	interval time.Duration
}

func (t *sharedTimestamp) Load() string {
	value := t.value.Load()
	if value == nil {
		return ""
	}

	return *value
}

func (t *sharedTimestamp) store(now time.Time) {
	value := now.In(t.location).Format(t.format)
	t.value.Store(&value)
}

type timestampScheduler struct {
	states map[timestampKey]*sharedTimestamp
	wake   chan struct{}
	once   sync.Once

	mu sync.Mutex
}

func newTimestampScheduler() *timestampScheduler {
	return &timestampScheduler{
		states: make(map[timestampKey]*sharedTimestamp),
		wake:   make(chan struct{}, 1),
	}
}

func (s *timestampScheduler) get(format string, location *time.Location, interval time.Duration) *sharedTimestamp {
	s.once.Do(func() {
		go s.run()
	})

	return s.getOrCreate(format, location, interval, time.Now())
}

func (s *timestampScheduler) getOrCreate(format string, location *time.Location, interval time.Duration, now time.Time) *sharedTimestamp {
	key := timestampKey{
		format:   format,
		interval: interval,
		timeZone: location.String(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if state, ok := s.states[key]; ok {
		return state
	}

	state := &sharedTimestamp{
		format:   format,
		location: location,
		interval: interval,
		next:     now.Add(interval),
	}
	state.store(now)
	s.states[key] = state

	select {
	case s.wake <- struct{}{}:
	default:
	}

	return state
}

func (s *timestampScheduler) run() {
	timer := time.NewTimer(time.Hour)
	defer timer.Stop()

	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}

	for {
		s.mu.Lock()
		if len(s.states) == 0 {
			s.mu.Unlock()
			<-s.wake
			continue
		}

		now := time.Now()
		var next time.Time
		for _, state := range s.states {
			if !now.Before(state.next) {
				state.store(now)
				state.next = now.Add(state.interval)
			}
			if next.IsZero() || state.next.Before(next) {
				next = state.next
			}
		}
		wait := time.Until(next)
		s.mu.Unlock()

		if wait < 0 {
			wait = 0
		}

		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(wait)

		select {
		case <-timer.C:
		case <-s.wake:
		}
	}
}

var sharedTimestamps = newTimestampScheduler()
