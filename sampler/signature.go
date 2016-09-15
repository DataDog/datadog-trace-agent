package sampler

import (
	"hash/fnv"
	"sort"

	"github.com/DataDog/raclette/model"
	log "github.com/cihub/seelog"
)

// Signature is a simple representation of trace, used to identify simlar traces
type Signature uint64

// ComputeSignature generates a signature of a trace
// Signature based on the hash of (service, name, resource, is_error) for the root, plus the set of
// (service, name, is_error) of each span.
func ComputeSignature(trace model.Trace) Signature {
	rootHash := computeRootHash(*GetRoot(trace))
	spanHashes := make([]spanHash, len(trace))

	for i := range trace {
		spanHashes = append(spanHashes, computeSpanHash(trace[i]))
	}

	// Now sort, dedupe then merge all the hashes to build the signature
	sortHashes(spanHashes)

	last := spanHashes[0]
	traceHash := last ^ rootHash
	for i := 1; i < len(spanHashes); i++ {
		if spanHashes[i] != last {
			last = spanHashes[i]
			traceHash = spanHashes[i] ^ traceHash
		}
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

// GetRoot extract the root span from a trace
func GetRoot(trace model.Trace) *model.Span {
	// That should be caught beforehand
	if len(trace) == 0 {
		return nil
	}
	// General case: go over all spans and check for one which matching parent
	parentIDToChild := map[uint64]*model.Span{}

	for i := range trace {
		// Common case optimization: check for span with ParentID == 0, starting from the end,
		// since some clients report the root last
		j := len(trace) - 1 - i
		if trace[j].ParentID == 0 {
			return &trace[j]
		}
		parentIDToChild[trace[j].ParentID] = &trace[j]
	}

	for i := range trace {
		if _, ok := parentIDToChild[trace[i].SpanID]; ok {
			delete(parentIDToChild, trace[i].SpanID)
		}
	}

	// Here, if the trace is valid, we should have len(parentIDToChild) == 1
	if len(parentIDToChild) != 1 {
		log.Errorf("Didn't reliably find the root span for traceID:%v", trace[0].TraceID)
	}

	// Have a safe bahavior if that's not the case
	// Pick the first span without its parent
	for parentID := range parentIDToChild {
		return parentIDToChild[parentID]
	}

	// Gracefully fail with the last span of the trace
	return &trace[len(trace)-1]
}

// spanHash is the type of the hashes used during the computation of a signature
// Use FNV for hashing since it is super-cheap and we have no cryptographic needs
type spanHash uint32
type spanHashSlice []spanHash

func (p spanHashSlice) Len() int           { return len(p) }
func (p spanHashSlice) Less(i, j int) bool { return p[i] < p[j] }
func (p spanHashSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func sortHashes(hashes []spanHash)         { sort.Sort(spanHashSlice(hashes)) }
