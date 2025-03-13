package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStackPopEmpty(t *testing.T) {
	assert := assert.New(t)

	var stack Stack
	_, ok := stack.Pop()

	assert.False(ok)

	stack.Init(7)
	_, ok = stack.Pop()
	assert.False(ok)
}

func TestStackPushPop(t *testing.T) {
	assert := assert.New(t)

	var stack Stack
	stack.Init(5)

	assert.True(stack.Push(6))
	assert.True(stack.Push(3))
	assert.True(stack.Push(9))

	v, ok := stack.Pop()
	assert.Equal(9, v)
	assert.True(ok)

	v, ok = stack.Pop()
	assert.Equal(3, v)
	assert.True(ok)

	v, ok = stack.Pop()
	assert.Equal(6, v)
	assert.True(ok)

	_, ok = stack.Pop()
	assert.False(ok)
}

func TestStackPushToTheLimit(t *testing.T) {
	assert := assert.New(t)

	var stack Stack
	stack.Init(3)

	assert.True(stack.Push(6))
	assert.True(stack.Push(3))
	assert.True(stack.Push(9))
	assert.False(stack.Push(1))

	v, ok := stack.Pop()
	assert.Equal(9, v)
	assert.True(ok)
}
