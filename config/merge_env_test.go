package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnalyzedSpansEnvConfigParsing(t *testing.T) {
	assert := assert.New(t)

	// Check valid cases

	a, err := readAnalyzedSpanEnvVariable("service|operation=1")
	assert.Nil(err)
	assert.Len(a, 1)
	assert.Len(a["service"], 1)
	assert.Equal(a["service"]["operation"], float64(1))

	a = nil
	a, err = readAnalyzedSpanEnvVariable("service|operation=0.01")
	assert.Nil(err)
	assert.Len(a, 1)
	assert.Len(a["service"], 1)
	assert.Equal(a["service"]["operation"], 0.01)

	a = nil
	a, err = readAnalyzedSpanEnvVariable("service|operation=1,service2|operation2=1")
	assert.Nil(err)
	assert.Len(a, 2)
	assert.Len(a["service"], 1)
	assert.Equal(a["service"]["operation"], float64(1))
	assert.Equal(a["service2"]["operation2"], float64(1))

	a, err = readAnalyzedSpanEnvVariable("")
	assert.Nil(err)
	assert.Len(a, 0)

	// Check errors

	a, err = readAnalyzedSpanEnvVariable("service|operation=")
	assert.NotNil(err)

	a, err = readAnalyzedSpanEnvVariable("serviceoperation=1")
	assert.NotNil(err)

	a, err = readAnalyzedSpanEnvVariable("service|operation=1,")
	assert.NotNil(err)
}
