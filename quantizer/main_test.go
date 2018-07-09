package quantizer

import (
	"flag"
	"log"
	"os"
	"testing"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/cihub/seelog"
	"github.com/stretchr/testify/assert"
)

type compactSpacesTestCase struct {
	before string
	after  string
}

func TestMain(m *testing.M) {
	flag.Parse()

	// disable loggging in tests
	seelog.UseLogger(seelog.Disabled)

	// prepare JSON obfuscator tests
	suite, err := loadTests()
	if err != nil {
		log.Fatal(err)
	}
	if len(suite) == 0 {
		log.Fatal("no tests in suite")
	}
	jsonSuite = suite

	os.Exit(m.Run())
}

func TestNewObfuscator(t *testing.T) {
	assert := assert.New(t)
	o := NewObfuscator(nil)
	assert.Nil(o.es)
	assert.Nil(o.mongo)

	o = NewObfuscator(&config.ObfuscationConfig{
		ES:    config.JSONObfuscationConfig{},
		Mongo: config.JSONObfuscationConfig{},
	})
	assert.Nil(o.es)
	assert.Nil(o.mongo)

	o = NewObfuscator(&config.ObfuscationConfig{
		ES:    config.JSONObfuscationConfig{Enabled: true},
		Mongo: config.JSONObfuscationConfig{Enabled: true},
	})
	assert.NotNil(o.es)
	assert.NotNil(o.mongo)
}

func TestCompactWhitespaces(t *testing.T) {
	assert := assert.New(t)

	resultsToExpect := []compactSpacesTestCase{
		{"aa",
			"aa"},

		{" aa bb",
			"aa bb"},

		{"aa    bb  cc  dd ",
			"aa bb cc dd"},
	}

	for _, testCase := range resultsToExpect {
		assert.Equal(testCase.after, compactWhitespaces(testCase.before))
	}
}
