package config

import (
	"sync"
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

func TestRateByServiceGetSet(t *testing.T) {
	assert := assert.New(t)

	var rbc RateByService
	testCases := []map[string]float64{
		{"service:,env:": 0.1},
		{"service:,env:": 0.3, "service:mcnulty,env:dev": 0.7, "service:postgres,env:dev": 0.2},
		{"service:,env:": 1},
		{},
		{"service:,env:": 0.2},
	}

	for _, tc := range testCases {
		rbc.SetAll(tc)
		assert.Equal(tc, rbc.GetAll())
	}
}

func TestRateByServiceLimits(t *testing.T) {
	assert := assert.New(t)

	var rbc RateByService
	rbc.SetAll(map[string]float64{"service:high,env:": 2, "service:low,env:": -1})
	assert.Equal(map[string]float64{"service:high,env:": 1, "service:low,env:": 0}, rbc.GetAll())
}

func TestRateByServiceConcurrency(t *testing.T) {
	assert := assert.New(t)

	var rbc RateByService

	const n = 1000
	var wg sync.WaitGroup
	wg.Add(2)

	rbc.SetAll(map[string]float64{"service:mcnulty,env:test": 1})
	go func() {
		for i := 0; i < n; i++ {
			rate := float64(i) / float64(n)
			rbc.SetAll(map[string]float64{"service:mcnulty,env:test": rate})
		}
		wg.Done()
	}()
	go func() {
		for i := 0; i < n; i++ {
			rates := rbc.GetAll()
			_, ok := rates["service:mcnulty,env:test"]
			assert.True(ok, "key should be here")
		}
		wg.Done()
	}()
}
