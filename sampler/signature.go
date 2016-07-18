package sampler

import (
	"hash/fnv"
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"

	log "github.com/cihub/seelog"

	"github.com/DataDog/raclette/config"
	"github.com/DataDog/raclette/model"
	"github.com/DataDog/raclette/statsd"
)

// maxTPS is a hard-limit on the maximum number of traces to sample per second
const maxTPS = 100

var statsdSignatureTags = []string{"sampler:signature"}

// Signature is a simple representation of trace, used to identify simlar traces
type Signature uint64

// SignatureSampler samples by identifying traces with a signature then score it
type SignatureSampler struct {
	// Last time we sampled a given signature (epoch in seconds)
	lastTSBySignature map[Signature]float64
	// Traces sampled kept until the next flush
	sampledTraces []model.Trace
	// Time of the last flush
	lastFlush float64

	// Scoring configuration
	sMin   float64 // Score required to be sampled, sample when score is over sMin
	theta  float64 // Typical last-seen duration (in s) after which we want to sample a trace
	jitter float64 // Multiplicative random coefficient (0 to 1)
	tpsMax float64 // Hard-limit on the number of traces per second

	mu sync.Mutex
}

// NewSignatureSampler creates a new SignatureSampler, ready to ingest traces
func NewSignatureSampler(conf *config.AgentConfig) *SignatureSampler {
	// TODO: have a go-routine expiring old signatures from lastTSBySignature
	// TODO: have a max on the size of lastTSBySignature

	return &SignatureSampler{
		lastTSBySignature: map[Signature]float64{},
		sampledTraces:     []model.Trace{},
		lastFlush:         float64(time.Now().UnixNano()) / 1e9,

		sMin:   conf.SamplerSMin,
		theta:  conf.SamplerTheta,
		jitter: conf.SamplerJitter,
		// Hardcoded hard-limit for now, to prevent massive spam
		tpsMax: maxTPS,
	}
}

// AddTrace samples a trace then keep it until the next flush
func (s *SignatureSampler) AddTrace(trace model.Trace) {
	signature := s.ComputeSignature(trace)

	s.mu.Lock()

	score := s.GetScore(signature)
	sampled := score > s.sMin
	if sampled {
		s.sampledTraces = append(s.sampledTraces, trace)
		s.lastTSBySignature[signature] = float64(time.Now().UnixNano()) / 1e9
	}

	s.mu.Unlock()

	statsd.Client.Count("trace_agent.sampler.trace.total", 1, statsdSignatureTags, 1)
	statsd.Client.Count("trace_agent.sampler.span.total", int64(len(trace)), statsdSignatureTags, 1)
	if sampled {
		statsd.Client.Count("trace_agent.sampler.trace.kept", 1, statsdSignatureTags, 1)
		statsd.Client.Count("trace_agent.sampler.span.kept", int64(len(trace)), statsdSignatureTags, 1)
	}

	log.Debugf("trace_id:%v signature:%v score:%v sampled:%v", trace[0].TraceID, signature, score, sampled)
}

// GetScore gives a score to a trace reflecting how strong we want to sample it
// Current implementation only cares about the last time a similar trace was seen + some randomness
// Score is from 0 to 10.
func (s *SignatureSampler) GetScore(signature Signature) float64 {
	timeScore := s.GetTimeScore(signature)

	// Add some jitter
	return timeScore * (1 + s.jitter*(1-2*rand.Float64()))
}

// GetTimeScore function constants to give it the shape we want
const (
	logMultiplier = 10
	// logRescaler   = 5 * 1 / math.Log(1+logMultiplier)
	logRescaler = 11.989476363991853
)

// GetTimeScore gives a score based on the square root of the last time this signature was seen.
// Current implementation and constant give a score of:
// | Δ/θ | Score |
// | --- | ----- |
// |  0  |    0  |
// |.02  |  .35  |
// | .2  |  2.3  |
// | .5  |  3.7  |
// |  1  |    5  |
// |  2  |  6.3  |
// |  5  |  8.2  |
// | 10  |  9.6  |
// | 12+ |   10  |
// | --- | ----- |
func (s *SignatureSampler) GetTimeScore(signature Signature) float64 {
	ts, seen := s.lastTSBySignature[signature]
	if !seen {
		return 10
	}
	delta := float64(time.Now().UnixNano())/1e9 - ts

	if delta <= 0 {
		return 0
	}

	return math.Min(logRescaler*math.Log(1+logMultiplier*delta/s.theta), 10)
}

// spanHash is the type of the hashes used during the computation of a signature
// Use FNV for hashing since it is super-cheap and we have no cryptographic needs
type spanHash uint32
type spanHashSlice []spanHash

func (p spanHashSlice) Len() int           { return len(p) }
func (p spanHashSlice) Less(i, j int) bool { return p[i] < p[j] }
func (p spanHashSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func SortHashes(hashes []spanHash)         { sort.Sort(spanHashSlice(hashes)) }

// ComputeSignature generates a signature of a trace
// Signature based on the hash of (service, name, resource, is_error) for the root, plus the set of
// (service, name, is_error) of each span.
func (s *SignatureSampler) ComputeSignature(trace model.Trace) Signature {
	traceHash := computeRootHash(s.getRoot(trace))
	spanHashes := make([]spanHash, len(trace))

	for i := range trace {
		spanHashes = append(spanHashes, computeSpanHash(trace[i]))
	}

	// Now sort, dedupe then merge all the hashes to build the signature
	SortHashes(spanHashes)

	last := spanHashes[0]
	idx := 1
	for i := 1; i < len(spanHashes); i++ {
		if spanHashes[i] != last {
			last = spanHashes[i]
			spanHashes[idx] = last
			idx++
		}
	}
	// spanHashes[:idx] is the sorted and deduped slice

	// Build the signature like a barbarian (with a XOR of all the hashes).
	// Stupid but cheap and does the job for now.
	for i := 0; i < idx; i++ {
		traceHash = spanHashes[i] ^ traceHash
	}

	return Signature(traceHash)
}

func computeSpanHash(span model.Span) spanHash {
	h := fnv.New32a()
	h.Write([]byte(span.Service))
	h.Write([]byte(span.Name))
	h.Write([]byte{byte(span.Error)})

	return spanHash(h.Sum32())
}

func computeRootHash(span model.Span) spanHash {
	h := fnv.New32a()
	h.Write([]byte(span.Service))
	h.Write([]byte(span.Name))
	h.Write([]byte(span.Resource))
	h.Write([]byte{byte(span.Error)})

	return spanHash(h.Sum32())
}

// getRoot extract the root span from a trace
func (s *SignatureSampler) getRoot(trace model.Trace) model.Span {
	// This current implementation is not 100% reliable, and would be wrong if we receive a sub-trace with its local
	// root not being at the end
	for i := range trace {
		if trace[len(trace)-1-i].ParentID == 0 {
			return trace[len(trace)-1-i]
		}
	}
	return trace[len(trace)-1]
}

// Flush returns representative spans based on GetSamples and reset its internal memory
func (s *SignatureSampler) Flush() []model.Trace {
	now := float64(time.Now().UnixNano()) / 1e9
	sampledDuration := now - s.lastFlush
	hardLimit := int(s.tpsMax * sampledDuration)

	s.mu.Lock()
	samples := s.sampledTraces
	s.sampledTraces = []model.Trace{}
	s.lastFlush = now
	s.mu.Unlock()

	// Ensure the hard limit the dumb way
	// TODO: adjust sampler configuration instead
	if len(samples) > hardLimit {
		log.Warnf("truncate set of sampled traces (from %v to %v), you should reduce sampler sensitivity", len(samples), hardLimit)
		return samples[:hardLimit]
	}

	return samples
}
