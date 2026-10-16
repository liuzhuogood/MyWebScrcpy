package ocr

import (
	"context"
	"errors"
	"log"
	"time"

	"mywebscrcpy/internal/db"
	"mywebscrcpy/internal/servicepython"
	"mywebscrcpy/internal/ws"
)

type pythonRuntime interface {
	Acquire(string) (string, func(), error)
	Prepare(context.Context, string, func(string)) (string, func(), error)
	Check(context.Context, string, string) error
	PaddleScript(string) (string, error)
	IndraScript(string) (string, error)
}

func NewManagedService(database *db.DB, hub *ws.Hub, runtime pythonRuntime, control func(bool, func() error) error) *Service {
	s := NewService(database, hub)
	s.runtime, s.control = runtime, control
	s.indra.runtime = runtime
	return s
}

func (s *Service) startManagedPaddle(ctx context.Context) (err error) {
	s.mu.Lock()
	if s.prepareDone != nil {
		s.mu.Unlock()
		return errors.New("OCR 服务正在准备或启动")
	}
	op, cancel := context.WithTimeout(ctx, 17*time.Minute)
	done := make(chan struct{})
	s.prepareCancel, s.prepareDone, s.preparePhase = cancel, done, "starting"
	s.lastStartErr = ""
	s.mu.Unlock()
	defer func() {
		cancel()
		s.mu.Lock()
		s.prepareCancel, s.prepareDone, s.preparePhase = nil, nil, ""
		s.installedAt = time.Time{}
		if err != nil {
			s.lastStartErr = err.Error()
		}
		s.mu.Unlock()
		close(done)
		if err != nil {
			log.Printf("[OCR] 准备或启动失败: %v", err)
		}
	}()
	python, release, err := s.runtime.Prepare(op, s.profile, func(phase string) {
		s.mu.Lock()
		s.preparePhase = phase
		s.mu.Unlock()
	})
	if err != nil {
		return err
	}
	var script string
	if s.profile == servicepython.IndraOCR {
		script, err = s.runtime.IndraScript(python)
	} else {
		script, err = s.runtime.PaddleScript(python)
	}
	if err != nil {
		release()
		return err
	}
	if err := op.Err(); err != nil {
		release()
		return err
	}
	return s.startPaddleProcess(op, python, script, release)
}
