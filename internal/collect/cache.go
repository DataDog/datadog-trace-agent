// Package collect provides tools to buffer spans in a size bounded cache
// until they are grown into complete traces or evicted for other reasons.
package collect

import (
	"container/list"
	"sync"
	"time"

	"github.com/DataDog/datadog-trace-agent/internal/agent"
	"github.com/DataDog/datadog-trace-agent/internal/statsd"
)

// EvictionReason specifies the reason why a trace was evicted.
type EvictionReason int

const (
	// EvictReasonRoot specifies that a trace was evicted because the root was received.
	EvictReasonRoot EvictionReason = iota
	// EvictReasonSpace specifies that this trace had to be evicted to free up memory space.
	EvictReasonSpace
)

// EvictedTrace contains information about a trace which was evicted.
type EvictedTrace struct {
	// Reason specifies the reason why this trace was evicted.
	Reason EvictionReason
	// Root specifies the root which was selected for this trace when the
	// reason is EvictReasonRoot.
	Root *agent.Span
	// Trace holds the trace that was evicted. It is only available
	// for the duration of the OnEvict call.
	Trace agent.Trace
	// LastMod specifies the last time this trace was added to.
	LastMod time.Time
	// Msgsize specifies the total msgpack computed (upper bound estimate)
	// message size of the trace.
	Msgsize int
}

// tagRootSpan is the metric key which signals distributed root spans.
const tagRootSpan = "_root_span"

// defaultCacheSize holds the maximum size allowed for the cache.
const defaultCacheSize = 200 * 1024 * 1024 // 200MB

// Settings specifies the settings to initialize the Cache with.
type Settings struct {
	// Out channel specifies the channel where evicted traces will be sent
	// to before eviction.
	Out chan<- EvictedTrace

	// MaxSize specifies the maximum space allowed for the cache. This space is calculated
	// based on the sum of span's Msgsize (msgpack generated) method [1]. It is an upper bound
	// estimate of the space the span will use when encoded. When thinking in terms of memory
	// size, the memory used by the cache should be evaluated to at most double this value.
	// [1] model/span_gen.go#333
	MaxSize int

	// Statsd specifies the dogstatsd client that will be used for debugging. Adding a client
	// might slightly affect performance.
	Statsd statsd.StatsClient
}

// NewCache returns a new Cache which will call the given function when a trace
// is evicted due to completion or due to maxSize being reached. If addr is not
// empty, it will be used to report stats to a statsd client.
func NewCache(opts Settings) *Cache {
	if opts.Out == nil {
		panic("collect.NewCache: out channel can not be nil")
	}
	if opts.MaxSize <= 0 {
		opts.MaxSize = defaultCacheSize
	}
	c := &Cache{
		out:     opts.Out,
		maxSize: opts.MaxSize,
		ll:      list.New(),
		cache:   make(map[uint64]*list.Element),
	}
	if opts.Statsd != nil {
		go c.monitor(opts.Statsd)
	}
	return c
}

// Cache caches spans until they are considered complete based on certain rules,
// or until they are evicted due to memory consumption limits (maxSize).
type Cache struct {
	out     chan<- EvictedTrace
	maxSize int

	mu    sync.RWMutex
	ll    *list.List // traces ordered by age
	cache map[uint64]*list.Element
	size  int
}

type trace struct {
	key     uint64
	size    int
	lastmod time.Time
	spans   agent.Trace
}

// Add adds a new list of spans to the cache and evicts them when they are completed
// or when they start filling up too much space.
func (c *Cache) Add(spans []*agent.Span) {
	c.addWithTime(spans, time.Now())
}

// addWithTime adds the spans to the cache with the timestamp from now
// added to each trace.
func (c *Cache) addWithTime(spans []*agent.Span, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var roots []*agent.Span
	for _, span := range spans {
		c.addSpan(span, now)
		if isRoot(span) {
			roots = append(roots, span)
		}
	}
	for _, span := range roots {
		c.evictReasonRoot(span)
	}
	for c.size > c.maxSize {
		c.evictReasonSpace()
	}
}

func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.cache)
}

func (c *Cache) addSpan(span *agent.Span, now time.Time) {
	key := span.TraceID
	ee, ok := c.cache[key]
	if ok {
		// trace already started
		c.ll.MoveToFront(ee)
	} else {
		// this is a new trace
		ee = c.ll.PushFront(&trace{key: key})
		c.cache[key] = ee
	}
	size := span.Msgsize()
	trace := ee.Value.(*trace)
	trace.spans = append(trace.spans, span)
	trace.lastmod = now
	trace.size += size
	c.size += size
}

// evictReasonSpace evicts the least recently added to trace from the cache.
func (c *Cache) evictReasonSpace() {
	ele := c.ll.Back()
	if ele == nil {
		return
	}
	t := ele.Value.(*trace)
	c.out <- EvictedTrace{
		Reason:  EvictReasonSpace,
		Trace:   agent.Trace(t.spans),
		LastMod: t.lastmod,
		Msgsize: t.size,
		// Root: nil (unknown)
	}
	c.remove(ele)
}

// evictReasonRoot evicts the trace at key with the given root.
func (c *Cache) evictReasonRoot(root *agent.Span) {
	key := root.TraceID
	if ele, found := c.cache[key]; found {
		t := ele.Value.(*trace)
		c.out <- EvictedTrace{
			Reason:  EvictReasonRoot,
			Trace:   agent.Trace(t.spans),
			LastMod: t.lastmod,
			Msgsize: t.size,
			Root:    root,
		}
		c.remove(ele)
	}
}

func (c *Cache) get(key uint64) (t *trace, ok bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ele, ok := c.cache[key]
	if ok {
		return ele.Value.(*trace), true
	}
	return nil, false
}

// remove removes ele from the cache.
func (c *Cache) remove(ele *list.Element) {
	trace := ele.Value.(*trace)
	c.size -= trace.size
	c.ll.Remove(ele)
	delete(c.cache, trace.key)
}

// isRoot returns true if the span is considered to be the last in its trace.
func isRoot(span *agent.Span) bool {
	rule1 := span.ParentID == 0                                    // parent ID is 0, means root
	rule2 := span.Metrics != nil && span.Metrics[tagRootSpan] == 1 // client set root
	return rule1 || rule2
}

// iterator is an iterator through the list. The iterator points to a nil
// element at the end of the list.
type iterator struct {
	c       *Cache
	e       *list.Element
	forward bool
}

// newIterator returns a new iterator for the Cache.
func (c *Cache) newIterator() *iterator {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return &iterator{c: c, e: c.ll.Front(), forward: true}
}

// newReverseIterator returns a new reverse iterator for the Cache.
func (c *Cache) newReverseIterator() *iterator {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return &iterator{c: c, e: c.ll.Back(), forward: false}
}

// len returns the total number of items in the list.
func (i *iterator) len() int {
	i.c.mu.RLock()
	defer i.c.mu.RUnlock()
	return i.c.ll.Len()
}

// getAndAdvance returns key, value, true if the current entry is valid and advances the
// iterator. Otherwise it returns nil, nil, false.
func (i *iterator) getAndAdvance() (t *trace, ok bool) {
	i.c.mu.Lock()
	defer i.c.mu.Unlock()
	if i.e == nil {
		return nil, false
	}
	t = i.e.Value.(*trace)
	if i.forward {
		i.e = i.e.Next()
	} else {
		i.e = i.e.Prev()
	}
	return t, true
}
