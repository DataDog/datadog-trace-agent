// This is mostly for debugging, to dump all input to a file and keep a trace
// of it, typically to replay it for profiling.

package profile

import (
	"encoding/json"
	"github.com/DataDog/datadog-trace-agent/model"
	"io"
	"sync"
)

// ServicesDumper is a generic interface to dump services for profiling/debugging
type ServicesDumper interface {
	// Dump writes services metadata to a log file
	Dump(trace model.ServicesMetadata) error
}

type ServicesDump struct {
	m      sync.Mutex
	writer io.Writer
}

// NewTracesDump creates a ServicesDumper which writes data to a standard writer
func NewServicesDump(writer io.Writer) ServicesDumper {
	return &ServicesDump{writer: writer}
}

// Dump writes services metadata to a log file
func (td *ServicesDump) Dump(services model.ServicesMetadata) error {
	td.m.Lock()
	defer td.m.Unlock()

	enc := json.NewEncoder(td.writer)
	err := enc.Encode(services)
	return err
}
