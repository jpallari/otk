package retry

import (
	"context"
	"errors"
	"math"
	"math/rand/v2"
	"time"

	"github.com/rs/zerolog"
)

var (
	ErrCtxCancelled = errors.New("cancelled by context")
)

type DelayFunc func(int) time.Duration

func ConstantBackoff(duration time.Duration) DelayFunc {
	return func(attemptNr int) time.Duration {
		return duration
	}
}

func LinearBackoff(initialDelay, increment time.Duration) DelayFunc {
	return func(attemptNr int) time.Duration {
		return initialDelay + time.Duration(attemptNr)*increment
	}
}

func ExponentialBackoff(minDelay, maxDelay time.Duration) DelayFunc {
	return func(attemptNr int) time.Duration {
		mult := math.Pow(2, float64(attemptNr)) * float64(minDelay)
		sleep := time.Duration(mult)
		if float64(sleep) != mult || sleep > maxDelay {
			sleep = maxDelay
		}
		return sleep
	}
}

func (this DelayFunc) WithFullJitter(
	rand rand.Rand,
) DelayFunc {
	return func(attemptNr int) time.Duration {
		delay := this(attemptNr)
		r := rand.Float64()
		return time.Duration(float64(delay) * r)
	}
}

func (this DelayFunc) WithJitter(
	rand rand.Rand,
	maxJitter time.Duration,
) DelayFunc {
	return func(attemptNr int) time.Duration {
		delay := this(attemptNr)

		jitterBase := delay / 2
		if jitterBase > maxJitter {
			jitterBase = maxJitter
		}

		r := rand.Float64()
		jitter := time.Duration(float64(jitterBase) * r)
		return delay + jitter
	}
}

type Retryable func(context.Context) error

func Retry(
	ctx context.Context,
	maxRetries int,
	delayFunc DelayFunc,
	f Retryable,
) error {
	log := zerolog.Ctx(ctx)
	var err error
	timer := time.NewTimer(0)
	var cancelErr *cancelError

	for attempt := 0; attempt <= maxRetries; attempt += 1 {
		select {
		case <-ctx.Done():
			timer.Stop()
			return ErrCtxCancelled
		case <-timer.C:
			err = f(ctx)
			if err == nil {
				return nil
			}
			if errors.As(err, &cancelErr) {
				return cancelErr.Cause
			}

			delay := delayFunc(attempt)
			log.Debug().
				Err(err).
				Str("nextAttemptIn", delay.String()).
				Int("attempt", attempt).
				Msg("retryable function failed")
			timer = time.NewTimer(delay)
		}
	}
	return err
}

type cancelError struct {
	Cause error
}

func (this *cancelError) Error() string {
	return this.Cause.Error()
}

func Cancel(err error) error {
	return &cancelError{Cause: err}
}
