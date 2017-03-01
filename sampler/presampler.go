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

// Rate returns the current target pre-sample rate, thread-safe.
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

// Stats returns a copy of the currrent pre-sampler stats.
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
		ps.logger.Errorf("pre-sampling at rate %f dropped payload with %d traces", ps.Rate(), traceCount) // [FIXME:christian] this is not an error...
	}

	return keep
}

// CalcPreSampleRate gives the new sample rate to apply for a given max user CPU average.
// It takes the current sample rate and user CPU average as those parameters both
// have an influence on the result.
func CalcPreSampleRate(maxUserAvg, currentUserAvg, currentRate float64) float64 {
	const (
		// userAvg0 is the CPU usage we can possibly reach with sampling at 0%.
		// Of course this really depends on the data received, the context,
		// the machine running the code etc. But, there's no point in targetting 0.
		// Benchmarks show 2% should always be, reasonnably, reachable.
		userAvg0 = float64(0.02) // 2% CPU usage
		// userAvgMin is a limit that maxUserAvg should respect, because trying
		// to remain below this through pre-sampling can do more harm than good,
		// trying to drop everything and still not reaching the goal.
		userAvgMin = float64(0.04) // 4% CPU usage
		// deltaMin is a threshold that must be passed before changing the
		// pre-sampling rate. If set to 0.3, for example, the new rate must be
		// either over 130% or below 70% of the previous value, before we actually
		// adjust the sampling rate. This is to avoid over-adapting and jittering.
		deltaMin = float64(0.3) // +/- 30% change
		// rateMin is an absolute minimum rate, never sample more than this, it is
		// inefficient, the cost handling the payloads without even reading them
		// is too high anyway.
		rateMin = float64(0.05) // 5% hard-limit
	)

	if maxUserAvg <= userAvg0 || currentUserAvg <= 0 || currentRate <= 0 || currentRate > 1 {
		return 1 // inconsistent input data, in doubt, disable the feature
	}

	if maxUserAvg < userAvgMin {
		maxUserAvg = userAvgMin
	}

	newRate := float64(1)
	slope := (currentUserAvg - userAvg0) / currentRate
	if slope <= 0 {
		// OK, here, slope is:
		// - zero -> no matter how we presample, CPU will remain the same
		// - negative -> would mean the more we sample, the more CPU we consume...
		// So in short, there's no need to sample (in practice, it means
		// we're below userAvg0, which is our base CPU usage).
		return 1
	}

	newRate = (maxUserAvg - userAvg0) / slope
	if newRate <= 0 || newRate >= 1 {
		return 1 // again, in doubt, disable pre-sampling
	}

	delta := 2 * (newRate - currentRate) / (newRate + currentRate)
	if delta > -deltaMin && delta < 0 {
		return currentRate
	}
	if newRate < rateMin {
		return rateMin
	}

	return newRate
}
