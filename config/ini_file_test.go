package config

import (
	"github.com/stretchr/testify/assert"

	"testing"

	"github.com/go-ini/ini"
)

func TestGetStrArray(t *testing.T) {
	assert := assert.New(t)
	f, _ := ini.Load([]byte("[Main]\n\nports = 10,15,20,25"))
	conf := File{
		f,
		"some/path",
	}

	ports, err := conf.GetStrArray("Main", "ports", ',')
	assert.Nil(err)
	assert.Equal(ports, []string{"10", "15", "20", "25"})
}

func TestGetStrArrayWithCommas(t *testing.T) {
	assert := assert.New(t)
	f, _ := ini.Load([]byte("[trace.ignore]\n\nresource = \"x,y,z\", foobar"))
	conf := File{f, "some/path"}

	vals, err := conf.GetStrArray("trace.ignore", "resource", ',')
	assert.Nil(err)
	assert.Equal(vals, []string{"x,y,z", "foobar"})
}

func TestGetStrArrayWithEscapedSequences(t *testing.T) {
	assert := assert.New(t)
	f, _ := ini.Load([]byte("[trace.ignore]\n\nresource = \"foo\\.bar\", \"\"\""))
	conf := File{f, "some/path"}

	vals, err := conf.GetStrArray("trace.ignore", "resource", ',')
	assert.Nil(err)
	assert.Equal(vals, []string{`foo\.bar`, `"`})
}

func TestGetStrArrayEmpty(t *testing.T) {
	assert := assert.New(t)
	f, _ := ini.Load([]byte("[Main]\n\nports = "))
	conf := File{
		f,
		"some/path",
	}

	ports, err := conf.GetStrArray("Main", "ports", ',')
	assert.Nil(err)
	assert.Equal([]string{}, ports)
}
