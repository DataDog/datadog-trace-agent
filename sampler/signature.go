package sampler

import (
	"hash/fnv"
	"sort"

	"github.com/DataDog/datadog-trace-agent/model"
)

// Signature is a simple representation of trace, used to identify simlar traces
type Signature uint64

// ComputeSignatureWithRoot generates the signature of a trace knowing its root
// Signature based on the hash of (service, name, resource, is_error) for the root, plus the set of
// (service, name, is_error) of each span.
func ComputeSignatureWithRoot(trace model.Trace, root *model.Span) Signature {
	rootHash := computeRootHash(*root)
	spanHashes := make([]spanHash, 0, len(trace))

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

// ComputeSignature is the same as ComputeSignatureWithRoot, except that it finds the root itself
func ComputeSignature(trace model.Trace) Signature {
	root := trace.GetRoot()

	return ComputeSignatureWithRoot(trace, root)
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

// spanHash is the type of the hashes used during the computation of a signature
// Use FNV for hashing since it is super-cheap and we have no cryptographic needs
type spanHash uint32
type spanHashSlice []spanHash

func (p spanHashSlice) Len() int           { return len(p) }
func (p spanHashSlice) Less(i, j int) bool { return p[i] < p[j] }
func (p spanHashSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func sortHashes(hashes []spanHash)         { sort.Sort(spanHashSlice(hashes)) }
