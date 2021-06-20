package service

import "github.com/jonboulle/clockwork"

type Option func(*Service) error

func WithClock(clock clockwork.Clock) Option {
	return func(s *Service) error {
		s.clock = clock
		return nil
	}
}
