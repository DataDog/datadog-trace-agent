package main

import (
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/profile"
	log "github.com/cihub/seelog"
	"os"
)

var globalTracesDumpFile *os.File
var globalServicesDumpFile *os.File
var globalTracesDumper profile.TracesDumper
var globalServicesDumper profile.ServicesDumper

func dumpInit(dumpTraces, dumpServices string) error {
	if dumpTraces != "" {
		globalTracesDumpFile, err := os.Create(dumpTraces)
		if err != nil {
			return err
		}
		globalTracesDumper = profile.NewTracesDump(globalTracesDumpFile)
		log.Infof("Dumping traces in '%s'", dumpTraces)
	}

	if dumpServices != "" {
		globalServicesDumpFile, err := os.Create(dumpServices)
		if err != nil {
			return err
		}
		globalServicesDumper = profile.NewServicesDump(globalServicesDumpFile)
		log.Infof("Dumping services in '%s'", dumpServices)
	}

	return nil
}

func dumpTraces(traces []model.Trace) {
	if globalTracesDumper != nil {
		err := globalTracesDumper.Dump(traces)
		if err != nil {
			log.Debugf("unable to dump %d traces: %v", len(traces), err)
		}
	}
}

func dumpServices(services model.ServicesMetadata) {
	if globalServicesDumper != nil {
		err := globalServicesDumper.Dump(services)
		if err != nil {
			log.Debugf("unable to dump %d services: %v", len(services), err)
		}
	}
}

func dumpClose() {
	if globalServicesDumpFile != nil {
		globalServicesDumpFile.Close()
	}
	if globalServicesDumpFile != nil {
		globalServicesDumpFile.Close()
	}
}
