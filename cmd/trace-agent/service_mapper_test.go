package main

import (
	"testing"

	"github.com/DataDog/datadog-trace-agent/agent"
	"github.com/stretchr/testify/assert"
)

func TestServiceMapper(t *testing.T) {
	assert := assert.New(t)

	mapper, in, out := testMapper()
	mapper.Start()
	defer mapper.Stop()

	input := agent.ServicesMetadata{"service-a": {"app_type": "type-a"}}
	in <- input
	output := <-out

	// When the service is ingested for the first time, we simply propagate it
	// to the output channel and add an entry to the cache map
	assert.Equal(input, output)

	// This entry will result in a cache-hit and therefore will be filtered out
	in <- agent.ServicesMetadata{"service-a": {"app_type": "SOMETHING_DIFFERENT"}}

	// This represents a new service and thus will be cached and propagated to the outbound channel
	newService := agent.ServicesMetadata{"service-b": {"app_type": "type-b"}}
	in <- newService
	output = <-out

	assert.Equal(newService, output)
}

func TestCachePolicy(t *testing.T) {
	assert := assert.New(t)

	mapper, in, out := testMapper()
	mapper.Start()
	defer mapper.Stop()

	input := agent.ServicesMetadata{"service-a": {"app_type": "type-a"}}
	in <- input
	output := <-out

	// A new service entry should propagate the metadata the the outbound channel
	assert.Equal(input, output)

	// A service entry that is already in cache should only be propagated IF:
	// - Current version does NOT have "app"
	// - New version DOES have "app"

	// This first attempt won't be propagated to the writer
	firstAttempt := agent.ServicesMetadata{"service-a": {"app_type": "FIRST_ATTEMPT"}}
	in <- firstAttempt

	// But this second will
	secondAttempt := agent.ServicesMetadata{"service-a": {"app_type": "SECOND_ATTEMPT", "app": "app-a"}}
	in <- secondAttempt

	output = <-out
	assert.Equal(secondAttempt, output)
}

func testMapper() (mapper *ServiceMapper, in, out chan agent.ServicesMetadata) {
	in = make(chan agent.ServicesMetadata, 1)
	out = make(chan agent.ServicesMetadata, 1)
	mapper = NewServiceMapper(in, out)

	return mapper, in, out
}
