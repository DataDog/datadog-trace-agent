package sampler

import (
	"hash/fnv"
	"sort"

	"github.com/StackVista/stackstate-trace-agent/model"
)

// Signature is a simple representation of trace, used to identify simlar traces
type Signature uint64

// spanHash is the type of the hashes used during the computation of a signature
// Use FNV for hashing since it is super-cheap and we have no cryptographic needs
type spanHash uint32
type spanHashSlice []spanHash

func (p spanHashSlice) Len() int           { return len(p) }
func (p spanHashSlice) Less(i, j int) bool { return p[i] < p[j] }
func (p spanHashSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func sortHashes(hashes []spanHash)         { sort.Sort(spanHashSlice(hashes)) }

// computeSignatureWithRootAndEnv generates the signature of a trace knowing its root
// Signature based on the hash of (env, service, name, resource, is_error) for the root, plus the set of
// (env, service, name, is_error) of each span.
func computeSignatureWithRootAndEnv(trace model.Trace, root *model.Span, env string) Signature {
	rootHash := computeRootHash(*root, env)
	spanHashes := make([]spanHash, 0, len(trace))

	for i := range trace {
		spanHashes = append(spanHashes, computeSpanHash(trace[i], env))
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

// computeServiceSignature generates the signature of a trace with minimal
// information such as service and env, this is typically used by distributed
// sampling based on priority, and used as a key to store the desired rate
// for a given service,env tuple.
func computeServiceSignature(root *model.Span, env string) Signature {
	return Signature(computeServiceHash(*root, env))
}

func computeSpanHash(span *model.Span, env string) spanHash {
	h := fnv.New32a()
	h.Write([]byte(env))
	h.Write([]byte(span.Service))
	h.Write([]byte(span.Name))
	h.Write([]byte{byte(span.Error)})

	return spanHash(h.Sum32())
}

func computeRootHash(span model.Span, env string) spanHash {
	h := fnv.New32a()
	h.Write([]byte(env))
	h.Write([]byte(span.Service))
	h.Write([]byte(span.Name))
	h.Write([]byte(span.Resource))
	h.Write([]byte{byte(span.Error)})

	return spanHash(h.Sum32())
}

func computeServiceHash(span model.Span, env string) spanHash {
	h := fnv.New32a()
	h.Write([]byte(span.Service))
	h.Write([]byte{','})
	h.Write([]byte(env))

	return spanHash(h.Sum32())
}
