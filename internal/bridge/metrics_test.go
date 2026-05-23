package bridge

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMetrics_EndpointExposesBridgeSeries(t *testing.T) {
	s := newTestServer(t, nil)
	s.Metrics = NewMetrics()
	s.Metrics.MessagesIn.Inc()
	s.Metrics.MessagesOut.Add(2)
	s.Metrics.MessagesFailed.Inc()
	s.Metrics.JobDuration.WithLabelValues("inbound").Observe(0.42)
	s.Metrics.QueueDepth.WithLabelValues("inbox").Set(3)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	require.Contains(t, body, "bridge_messages_in_total")
	require.Contains(t, body, "bridge_messages_out_total")
	require.Contains(t, body, "bridge_messages_failed_total")
	require.Contains(t, body, "bridge_job_duration_seconds")
	require.Contains(t, body, "bridge_queue_depth")
	require.Contains(t, body, "bridge_messages_out_total 2")
}

func TestMetrics_NilMetricsHandlerStillResponds(t *testing.T) {
	s := newTestServer(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code,
		"metrics endpoint must respond even when Metrics is nil to avoid scraper alarms")
}

func TestMetrics_ObserveJobDurationTracksHistogram(t *testing.T) {
	m := NewMetrics()
	start := time.Now().Add(-50 * time.Millisecond)
	m.ObserveJobDuration("outbound", start)

	body := scrapeMetrics(t, m)
	require.Contains(t, body, `bridge_job_duration_seconds_count{direction="outbound"} 1`)
}

func TestMetrics_QueueDepthUpdater(t *testing.T) {
	m := NewMetrics()
	m.UpdateQueueDepth(7, 3)
	body := scrapeMetrics(t, m)
	require.Contains(t, body, `bridge_queue_depth{queue="inbox"} 7`)
	require.Contains(t, body, `bridge_queue_depth{queue="outbox"} 3`)
}

func scrapeMetrics(t *testing.T, m *Metrics) string {
	t.Helper()
	srv := httptest.NewServer(m.Handler())
	defer srv.Close()
	resp, err := http.Get(srv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			sb.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
	return sb.String()
}
