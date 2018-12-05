package sampler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestByServiceKey(t *testing.T) {
	assert := assert.New(t)

	assert.Equal(defaultServiceRateKey, byServiceKey("", ""))
	assert.Equal("service:mcnulty,env:test", byServiceKey("mcnulty", "test"))
}

func TestNewServiceKeyCatalog(t *testing.T) {
	assert := assert.New(t)

	cat := newServiceKeyCatalog()
	assert.NotNil(cat)
	assert.Equal(map[string]Signature{}, map[string]Signature(cat))
}

func TestServiceKeyCatalogRegister(t *testing.T) {
	assert := assert.New(t)

	cat := newServiceKeyCatalog()
	s := getTestPriorityEngine()

	_, root1 := getTestTraceWithService(t, "service1", s)
	sig1 := ServiceSignature{root1.Service, defaultEnv}.Hash()
	cat.register(root1, defaultEnv, sig1)
	assert.Equal(map[string]Signature{"service:service1,env:none": sig1}, map[string]Signature(cat))

	_, root2 := getTestTraceWithService(t, "service2", s)
	sig2 := ServiceSignature{root2.Service, defaultEnv}.Hash()
	cat.register(root2, defaultEnv, sig2)
	assert.Equal(map[string]Signature{
		"service:service1,env:none": sig1,
		"service:service2,env:none": sig2,
	}, map[string]Signature(cat))
}

func TestServiceKeyCatalogGetRateByService(t *testing.T) {
	assert := assert.New(t)

	cat := newServiceKeyCatalog()
	s := getTestPriorityEngine()

	_, root1 := getTestTraceWithService(t, "service1", s)
	sig1 := ServiceSignature{root1.Service, defaultEnv}.Hash()
	cat.register(root1, defaultEnv, sig1)
	_, root2 := getTestTraceWithService(t, "service2", s)
	sig2 := ServiceSignature{root2.Service, defaultEnv}.Hash()
	cat.register(root2, defaultEnv, sig2)

	rates := map[Signature]float64{
		sig1: 0.3,
		sig2: 0.7,
	}
	const totalRate = 0.2

	var rateByService map[string]float64

	rateByService = cat.getRateByService(rates, totalRate)
	assert.Equal(map[string]float64{
		"service:service1,env:none": 0.3,
		"service:service2,env:none": 0.7,
		"service:,env:":             0.2,
	}, rateByService)

	delete(rates, sig1)

	rateByService = cat.getRateByService(rates, totalRate)
	assert.Equal(map[string]float64{
		"service:service2,env:none": 0.7,
		"service:,env:":             0.2,
	}, rateByService)

	delete(rates, sig2)

	rateByService = cat.getRateByService(rates, totalRate)
	assert.Equal(map[string]float64{
		"service:,env:": 0.2,
	}, rateByService)
}
