package profile

import (
	"bytes"
	"encoding/json"
	"github.com/DataDog/datadog-trace-agent/fixtures"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDumpTraces(t *testing.T) {
	assert := assert.New(t)
	traces := []model.Trace{fixtures.RandomTrace(3, 5), fixtures.RandomTrace(5, 7)}

	var buf bytes.Buffer

	d := NewTracesDump(&buf)
	d.Dump(traces)
	s := buf.String()
	t.Logf("traces dump: %s", buf.String())
	assert.NotEqual(0, len(s))
	if len(s) > 0 {
		assert.Equal("\n", s[len(s)-1:len(s)], "line should end with NewLine")
		assert.NotContains(s[0:len(s)-1], "\n", "JSON should not contain any new line")
	}

	var checkBuf bytes.Buffer
	var checkTraces []model.Trace

	_, err := checkBuf.Write([]byte(s[0 : len(s)-1])) // strip '\n'
	assert.Nil(err)
	dec := json.NewDecoder(&checkBuf)
	err = dec.Decode(&checkTraces)
	assert.Nil(err, "unable to decode traces")
	assert.Equal(2, len(checkTraces))
	assert.Equal(traces, checkTraces)
}
