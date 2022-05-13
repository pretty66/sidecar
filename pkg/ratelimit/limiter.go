package ratelimit

import (
	"context"

	"github.com/labstack/echo/v4"
)

// Op operations type.
type Op int

const (
	// Success opertion type: success
	Success Op = iota
	// Ignore opertion type: ignore
	Ignore
	// Drop opertion type: drop
	Drop
)

type AllowOptions struct{}

// AllowOptions allow options.
type AllowOption interface {
	Apply(*AllowOptions)
}

// DoneInfo done info.
type DoneInfo struct {
	Err error
	Op  Op
}

// DefaultAllowOpts returns the default allow options.
func DefaultAllowOpts() *AllowOptions {
	return &AllowOptions{}
}

// Limiter limit interface.
type Limiter interface {
	Allow(ctx context.Context, opts ...AllowOption) (func(info DoneInfo), error)
	HTTPBBRAllow(next echo.HandlerFunc) echo.HandlerFunc
}
