package quantizer

import (
	"encoding/json"
	"encoding/xml"
	"flag"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/cihub/seelog"
	"github.com/stretchr/testify/assert"
)

// obfuscateTestFile contains all the tests for JSON obfuscation
const obfuscateTestFile = "./testdata/json_tests.xml"

type xmlObfuscateTests struct {
	XMLName xml.Name            `xml:"ObfuscateTests,-"`
	Tests   []*xmlObfuscateTest `xml:"TestSuite>Test"`
}

type xmlObfuscateTest struct {
	Tag        string
	In         string
	Out        string
	KeepValues []string `xml:"KeepValues>key"`
}

// loadTests loads all XML tests from ./testdata/obfuscate.xml
func loadTests() ([]*xmlObfuscateTest, error) {
	path, err := filepath.Abs(obfuscateTestFile)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	defer f.Close()
	if err != nil {
		return nil, err
	}
	var suite xmlObfuscateTests
	if err := xml.NewDecoder(f).Decode(&suite); err != nil {
		return nil, err
	}
	for _, test := range suite.Tests {
		// normalize JSON output
		test.Out = normalize(test.Out)
		test.In = normalize(test.In)
	}
	return suite.Tests, err
}

func normalize(in string) string {
	var tmp map[string]interface{}
	if err := json.Unmarshal([]byte(in), &tmp); err != nil {
		log.Fatal(err)
	}
	out, err := json.Marshal(tmp)
	if err != nil {
		log.Fatal(err)
	}
	return string(out)
}

var jsonSuite []*xmlObfuscateTest

func TestMain(m *testing.M) {
	flag.Parse()
	// Disable debug logs in these tests
	seelog.UseLogger(seelog.Disabled)

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

func TestObfuscateJSON(t *testing.T) {
	for i, s := range jsonSuite {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			assert := assert.New(t)
			cfg := &config.JSONObfuscationConfig{KeepValues: s.KeepValues}
			out, err := newJSONObfuscator(cfg).obfuscate(s.In)
			assert.NoError(err)
			assert.Equal(s.Out, out)
		})
	}
}

func BenchmarkObfuscateJSON(b *testing.B) {
	cfg := &config.JSONObfuscationConfig{KeepValues: []string{"highlight"}}
	if len(jsonSuite) == 0 {
		b.Fatal("no test suite loaded")
	}
	var ran int
	for i := len(jsonSuite) - 1; i >= 0; i-- {
		ran++
		if ran > 3 {
			// run max 3 benchmarks
			break
		}
		test := jsonSuite[i]
		b.Run(strconv.Itoa(len(test.In)), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := newJSONObfuscator(cfg).obfuscate(test.In); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
