package retry

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConstantBackoff(t *testing.T) {
	assert := assert.New(t)
	expectedTime := time.Millisecond * 10
	delayFunc := ConstantBackoff(expectedTime)

	for i := range 10000 {
		nextDuration := delayFunc(i)
		assert.Equal(expectedTime, nextDuration)
	}
}

func TestLinearBackoff(t *testing.T) {
	assert := assert.New(t)
	initialDelay := time.Millisecond * 100
	increment := time.Millisecond * 10
	delayFunc := LinearBackoff(initialDelay, increment)

	assert.Equal(
		initialDelay,
		delayFunc(0),
	)

	for i := range 1000 {
		nextDuration := delayFunc(i)
		assert.Equal(
			initialDelay+time.Duration(i)*increment,
			nextDuration,
		)
	}
}

func TestExponentialBackoffMinMax(t *testing.T) {
	assert := assert.New(t)
	minTime := time.Millisecond * 100
	maxTime := time.Millisecond * 2000
	delayFunc := ExponentialBackoff(minTime, maxTime)

	for i := range 10000 {
		nextDuration := delayFunc(i)
		assert.LessOrEqual(minTime, nextDuration)
		assert.LessOrEqual(nextDuration, maxTime)
	}
}

func oneMsBackoff(attempt int) time.Duration {
	return 1 * time.Millisecond
}

type callCounter struct {
	nrOfCalls int
}

func (c *callCounter) call() {
	c.nrOfCalls += 1
}

func TestRetryInstantHappy(t *testing.T) {
	assert := assert.New(t)
	cc := &callCounter{}

	err := Retry(
		context.Background(),
		3,
		oneMsBackoff,
		func(ctx context.Context) error {
			cc.call()
			return nil
		},
	)

	assert.NoError(err)
	assert.Equal(1, cc.nrOfCalls)
}

func TestRetryEventualHappy(t *testing.T) {
	assert := assert.New(t)
	cc := &callCounter{}

	err := Retry(
		context.Background(),
		3,
		oneMsBackoff,
		func(ctx context.Context) error {
			cc.call()
			if cc.nrOfCalls < 2 {
				return fmt.Errorf("nr of calls = %d", cc.nrOfCalls)
			}
			return nil
		},
	)

	assert.NoError(err)
	assert.Equal(2, cc.nrOfCalls)
}

func TestRetryUnhappy(t *testing.T) {
	assert := assert.New(t)
	cc := &callCounter{}

	err := Retry(
		context.Background(),
		3,
		oneMsBackoff,
		func(ctx context.Context) error {
			cc.call()
			return fmt.Errorf("nr of calls = %d", cc.nrOfCalls)
		},
	)

	assert.Error(err)
	assert.Equal(4, cc.nrOfCalls)
}

func TestRetryCtxCancel(t *testing.T) {
	assert := assert.New(t)
	cc := &callCounter{}
	ctx, cancel := context.WithCancel(context.Background())

	err := Retry(
		ctx,
		3,
		oneMsBackoff,
		func(ctx context.Context) error {
			cc.call()
			if cc.nrOfCalls == 2 {
				cancel()
			}
			if cc.nrOfCalls > 2 {
				return nil
			}
			return fmt.Errorf("nr of calls = %d", cc.nrOfCalls)
		},
	)

	assert.Equal(ErrCtxCancelled, err)
	assert.Equal(2, cc.nrOfCalls)
}

func TestRetryCancel(t *testing.T) {
	assert := assert.New(t)
	cc := &callCounter{}
	expectedErr := fmt.Errorf("expected error from cancel")

	err := Retry(
		context.Background(),
		3,
		oneMsBackoff,
		func(ctx context.Context) error {
			cc.call()
			if cc.nrOfCalls == 2 {
				return Cancel(expectedErr)
			}
			if cc.nrOfCalls > 2 {
				return nil
			}
			return fmt.Errorf("nr of calls = %d", cc.nrOfCalls)
		},
	)

	assert.Equal(expectedErr, err)
	assert.Equal(2, cc.nrOfCalls)
}
