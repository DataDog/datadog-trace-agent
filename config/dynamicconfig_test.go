package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDynamicConfig(t *testing.T) {
	assert := assert.New(t)

	dc := NewDynamicConfig()
	assert.NotNil(dc)

	rates := map[string]float64{"service:myservice,env:myenv": 0.5}

	// Not doing a complete test of the different components of dynamic config,
	// but still assessing it can do the bare minimum once initialized.
	dc.RateByService.SetAll(rates)
	rbs := dc.RateByService.GetAll()
	assert.Equal(rates, rbs)
}
