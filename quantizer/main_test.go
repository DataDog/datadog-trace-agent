package quantizer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type compactSpacesTestCase struct {
	before string
	after  string
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

func TestCompactAllSpaces(t *testing.T) {
	assert := assert.New(t)

	resultsToExpect := []compactSpacesTestCase{
		{"aa",
			"aa"},

		{"aa bb \n   ",
			"aa bb"},

		{"	aa 	bb	cc\n ",
			"aa bb cc"},

		{"aa \n  \n bb\ncc 		 dd\n",
			"aa bb cc dd"},

		{"\n ¡™£¢∞§¶ \n •ªº–≠œ∑´®†¥¨ˆøπ  “‘«åß∂ƒ©˙∆˚¬…æΩ≈ç√	∫˜µ≤≥÷",
			"¡™£¢∞§¶ •ªº–≠œ∑´®†¥¨ˆøπ “‘«åß∂ƒ©˙∆˚¬…æΩ≈ç√ ∫˜µ≤≥÷"},
	}

	for _, testCase := range resultsToExpect {
		assert.Equal(testCase.after, compactAllSpaces(testCase.before))
	}
}

func BenchmarkCompactAllSpacesWithRegexp(b *testing.B) {
	for n := 0; n < b.N; n++ {
		compactAllSpaces("SELECT org_id,metric_key \n		FROM metrics_metadata \n		WHERE org_id = %(org_id)s 	AND 	metric_key = ANY(array[21, 25, 32])")
	}
}
