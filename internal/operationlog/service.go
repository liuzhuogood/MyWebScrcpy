// Package operationlog persists manual player actions independently of device control.
package operationlog

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"mywebscrcpy/internal/db"
)

var ErrUnavailable = errors.New("操作日志服务不可用或队列已满")

type job struct {
	items  []db.OperationLog
	serial string
	clear  bool
	reply  chan result
}

type result struct {
	deleted int64
	err     error
}

type Service struct {
	database *db.DB
	queue    chan job
	mu       sync.RWMutex
	closed   bool
	done     chan struct{}
	ctx      context.Context
	cancel   context.CancelFunc
}

func New(database *db.DB) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{database: database, queue: make(chan job, 128), done: make(chan struct{}), ctx: ctx, cancel: cancel}
	if database == nil {
		close(s.done)
	} else {
		go s.run()
	}
	return s
}

// Close stops accepting work and drains accepted batches before the database closes.
func (s *Service) Close() {
	s.mu.Lock()
	if !s.closed {
		s.closed = true
		close(s.queue)
	}
	s.mu.Unlock()
	// Drain normally, but a blocked database must not hold shutdown indefinitely.
	timer := time.AfterFunc(5*time.Second, s.cancel)
	<-s.done
	timer.Stop()
	s.cancel()
}

func (s *Service) submit(ctx context.Context, j job) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	j.reply = make(chan result, 1)
	s.mu.RLock()
	if s.closed || s.database == nil {
		s.mu.RUnlock()
		return 0, ErrUnavailable
	}
	select {
	case s.queue <- j:
	default:
		s.mu.RUnlock()
		log.Printf("[OperationLog] 队列已满，拒绝日志请求")
		return 0, ErrUnavailable
	}
	s.mu.RUnlock()
	select {
	case reply := <-j.reply:
		return reply.deleted, reply.err
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-s.done:
		return 0, ErrUnavailable
	}
}

func (s *Service) prune() {
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()
	if err := s.database.PruneOperationLogs(ctx); err != nil {
		log.Printf("[OperationLog] 清理过期日志失败: %v", err)
	}
}

func (s *Service) run() {
	defer close(s.done)
	s.prune()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case j, ok := <-s.queue:
			if !ok {
				return
			}
			ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
			var reply result
			if j.clear {
				reply.deleted, reply.err = s.database.ClearOperationLogs(ctx, j.serial)
			} else {
				reply.err = s.database.AppendOperationLogs(ctx, j.items)
			}
			cancel()
			if reply.err != nil {
				log.Printf("[OperationLog] 日志处理失败 clear=%t count=%d: %v", j.clear, len(j.items), reply.err)
			}
			j.reply <- reply
		case <-ticker.C:
			s.prune()
		}
	}
}
