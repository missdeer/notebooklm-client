package util

import (
	"context"
	"math"
	"math/rand"
	"time"
)

func JitteredDelay(baseMs int, jitter float64) int {
	if jitter == 0 {
		jitter = 0.3
	}
	factor := 1 + (rand.Float64()*2-1)*jitter
	return int(math.Round(float64(baseMs) * factor))
}

func HumanSleep(ctx context.Context, baseMs int) error {
	ms := JitteredDelay(baseMs, 0.3)
	t := time.NewTimer(time.Duration(ms) * time.Millisecond)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func JitteredIncrement(base int, jitter float64) int {
	if jitter == 0 {
		jitter = 0.3
	}
	factor := 1 + (rand.Float64()*2-1)*jitter
	return int(math.Round(float64(base) * factor))
}
