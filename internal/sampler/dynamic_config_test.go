package sampler

import (
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDynamicConfig(t *testing.T) {
	assert := assert.New(t)

	dc := NewDynamicConfig()
	assert.NotNil(dc)

	rates := map[ServiceSignature]float64{
		ServiceSignature{"myservice", "myenv"}: 0.5,
	}

	// Not doing a complete test of the different components of dynamic config,
	// but still assessing it can do the bare minimum once initialized.
	dc.RateByService.SetAll(rates)
	rbs := dc.RateByService.GetAll()
	assert.Equal(map[string]float64{"service:myservice,env:myenv": 0.5}, rbs)
}

func TestRateByServiceGetSet(t *testing.T) {
	var rbc RateByService
	for i, tc := range []struct {
		in  map[ServiceSignature]float64
		out map[string]float64
	}{
		{
			in: map[ServiceSignature]float64{
				ServiceSignature{}: 0.1,
			},
			out: map[string]float64{
				"service:,env:": 0.1,
			},
		},
		{
			in: map[ServiceSignature]float64{
				ServiceSignature{}:                  0.3,
				ServiceSignature{"mcnulty", "dev"}:  0.2,
				ServiceSignature{"postgres", "dev"}: 0.1,
			},
			out: map[string]float64{
				"service:,env:":            0.3,
				"service:mcnulty,env:dev":  0.2,
				"service:postgres,env:dev": 0.1,
			},
		},
		{
			in: map[ServiceSignature]float64{
				ServiceSignature{}: 1,
			},
			out: map[string]float64{
				"service:,env:": 1,
			},
		},
		{
			out: map[string]float64{},
		},
		{
			in: map[ServiceSignature]float64{
				ServiceSignature{}: 0.2,
			},
			out: map[string]float64{
				"service:,env:": 0.2,
			},
		},
	} {
		rbc.SetAll(tc.in)
		assert.Equal(t, tc.out, rbc.GetAll(), strconv.Itoa(i))
	}
}

func TestRateByServiceLimits(t *testing.T) {
	assert := assert.New(t)

	var rbc RateByService
	rbc.SetAll(map[ServiceSignature]float64{
		ServiceSignature{"high", ""}: 2,
		ServiceSignature{"low", ""}:  -1,
	})
	assert.Equal(map[string]float64{"service:high,env:": 1, "service:low,env:": 0}, rbc.GetAll())
}

func TestRateByServiceConcurrency(t *testing.T) {
	assert := assert.New(t)

	var rbc RateByService

	const n = 1000
	var wg sync.WaitGroup
	wg.Add(2)

	rbc.SetAll(map[ServiceSignature]float64{ServiceSignature{"mcnulty", "test"}: 1})
	go func() {
		for i := 0; i < n; i++ {
			rate := float64(i) / float64(n)
			rbc.SetAll(map[ServiceSignature]float64{ServiceSignature{"mcnulty", "test"}: rate})
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
