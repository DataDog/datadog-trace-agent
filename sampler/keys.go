package sampler

const (
	// ClientSampleRateKey is the metric key of the client sampling rate.
	ClientSampleRateKey = "_dd.v1.client_sample_rate"

	// PreSampleRateKey is the metric key of the Agent pre-sampling rate.
	PreSampleRateKey = "_dd.v1.pre_sample_rate"

	// EventSampleRateKey is the metric key of the tracer or Agent configured event sampling rate.
	EventSampleRateKey = "_dd.v1.event_sample_rate"
)
