package obfuscate

import (
	"flag"
	"log"
	"os"
	"testing"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/model"
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

		{"    ",
			""},

		{"a b       cde     fg       hi                     j  jk   lk lkjfdsalfd     afsd sfdafsd f",
			"a b cde fg hi j jk lk lkjfdsalfd afsd sfdafsd f"},

		{"   ¡™£¢∞§¶    •ªº–≠œ∑´®†¥¨ˆøπ “‘«åß∂ƒ©˙∆˚¬…æΩ≈ç√ ∫˜µ≤≥÷    ",
			"¡™£¢∞§¶ •ªº–≠œ∑´®†¥¨ˆøπ “‘«åß∂ƒ©˙∆˚¬…æΩ≈ç√ ∫˜µ≤≥÷"},
	}

	for _, testCase := range resultsToExpect {
		assert.Equal(testCase.after, compactWhitespaces(testCase.before))
	}
}

func TestObfuscateConfig(t *testing.T) {
	// testConfig returns a test function which creates a span of type typ,
	// having a tag with key/val, runs the obfuscator on it using the given
	// configuration and asserts that the new tag value matches exp.
	testConfig := func(
		typ, key, val, exp string,
		cfg *config.ObfuscationConfig,
	) func(*testing.T) {
		return func(t *testing.T) {
			span := &model.Span{Type: typ, Meta: map[string]string{key: val}}
			NewObfuscator(cfg).Obfuscate(span)
			assert.Equal(t, exp, span.Meta[key])
		}
	}

	t.Run("redis/enabled", testConfig(
		"redis",
		"redis.raw_command",
		"SET key val",
		"SET key ?",
		&config.ObfuscationConfig{
			Redis: config.Enablable{Enabled: true},
		},
	))

	t.Run("redis/disabled", testConfig(
		"redis",
		"redis.raw_command",
		"SET key val",
		"SET key val",
		&config.ObfuscationConfig{},
	))

	t.Run("http/enabled", testConfig(
		"http",
		"http.url",
		"http://mysite.mydomain/1/2?q=asd",
		"http://mysite.mydomain/?/??",
		&config.ObfuscationConfig{
			HTTP: config.HTTPObfuscationConfig{
				RemovePathDigits:  true,
				RemoveQueryString: true,
			},
		},
	))

	t.Run("http/disabled", testConfig(
		"http",
		"http.url",
		"http://mysite.mydomain/1/2?q=asd",
		"http://mysite.mydomain/1/2?q=asd",
		&config.ObfuscationConfig{},
	))

	t.Run("json/enabled", testConfig(
		"elasticsearch",
		"elasticsearch.body",
		`{"role": "database"}`,
		`{"role":"?"}`,
		&config.ObfuscationConfig{
			ES: config.JSONObfuscationConfig{Enabled: true},
		},
	))

	t.Run("json/disabled", testConfig(
		"elasticsearch",
		"elasticsearch.body",
		`{"role": "database"}`,
		`{"role": "database"}`,
		&config.ObfuscationConfig{},
	))
}

func BenchmarkCompactWhitespaces(b *testing.B) {
	str := "a b       cde     fg       hi                     j  jk   lk lkjfdsalfd     afsd sfdafsd f"
	for i := 0; i < b.N; i++ {
		compactWhitespaces(str)
	}
}
