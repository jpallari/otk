package validation

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestFlatValidation(t *testing.T) {
	assert := assert.New(t)
	var root Validator
	root.Init(0, 2)

	root.AddFault("first", "First error")
	root.AddFault("second", "Second error")
	
	assert.Equal(2, root.Count())
	assert.Equal(
`first: First error
second: Second error
`,
		root.String(),
	)
}

func TestLayeredValidation(t *testing.T) {
	assert := assert.New(t)
	var root Validator
	root.Init(4, 20)

	level1 := root.SubValidator("level1")
	level2 := level1.SubValidator("level2")
	level3 := level2.SubValidator("level3")
	level4 := level1.SubValidator("level4")

	level3.AddFault("leaf1", "Leaf 1")
	root.AddFault("root1", "Root 1")
	level3.AddFault("leaf2", "Leaf 2")
	level4.AddFault("leaf3", "Leaf 3")
	root.AddFault("root2", "Root 2")
	
	assert.Equal(5, root.Count())
	assert.Equal(
``,
		root.String(),
	)
}
