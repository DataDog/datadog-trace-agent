package main

import (
	"testing"

	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/stretchr/testify/assert"
)

func TestServiceMapper(t *testing.T) {
	assert := assert.New(t)

	mapper, in, out := testMapper()
	mapper.Start()
	defer mapper.Stop()

	// Let's ensure we have a proper context
	assert.Len(mapper.cache, 0)

	input := model.ServicesMetadata{"service-a": {"app_type": "type-a"}}
	in <- input
	output := <-out

	// When the service is ingested for the first time, we simply propagate it
	// to the output channel and add an entry to the cache map
	assert.Equal(input, output)
	assert.Len(mapper.cache, 1)

	// This entry will result in a cache-hit and therefore will be filtered out
	in <- model.ServicesMetadata{"service-a": {"app_type": "SOMETHING_DIFFERENT"}}

	// This represents a new service and thus will be cached and propagated to the outbound channel
	newService := model.ServicesMetadata{"service-b": {"app_type": "type-b"}}
	in <- newService
	output = <-out

	assert.Equal(newService, output)
	assert.Len(mapper.cache, 2)
}

func testMapper() (mapper *ServiceMapper, in, out chan model.ServicesMetadata) {
	in = make(chan model.ServicesMetadata, 1)
	out = make(chan model.ServicesMetadata, 1)
	mapper = NewServiceMapper(in, out)

	return mapper, in, out
}
