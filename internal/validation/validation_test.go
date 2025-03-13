package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFlatValidation(t *testing.T) {
	assert := assert.New(t)
	var root V
	root.Init()

	root.Fail("first", "First error")
	root.Fail("second", "Second error")

	assert.Equal(2, root.Count())
	assert.Equal(
		`first: First error
second: Second error
`,
		root.Report(),
	)
}

func TestLayeredValidation(t *testing.T) {
	assert := assert.New(t)
	var root V
	root.Init()

	level1 := root.Sub("level1")
	level2 := level1.Sub("level2")
	level3 := level2.Sub("level3")
	level4a := level1.Sub("level4")
	level4b := level1.Sub("level4")
	_ = level1.Sub("level1_empty")
	_ = level2.Sub("level2_empty")
	_ = level4a.Sub("level4_empty")

	level3.Fail("leaf1", "Leaf 1")
	root.Fail("root1", "Root 1")
	level3.Fail("leaf2", "Leaf 2")
	level4a.Fail("leaf3", "Leaf 3")
	root.Fail("root2", "Root 2")
	level4b.Fail("leaf4", "Leaf 4")

	assert.Equal(6, root.Count())
	assert.Equal(
		`root1: Root 1
root2: Root 2
level1:
  level2:
    level3:
      leaf1: Leaf 1
      leaf2: Leaf 2
  level4:
    leaf3: Leaf 3
    leaf4: Leaf 4
`,
		root.Report(),
	)
}
