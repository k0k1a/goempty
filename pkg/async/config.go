package async

import (
	"errors"
	"math"
	"time"
)

// Config controls a process-local pool. Zero values use 4 workers,
// 1000 blocked submitters and a 30-second execution timeout.
// Changes take effect only when a new pool is constructed.
type Config struct {
	Concurrency      int           `mapstructure:"concurrency"`
	MaxBlockingTasks int           `mapstructure:"max_blocking_tasks"`
	Timeout          time.Duration `mapstructure:"timeout"`
}

func (c Config) Validate() error {
	if c.Concurrency < 0 || c.Concurrency > math.MaxInt32 || c.MaxBlockingTasks < 0 || c.Timeout < 0 {
		return errors.New("async: concurrency must fit a positive int32, max blocking tasks and timeout must be nonnegative; zero uses defaults")
	}
	return nil
}
