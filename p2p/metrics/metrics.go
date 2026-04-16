// Package metrics provides Prometheus metrics for the FunAI P2P node.
// Metrics are exposed on :9091/metrics by default.
//
// Usage:
//
//	metrics.RecordInferenceLatency(modelId, time.Since(start).Seconds())
//	metrics.IncrSettlements(modelId)
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// InferenceLatency tracks end-to-end inference latency in seconds.
	InferenceLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "funai_inference_latency_seconds",
			Help:    "End-to-end inference latency from InferRequest dispatch to final StreamToken.",
			Buckets: []float64{0.5, 1, 2, 5, 10, 20, 30},
		},
		[]string{"model_id"},
	)

	// VerificationLatency tracks teacher-forcing verification duration.
	VerificationLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "funai_verification_latency_seconds",
			Help:    "Teacher-forcing verification duration in seconds. Target: <0.6s.",
			Buckets: []float64{0.1, 0.2, 0.4, 0.6, 1.0, 2.0},
		},
		[]string{"model_id", "result"},
	)

	// SettlementsTotal counts successfully settled tasks.
	SettlementsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "funai_settlement_total",
			Help: "Total number of settled inference tasks.",
		},
		[]string{"model_id", "status"},
	)

	// AuditRate tracks the dynamic audit rate (per-mille).
	AuditRate = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "funai_audit_rate_permille",
			Help: "Current dynamic audit rate in per-mille (100 = 10%).",
		},
		[]string{"model_id"},
	)

	// WorkerJailTotal counts jail events.
	WorkerJailTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "funai_worker_jail_total",
			Help: "Total number of worker jail events.",
		},
		[]string{"jail_count"}, // "1", "2", "permanent"
	)

	// LeaderFailoverTotal counts leader failover events.
	LeaderFailoverTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "funai_leader_failover_total",
			Help: "Total number of leader failover events (1.5s inactivity triggered).",
		},
		[]string{"model_id"},
	)

	// P2PConnectedPeers tracks the current number of connected libp2p peers.
	P2PConnectedPeers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "funai_p2p_connected_peers",
		Help: "Current number of connected libp2p peers.",
	})

	// PendingTasksInMempool tracks tasks waiting in the leader mempool.
	PendingTasksInMempool = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "funai_leader_mempool_size",
			Help: "Number of tasks currently waiting in the leader mempool.",
		},
		[]string{"model_id"},
	)

	// BatchSettlementSize tracks how many tasks are bundled per MsgBatchSettlement.
	BatchSettlementSize = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "funai_batch_settlement_size",
		Help:    "Number of tasks per MsgBatchSettlement submission.",
		Buckets: []float64{1, 5, 10, 25, 50, 100},
	})

	// FraudProofTotal counts fraud proof submissions.
	FraudProofTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "funai_fraud_proof_total",
		Help: "Total number of MsgFraudProof submissions.",
	})
)

// RecordInferenceLatency records the end-to-end inference latency.
func RecordInferenceLatency(modelId string, seconds float64) {
	InferenceLatency.WithLabelValues(modelId).Observe(seconds)
}

// RecordVerificationLatency records a single verification round duration.
// result should be "pass" or "fail".
func RecordVerificationLatency(modelId, result string, seconds float64) {
	VerificationLatency.WithLabelValues(modelId, result).Observe(seconds)
}

// IncrSettlements increments the settlement counter.
// status should be "success", "fail", or "fraud".
func IncrSettlements(modelId, status string) {
	SettlementsTotal.WithLabelValues(modelId, status).Inc()
}

// SetAuditRate updates the current dynamic audit rate gauge.
func SetAuditRate(modelId string, ratePermille float64) {
	AuditRate.WithLabelValues(modelId).Set(ratePermille)
}

// IncrJail increments the jail counter for a given jail level.
// jailCount should be "1", "2", or "permanent".
func IncrJail(jailCount string) {
	WorkerJailTotal.WithLabelValues(jailCount).Inc()
}

// IncrLeaderFailover increments the leader failover counter.
func IncrLeaderFailover(modelId string) {
	LeaderFailoverTotal.WithLabelValues(modelId).Inc()
}

// SetConnectedPeers updates the connected peers gauge.
func SetConnectedPeers(n int) {
	P2PConnectedPeers.Set(float64(n))
}

// SetMempoolSize updates the mempool size gauge.
func SetMempoolSize(modelId string, size int) {
	PendingTasksInMempool.WithLabelValues(modelId).Set(float64(size))
}

// ObserveBatchSize records the size of a batch settlement.
func ObserveBatchSize(n int) {
	BatchSettlementSize.Observe(float64(n))
}

// IncrFraudProof increments the fraud proof counter.
func IncrFraudProof() {
	FraudProofTotal.Inc()
}

// StartServer starts the Prometheus metrics HTTP server on the given address.
// Typical address: ":9091".
func StartServer(addr string) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return http.ListenAndServe(addr, mux)
}
