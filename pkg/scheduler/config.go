package scheduler

import (
	"fmt"
	"time"
)

// Config controls scheduling. An empty Timezone follows time.Local, which the
// application entry point may configure before constructing the scheduler.
type Config struct {
	Timezone string
}

func (c Config) location() (*time.Location, error) {
	if c.Timezone == "" {
		return time.Local, nil
	}
	loc, err := time.LoadLocation(c.Timezone)
	if err != nil {
		return nil, fmt.Errorf("scheduler: load timezone: %w", err)
	}
	return loc, nil
}
