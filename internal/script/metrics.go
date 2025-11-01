package script

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds Prometheus metric collectors for script execution
type Metrics struct {
	executionDuration *prometheus.HistogramVec
	executionTotal    *prometheus.CounterVec
	executionFailures *prometheus.CounterVec
	executionTimeouts *prometheus.CounterVec
	scriptsActive     prometheus.Gauge
}

// NewMetrics creates a new script metrics collector
func NewMetrics() *Metrics {
	return &Metrics{
		executionDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "script_execution_duration_seconds",
				Help:    "Histogram of script execution durations",
				Buckets: prometheus.DefBuckets, // Default: 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10
			},
			[]string{"script_name", "trigger_type"},
		),
		executionTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "script_executions_total",
				Help: "Total number of script executions",
			},
			[]string{"script_name", "trigger_type", "result"},
		),
		executionFailures: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "script_execution_failures_total",
				Help: "Total number of script execution failures",
			},
			[]string{"script_name", "trigger_type", "error_type"},
		),
		executionTimeouts: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "script_execution_timeouts_total",
				Help: "Total number of script execution timeouts",
			},
			[]string{"script_name", "trigger_type"},
		),
		scriptsActive: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "scripts_active_total",
				Help: "Number of active (enabled) scripts",
			},
		),
	}
}

// RecordExecution records a script execution with duration and result
func (m *Metrics) RecordExecution(scriptName, triggerType string, durationSeconds float64, success bool) {
	m.executionDuration.WithLabelValues(scriptName, triggerType).Observe(durationSeconds)

	result := "success"
	if !success {
		result = "failure"
	}
	m.executionTotal.WithLabelValues(scriptName, triggerType, result).Inc()
}

// RecordFailure records a script execution failure
func (m *Metrics) RecordFailure(scriptName, triggerType, errorType string) {
	m.executionFailures.WithLabelValues(scriptName, triggerType, errorType).Inc()
}

// RecordTimeout records a script execution timeout
func (m *Metrics) RecordTimeout(scriptName, triggerType string) {
	m.executionTimeouts.WithLabelValues(scriptName, triggerType).Inc()
}

// SetActiveScripts sets the number of active scripts
func (m *Metrics) SetActiveScripts(count int) {
	m.scriptsActive.Set(float64(count))
}
