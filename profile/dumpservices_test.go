package profile

import (
	"bytes"
	"encoding/json"
	"github.com/DataDog/datadog-trace-agent/fixtures"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDumpServices(t *testing.T) {
	assert := assert.New(t)
	services := fixtures.RandomServices(10, 5)

	var buf bytes.Buffer

	d := NewServicesDump(&buf)
	d.Dump(services)
	s := buf.String()
	t.Logf("services dump: %s", buf.String())
	assert.NotEqual(0, len(s))
	if len(s) > 0 {
		assert.Equal("\n", s[len(s)-1:len(s)], "line should end with NewLine")
		assert.NotContains(s[0:len(s)-1], "\n", "JSON should not contain any new line")
	}

	var checkBuf bytes.Buffer
	var checkServices model.ServicesMetadata

	_, err := checkBuf.Write([]byte(s[0 : len(s)-1])) // strip '\n'
	assert.Nil(err)
	dec := json.NewDecoder(&checkBuf)
	err = dec.Decode(&checkServices)
	assert.Nil(err, "unable to decode services")
	assert.NotEqual(0, len(services))
	assert.Equal(services, checkServices)
}
