package service

import "github.com/jonboulle/clockwork"

type Option func(*Service) error

// WithClock functionally configure the service with a clock.
func WithClock(clock clockwork.Clock) Option {
	return func(s *Service) error {
		s.clock = clock
		return nil
	}
}
