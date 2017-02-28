package sampler

// [TODO:christian] publish all through expvar, but wait until the PR
// with cpu watchdog is merged as there are probably going to be git conflicts...

import (
	"net/http"
	"strconv"
	"sync"
)

const (
	// TraceCountHeader is the header client implementation should fill
	// with the number of traces contained in the payload.
	TraceCountHeader = "X-Datadog-Trace-Count"

	// Every 100 payload, reset counters. This means that we can not
	// go below 1% pre-sampling, as the first payload will always be
	// accepted, since by default the RealRate is the Rate when there
	// is no data.
	preSamplerResetPayloads = 100
)

// PreSamplerStats contains pre-sampler data. The public content
// might be interesting for statistics, logging.
type PreSamplerStats struct {
	// Rate is the target pre-sampling rate.
	Rate float64
	// PayloadsSeen is the number of payloads that passed by.
	PayloadsSeen int64
	// TracesSeen is the number of traces that passed by.
	TracesSeen int64
	// TracesDropped is the number of traces that were dropped.
	TracesDropped int64
}

// PreSampler tries to tell wether we should keep a payload, even
// before fully processing it. Its only clues are the unparsed payload
// and the HTTP headers. It should remain very light and fast.
type PreSampler struct {
	stats  PreSamplerStats
	logger Logger
	mu     sync.RWMutex // needed since many requests can run in parallel
}

// Logger is an interface used internally in the agent receiver.
type Logger interface {
	// Errorf formats the string and logs.
	Errorf(format string, params ...interface{})
}

// NewPreSampler returns an initialized presampler
func NewPreSampler(rate float64, logger Logger) *PreSampler {
	return &PreSampler{
		stats: PreSamplerStats{
			Rate: rate,
		},
		logger: logger,
	}
}

// SetRate set the pre-sample rate, thread-safe.
func (ps *PreSampler) SetRate(rate float64) {
	ps.mu.Lock()
	ps.stats.Rate = rate
	ps.mu.Unlock()
}

// TargetRate returns the current target pre-sample rate, thread-safe.
// The target pre-sample rate is the value set with SetRate, ideally this
// is the sample rate, but depending on what is received, the real rate
// might defer.
func (ps *PreSampler) Rate() float64 {
	ps.mu.RLock()
	rate := ps.stats.Rate
	ps.mu.RUnlock()
	return rate
}

// RealRate returns the current real pre-sample rate, thread-safe.
// This is the value obtained by counting what was kept and dropped.
func (ps *PreSampler) RealRate() float64 {
	ps.mu.RLock()
	rate := ps.stats.RealRate()
	ps.mu.RUnlock()
	return rate
}

// RealRate calcuates the current real pre-sample rate from
// the stats data. If no data is available, returns the target rate.
func (stats *PreSamplerStats) RealRate() float64 {
	if stats.TracesSeen <= 0 { // careful with div by 0
		return stats.Rate
	}
	return 1 - (float64(stats.TracesDropped) / float64(stats.TracesSeen))
}

func (ps *PreSampler) Stats() *PreSamplerStats {
	ps.mu.RLock()
	stats := ps.stats
	ps.mu.RUnlock()
	return &stats
}

// Sample tells wether a given request should be kept (true means: "yes, keep it").
// Calling this alters the statistics, it affects the result of RealRate() so
// only call it once per payload.
func (ps *PreSampler) Sample(req *http.Request) bool {
	traceCount := int64(0)
	if traceCountStr := req.Header.Get(TraceCountHeader); traceCountStr != "" {
		var err error
		traceCount, err = strconv.ParseInt(traceCountStr, 10, 64)
		if err != nil {
			ps.logger.Errorf("unable to parse HTTP header %s: %s", TraceCountHeader, traceCountStr)
		}
	}

	if traceCount <= 0 {
		return true // no sensible value in traceCount, disable pre-sampling
	}

	keep := true

	ps.mu.Lock()

	if ps.stats.PayloadsSeen >= preSamplerResetPayloads {
		// Reset the stats so that we compute a new rate for the next round.
		ps.stats.PayloadsSeen = 0
		ps.stats.TracesSeen = 0
		ps.stats.TracesDropped = 0
	}

	if ps.stats.RealRate() > ps.stats.Rate {
		// Too many things processed, drop the current payload.
		keep = false
		ps.stats.TracesDropped += traceCount
	}

	// This should be done *after* testing RealRate() against Rate,
	// else we could end up systematically dropping the first payload.
	ps.stats.PayloadsSeen++
	ps.stats.TracesSeen += traceCount

	ps.mu.Unlock()

	if !keep {
		ps.logger.Errorf("pre-sampling at rate %f dropped payload with %d traces", traceCount)
	}

	return keep
}
