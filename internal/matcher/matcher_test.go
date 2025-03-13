package matcher

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatcherParsing(t *testing.T) {
	t.Run("parses json string with regex", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		var matcher M
		matcherJson := `"/v[0-9]+\\.[0-9]+\\.[0-9]+/"`
		require.NoError(json.Unmarshal([]byte(matcherJson), &matcher))
		matcherBytes, err := json.Marshal(&matcher)
		require.NoError(err)
		
		assert.True(matcher.UsesRegex())
		assert.Equal(`/v[0-9]+\.[0-9]+\.[0-9]+/`, matcher.String())
		assert.Equal([]byte(matcherJson), matcherBytes)
	})

	t.Run("parses json string no regex", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		var matcher M
		matcherJson := `"v1.2.3"`
		require.NoError(json.Unmarshal([]byte(matcherJson), &matcher))
		matcherBytes, err := json.Marshal(&matcher)
		require.NoError(err)
		
		assert.False(matcher.UsesRegex())
		assert.Equal("v1.2.3", matcher.String())
		assert.Equal([]byte(matcherJson), matcherBytes)
	})

	t.Run("parses json object with regex", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		var matcher M
		matcherJson := `{"spec": "main.*", "useRegex": true}`
		require.NoError(json.Unmarshal([]byte(matcherJson), &matcher))
		matcherBytes, err := json.Marshal(&matcher)
		require.NoError(err)
		
		assert.True(matcher.UsesRegex())
		assert.Equal("/main.*/", matcher.String())
		assert.Equal([]byte(`"/main.*/"`), matcherBytes)
	})

	t.Run("parses json object no regex", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		var matcher M
		matcherJson := `{"spec": "main", "useRegex": false}`
		require.NoError(json.Unmarshal([]byte(matcherJson), &matcher))
		matcherBytes, err := json.Marshal(&matcher)
		require.NoError(err)
		
		assert.False(matcher.UsesRegex())
		assert.Equal("main", matcher.String())
		assert.Equal([]byte(`"main"`), matcherBytes)
	})
}

func TestMatcherMatching(t *testing.T) {
	t.Run("matches regex", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		var matcher M
		require.NoError(matcher.FromString(`/v[0-9]+\.[0-9]+\.[0-9]+/`))

		assert.True(matcher.MatchString("v1.2.3"))
		assert.True(matcher.MatchString("v0.10.22"))
		assert.False(matcher.MatchString("0.10.22"))
		assert.False(matcher.MatchString("1.2.3"))
		assert.False(matcher.MatchString(""))
	})

	t.Run("matches plain", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		var matcher M
		require.NoError(matcher.FromString("hello world"))

		assert.True(matcher.MatchString("hello world"))
		assert.False(matcher.MatchString("hello"))
		assert.False(matcher.MatchString(""))
	})
}
