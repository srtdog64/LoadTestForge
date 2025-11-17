package strategy

import (
	"context"
)

type Target struct {
	URL     string
	Method  string
	Headers map[string]string
	Body    []byte
}

type AttackStrategy interface {
	Execute(ctx context.Context, target Target) error
	Name() string
}

type Result struct {
	Success      bool
	Error        error
	ResponseTime int64
	StatusCode   int
}
