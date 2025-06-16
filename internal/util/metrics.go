package util

// MetricsCollector defines the interface for metrics collection.
type MetricsCollector interface {
	IncCounter(name string, labels map[string]string)
	ObserveHistogram(name string, value float64, labels map[string]string)
	SetGauge(name string, value float64, labels map[string]string)
}

// NoopMetrics is a stub implementation that does nothing.
type NoopMetrics struct{}

func (n *NoopMetrics) IncCounter(name string, labels map[string]string)          {}
func (n *NoopMetrics) ObserveHistogram(name string, value float64, labels map[string]string) {}
func (n *NoopMetrics) SetGauge(name string, value float64, labels map[string]string)         {}

// DefaultMetrics is the global metrics collector (can be replaced with a real one later).
var DefaultMetrics MetricsCollector = &NoopMetrics{} 