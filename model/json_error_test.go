package model

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func testHumanReadableJSONError(s []byte, x interface{}) (error, string) {
	r := bytes.NewReader(s)
	dec := json.NewDecoder(r)
	err := dec.Decode(&x)
	prettyerr := HumanReadableJSONError(r, err)
	return err, prettyerr
}

func TestJSONSyntaxError(t *testing.T) {
	assert := assert.New(t)
	var x interface{}

	s := []byte(`{"test": "this is a JSON string", "next": "that has a syntax error",,}`)
	err, prettyerr := testHumanReadableJSONError(s, x)

	assert.NotNil(err)
	exp := `json syntax error at offset:69
error located at marker ---^:
     JSON string", "next": "that has a syntax error",,}
                                                  ---^`
	assert.Equal(exp, prettyerr, "expected:\n%s\ngot:\n%s\n", exp, string(prettyerr))
}

func TestJSONWrongInterfaceType(t *testing.T) {
	assert := assert.New(t)
	var x map[string]int

	s := []byte(`{"apples": 2, "cheese": 42, "raclette": "a lot", "baguette": 12}`)
	err, prettyerr := testHumanReadableJSONError(s, &x)

	assert.NotNil(err)
	exp := `was expecting type int and got type string at offset:47
error located at marker ---^:
    {"apples": 2, "cheese": 42, "raclette": "a lot", "baguette": 12}
                                               ---^`
	assert.Equal(exp, prettyerr, "expected:\n%s\ngot:\n%s\n", exp, string(prettyerr))
}
