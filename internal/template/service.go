package template

import "sync"

// Service owns template assets only. It deliberately has no image matching,
// video-frame subscription, device action, or background worker capability.
type Service struct {
	storage   *Storage
	closeOnce sync.Once
}

func NewService(storage *Storage) *Service {
	return &Service{storage: storage}
}

func (s *Service) Storage() *Storage {
	return s.storage
}

func (s *Service) Stop() {
	s.closeOnce.Do(func() {
		if s.storage != nil {
			_ = s.storage.Close()
		}
	})
}
