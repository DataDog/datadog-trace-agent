package quantizer

import (
	"encoding/json"
	"encoding/xml"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/stretchr/testify/assert"
)

type xmlObfuscateTest struct {
	XMLName    xml.Name `xml:"ObfuscateTest,-"`
	Tag        string
	In         string
	Out        string
	KeepValues []string `xml:"KeepValues>value"`
	Filename   string   `xml:"-"`
}

// loadTests returns all tests found in the XML files at ./testdata
func loadTests() ([]*xmlObfuscateTest, error) {
	dir, err := filepath.Abs("./testdata")
	if err != nil {
		return nil, err
	}
	tests := make([]*xmlObfuscateTest, 0)
	err = filepath.Walk(dir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return nil
		}
		if ext := filepath.Ext(fi.Name()); ext != ".xml" {
			return nil
		}
		f, err := os.Open(filepath.Join(dir, fi.Name()))
		if err != nil {
			return err
		}
		var test xmlObfuscateTest
		if err := xml.NewDecoder(f).Decode(&test); err != nil {
			return err
		}
		test.Filename = fi.Name()

		// normalize JSON output
		test.Out = normalize(test.Out)
		test.In = normalize(test.In)

		tests = append(tests, &test)
		return nil
	})
	return tests, err
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
	for _, s := range suite {
		t.Run(s.Filename, func(t *testing.T) {
			assert := assert.New(t)
			cfg := &config.JSONObfuscationConfig{KeepValues: s.KeepValues}
			out, err := newJSONObfuscator(cfg).obfuscate(s.In)
			assert.NoError(err)
			assert.Equal(s.Out, out)
		})
	}
}
