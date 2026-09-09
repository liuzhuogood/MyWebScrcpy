// Package devicegate coordinates ordinary device operations while allowing an
// explicitly bounded parallel mode for callers that accept device-state races.
package devicegate

import (
	"context"
	"errors"
)

type Gate struct {
	serial   chan struct{}
	parallel chan struct{}
}

func New(maxParallel int) *Gate {
	if maxParallel < 1 {
		maxParallel = 1
	}
	return &Gate{serial: make(chan struct{}, 1), parallel: make(chan struct{}, maxParallel)}
}

func (g *Gate) Run(ctx context.Context, parallel bool, fn func(context.Context) error) error {
	if parallel {
		select {
		case g.parallel <- struct{}{}:
			defer func() { <-g.parallel }()
		case <-ctx.Done():
			return ctx.Err()
		}
	} else {
		select {
		case g.serial <- struct{}{}:
			defer func() { <-g.serial }()
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if fn == nil {
		return errors.New("missing device operation")
	}
	return fn(ctx)
}
