package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DataDog/raclette/model"
)

type ResourceGenerator func() string
type DurationGenerator func() float64

type Service struct {
	Name          string
	SubServices   []Service
	ResourceMaker ResourceGenerator
	DurationMaker DurationGenerator
}

func ChooseRandomString(choices []string) string {
	idx := rand.Intn(len(choices))
	return choices[idx]
}

func GaussianDuration(mean float64, stdDev float64, leftCutoff float64, rightCutoff float64) float64 {
	sample := rand.NormFloat64()*stdDev + mean
	if leftCutoff != 0 && sample < leftCutoff {
		return leftCutoff
	}
	if rightCutoff != 0 && sample > rightCutoff {
		return rightCutoff
	}

	// a duration can never be negative
	return math.Max(sample, 0)
}

// generateTrace generate a trace for the given Service
// You can also pass some more parameters if you want to generate a trace
// in a given context (e.g. generate nested traces)
// * traceId will be used if != 0 or else generated
// * parentId will be used if != 0  or else generated
// * traces is a pointer to a slice of Spans where the trace we generate will be appended
// * minTs/maxTs are float64 timestamps, if != 0 they will be used as time boundaries for generated traces
//   this is something useful when you want to generatet "nested" traces
func generateTrace(s Service, traceId model.TID, parentId model.SID, traces *[]model.Span, minTs float64, maxTs float64) float64 {
	t := model.Span{
		TraceID:  traceId,
		SpanID:   model.NewSID(),
		ParentID: parentId,
		Service:  s.Name,
		Resource: s.ResourceMaker(),
		Type:     "custom",
		Duration: s.DurationMaker(),
	}
	t.Normalize()

	t.Start = math.Max(minTs, t.Start)
	if maxTs != 0 && t.Start+t.Duration > maxTs {
		t.Duration = maxTs - t.Start
	}

	//log.Printf("service %s, resource %s, duration %f, start %f, traceid %d, parentid %d, trace len %d, minTs %f, maxTs %f",
	//	s.Name, t.Resource, t.Duration, t.Start, traceId, parentId, len(*traces), minTs, maxTs)

	// for the next trace to start after this one
	maxGeneratedTs := t.Start + t.Duration
	if minTs == 0 {
		// except if this trace is the parent trace then the next one is at
		// start + some jitter
		maxGeneratedTs = t.Start
	}

	//log.Printf("maxgents %f", maxGeneratedTs)

	*traces = append(*traces, t)

	// replace that for subservices generation
	if maxTs == 0 {
		maxTs = t.Start + t.Duration
	}
	for _, subs := range s.SubServices {
		// subservices use the maxgen timestamp from last generation to keep them sequential in the timeline
		// ------
		//   s1
		//        -----------
		//			   s2
		genTs := generateTrace(subs, t.TraceID, t.SpanID, traces, maxGeneratedTs, maxTs)
		maxGeneratedTs = math.Max(maxGeneratedTs, genTs)
	}

	return maxGeneratedTs
}

func handleSignal(exit chan bool) {
	sigChan := make(chan os.Signal, 10)
	signal.Notify(sigChan)
	for signal := range sigChan {
		switch signal {
		case syscall.SIGINT, syscall.SIGTERM:
			log.Println("Terminating")
			close(exit)
		}
	}
}

func main() {
	var tpm = flag.Int("tpm", 1000, "nb of traces per minute generated")
	flag.Parse()
	// seed the generator
	rand.Seed(time.Now().Unix())

	s := NewFakeSobotka()

	exit := make(chan bool)
	go handleSignal(exit)

	tps := *tpm / 60
	tickChan := time.NewTicker(time.Second).C
	timerChan := time.NewTimer(60 * time.Second).C

	genTraces := 0

	for {
		select {
		case <-timerChan:
			log.Printf("generated %d traces this minute", genTraces)
			return
		case <-tickChan:
			log.Printf("Generate %d Traces this second\n", tps)
			assigned := 0
			for assigned < tps {
				batchSize := rand.Intn(tps)
				if batchSize > tps-assigned || batchSize == 0 {
					continue
				}
				go func() {
					//log.Printf("Batch of %d traces\n", batchSize)
					// avg depth level ~ 20
					var traces []model.Span
					for i := 0; i < batchSize; i++ {
						generateTrace(s, 0, 0, &traces, 0, 0)
						genTraces++
					}

					body, err := json.Marshal(traces)
					//log.Println(string(body))
					if err != nil {
						panic(err)
					}
					resp, err := http.Post("http://localhost:7777/spans", "application/json", bytes.NewBuffer(body))
					if err != nil {
						panic(err)
					}

					log.Println(resp.Status)
				}()
				assigned += batchSize
			}
		case <-exit:
			return
		}
	}
}
