package quantizer

import (
	"encoding/json"
	"encoding/xml"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/DataDog/datadog-trace-agent/config"
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

func TestObfuscateJSON(t *testing.T) {
	suite, err := loadTests()
	if err != nil {
		t.Fatal(err)
	}
	if len(suite) == 0 {
		t.Fatal("no tests in suite")
	}
	for i, s := range suite {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			assert := assert.New(t)
			cfg := &config.JSONObfuscationConfig{KeepValues: s.KeepValues}
			out, err := newJSONObfuscator(cfg).obfuscate(s.In)
			assert.NoError(err)
			assert.Equal(s.Out, out)
		})
	}
}
