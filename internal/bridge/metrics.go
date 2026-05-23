package bridge

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	Registry       *prometheus.Registry
	MessagesIn     prometheus.Counter
	MessagesOut    prometheus.Counter
	MessagesFailed prometheus.Counter
	JobDuration    *prometheus.HistogramVec
	QueueDepth     *prometheus.GaugeVec
}

func NewMetrics() *Metrics {
	reg := prometheus.NewRegistry()
	m := &Metrics{
		Registry: reg,
		MessagesIn: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "bridge_messages_in_total",
			Help: "Total messages accepted on the inbound (megaAPI->Chatwoot) path.",
		}),
		MessagesOut: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "bridge_messages_out_total",
			Help: "Total messages accepted on the outbound (Chatwoot->megaAPI) path.",
		}),
		MessagesFailed: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "bridge_messages_failed_total",
			Help: "Total messages that exhausted retries and were marked failed.",
		}),
		JobDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "bridge_job_duration_seconds",
			Help:    "Worker job duration in seconds by direction.",
			Buckets: prometheus.DefBuckets,
		}, []string{"direction"}),
		QueueDepth: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "bridge_queue_depth",
			Help: "Current in-process queue depth by channel.",
		}, []string{"queue"}),
	}
	reg.MustRegister(m.MessagesIn, m.MessagesOut, m.MessagesFailed, m.JobDuration, m.QueueDepth)
	return m
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{Registry: m.Registry})
}

func (m *Metrics) ObserveJobDuration(direction string, start time.Time) {
	m.JobDuration.WithLabelValues(direction).Observe(time.Since(start).Seconds())
}

func (m *Metrics) UpdateQueueDepth(inbox, outbox int) {
	m.QueueDepth.WithLabelValues("inbox").Set(float64(inbox))
	m.QueueDepth.WithLabelValues("outbox").Set(float64(outbox))
}
