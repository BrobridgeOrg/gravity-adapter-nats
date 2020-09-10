package pool

import "time"

type Options struct {
	InitCap     int
	MaxCap      int
	DialTimeout time.Duration
	IdleTimeout time.Duration
}