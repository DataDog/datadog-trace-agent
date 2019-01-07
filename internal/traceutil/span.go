package traceutil

import "github.com/DataDog/datadog-trace-agent/internal/pb"

const (
	// TraceMetricsKey is a tag key which, if set to true,
	// ensures all statistics are computed for this span.
	// [FIXME] *not implemented yet*
	TraceMetricsKey = "datadog.trace_metrics"

	// This is a special metric, it's 1 if the span is top-level, 0 if not.
	topLevelKey = "_top_level"
)

// HasTopLevel returns true if span is top-level.
func HasTopLevel(s *pb.Span) bool {
	return s.Metrics[topLevelKey] == 1
}

// HasForceMetrics returns true if statistics computation should be forced for this span.
func HasForceMetrics(s *pb.Span) bool {
	return s.Meta[TraceMetricsKey] == "true"
}

// setTopLevel sets the top-level attribute of the span.
func setTopLevel(s *pb.Span, topLevel bool) {
	if !topLevel {
		if s.Metrics == nil {
			return
		}
		delete(s.Metrics, topLevelKey)
		return
	}
	// Setting the metrics value, so that code downstream in the pipeline
	// can identify this as top-level without recomputing everything.
	s.SetMetric(topLevelKey, 1)
}
