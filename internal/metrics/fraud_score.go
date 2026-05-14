package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

type FraudScore struct {
	HandlerSeconds prometheus.Histogram
	KNNDuration    prometheus.Histogram
	Responses      *prometheus.CounterVec
}

func RegisterFraudScore(reg prometheus.Registerer) *FraudScore {
	m := &FraudScore{
		HandlerSeconds: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Namespace: "heimdall",
				Subsystem: "fraud_score",
				Name:      "handler_duration_seconds",
				Help:      "Duração do handler HTTP POST /fraud-score (wall clock).",
				Buckets:   []float64{.0005, .001, .0025, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
			},
		),
		KNNDuration: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Namespace: "heimdall",
				Subsystem: "fraud_score",
				Name:      "knn_duration_seconds",
				Help:      "Tempo gasto no passo KNN (FraudFraction), sem dados do payload.",
				Buckets:   []float64{.0005, .001, .0025, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
			},
		),
		Responses: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "heimdall",
				Subsystem: "fraud_score",
				Name:      "responses_total",
				Help:      "Respostas POST /fraud-score por classe de status (sem códigos individuais).",
			},
			[]string{"status_class"},
		),
	}
	reg.MustRegister(m.HandlerSeconds, m.KNNDuration, m.Responses)
	return m
}
