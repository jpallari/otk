package envvar

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLookupGet(t *testing.T) {
	assert := assert.New(t)
	var ok bool
	var v string

	var vars Vars
	err := vars.FromSlice([]string{
		"HOME=/home/bill",
		"EDITOR=vim",
		"SHELL=/bin/bash",
		"LS_COLORS=rs=0:di=01;34:ln=01:36",
		"USER=bill",
	})
	assert.NoError(err)

	v, ok = vars.Lookup("HOME")
	assert.True(ok)
	assert.Equal("/home/bill", v)

	v, ok = vars.Lookup("LS_COLORS")
	assert.True(ok)
	assert.Equal("rs=0:di=01;34:ln=01:36", v)

	assert.Equal("vim", vars.Get("EDITOR"))

	v, ok = vars.Lookup("NOTFOUND")
	assert.False(ok)
	assert.Equal("", v)
}

func TestLookupGetWithMap(t *testing.T) {
	assert := assert.New(t)
	var ok bool
	var v string

	var vars Vars
	vars.FromMap(map[string]string{
		"HOME":      "/home/bill",
		"EDITOR":    "vim",
		"SHELL":     "/bin/bash",
		"LS_COLORS": "rs=0:di=01;34:ln=01:36",
		"USER":      "bill",
	})

	v, ok = vars.Lookup("HOME")
	assert.True(ok)
	assert.Equal("/home/bill", v)

	v, ok = vars.Lookup("LS_COLORS")
	assert.True(ok)
	assert.Equal("rs=0:di=01;34:ln=01:36", v)

	assert.Equal("vim", vars.Get("EDITOR"))

	v, ok = vars.Lookup("NOTFOUND")
	assert.False(ok)
	assert.Equal("", v)
}

func TestInvalidSlice(t *testing.T) {
	assert := assert.New(t)

	var vars Vars
	err := vars.FromSlice([]string{
		"HOME=/home/bill",
		"EDITOR=vim",
		"SHELL/bin/bash",
		"LS_COLORS=rs=0:di=01;34:ln=01:36",
		"USER=bill",
	})

	var formatErr *FormatError
	assert.ErrorAs(err, &formatErr)
	assert.Equal(2, formatErr.Index())
	assert.Equal("SHELL/bin/bash", formatErr.Value())
	assert.Equal("invalid var format at index 2: SHELL/bin/bash", formatErr.Error())
}

func TestAppKey(t *testing.T) {
	assert := assert.New(t)

	assert.Equal(
		"MYAPP_LOG_LEVEL",
		AppKey("myapp", "LOG_LEVEL"),
	)
	assert.Equal(
		"MY_APP_LOG_LEVEL",
		AppKey("my-app", "LOG_LEVEL"),
	)
	assert.Equal(
		"MY_APP_LOG_LEVEL",
		AppKey("my app", "LOG_LEVEL"),
	)
}
