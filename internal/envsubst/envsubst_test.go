package envsubst

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestReplaceNoError(t *testing.T) {
	assert := assert.New(t)

	text := "Hello, ${TARGET}! FOO = ${ FOO }"
	vars := map[string]string{
		"TARGET": "world",
		"FOO":    "bar",
		"EXTRA":  "extra",
	}
	expectedText := "Hello, world! FOO = bar"

	actualText, err := Replace(text, vars)

	assert.NoError(err)
	assert.Equal(expectedText, actualText)
}

func TestReplaceIgnore(t *testing.T) {
	assert := assert.New(t)

	text := "Hello, $${TARGET}! FOO = ${ FOO }"
	vars := map[string]string{
		"TARGET": "world",
		"FOO":    "bar",
		"EXTRA":  "extra",
	}
	expectedText := "Hello, ${TARGET}! FOO = bar"

	actualText, err := Replace(text, vars)

	assert.NoError(err)
	assert.Equal(expectedText, actualText)
}

func TestReplaceMissingKeys(t *testing.T) {
	assert := assert.New(t)

	text := "Hello, ${TARGET}! FOO = ${ FOO }"
	vars := map[string]string{
		"EXTRA": "extra",
	}
	expectedText := "Hello, ! FOO = "

	actualText, err := Replace(text, vars)
	var keyError *KeyError
	if assert.ErrorAs(err, &keyError) {
		assert.ElementsMatch([]string{"TARGET", "FOO"}, keyError.MissingKeys())
		assert.Equal("no value found for keys: TARGET, FOO", keyError.Error())
	}

	assert.Equal(expectedText, actualText)
}
