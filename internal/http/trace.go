// Package http provides HTTP client utilities and helpers for AI providers.
package http

import (
	"crypto/tls"
	"net/http"
	"net/http/httptrace"
	"time"
)

// withConnectionTrace wraps an HTTP request with httptrace to collect connection-level metrics.
// It returns a modified request with the trace context attached and a connectionTrace instance
// that will be populated by the trace callbacks.
func withConnectionTrace(req *http.Request) (*http.Request, *connectionTrace) {
	trace := &connectionTrace{
		requestStartTime: time.Now(),
	}

	traceCtx := httptrace.WithClientTrace(req.Context(), &httptrace.ClientTrace{
		DNSStart: func(_ httptrace.DNSStartInfo) {
			trace.mu.Lock()
			defer trace.mu.Unlock()
			trace.dnsStart = time.Now()
		},
		DNSDone: func(_ httptrace.DNSDoneInfo) {
			trace.mu.Lock()
			defer trace.mu.Unlock()
			trace.dnsDone = time.Now()
		},
		ConnectStart: func(_, _ string) {
			trace.mu.Lock()
			defer trace.mu.Unlock()
			if trace.connectStart.IsZero() {
				trace.connectStart = time.Now()
			}
		},
		ConnectDone: func(_, _ string, _ error) {
			trace.mu.Lock()
			defer trace.mu.Unlock()
			trace.connectDone = time.Now()
		},
		TLSHandshakeStart: func() {
			trace.mu.Lock()
			defer trace.mu.Unlock()
			trace.tlsHandshakeStart = time.Now()
		},
		TLSHandshakeDone: func(_ tls.ConnectionState, _ error) {
			trace.mu.Lock()
			defer trace.mu.Unlock()
			trace.tlsHandshakeDone = time.Now()
		},
		GotConn: func(_ httptrace.GotConnInfo) {
			trace.mu.Lock()
			defer trace.mu.Unlock()
			trace.gotConn = time.Now()
		},
		GotFirstResponseByte: func() {
			trace.mu.Lock()
			defer trace.mu.Unlock()
			if trace.gotFirstResponseByte.IsZero() {
				trace.gotFirstResponseByte = time.Now()
			}
		},
	})

	return req.WithContext(traceCtx), trace
}

// markResponseReceived records when the response was received (for TTFB calculation).
// This should be called immediately after client.Do() returns successfully.
func (t *connectionTrace) markResponseReceived() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.gotFirstResponseByte.IsZero() {
		t.gotFirstResponseByte = time.Now()
	}
}

// toConnectionMetrics converts a connectionTrace to ConnectionMetrics.
// This should be called after the request completes to extract the final timing values.
func (t *connectionTrace) toConnectionMetrics() ConnectionMetrics {
	t.mu.Lock()
	defer t.mu.Unlock()

	metrics := ConnectionMetrics{}

	// DNS lookup duration
	if !t.dnsStart.IsZero() && !t.dnsDone.IsZero() {
		metrics.DNSLookupDuration = t.dnsDone.Sub(t.dnsStart)
	}

	// TCP connect duration
	if !t.connectStart.IsZero() && !t.connectDone.IsZero() {
		metrics.TCPConnectDuration = t.connectDone.Sub(t.connectStart)
	}

	// TLS handshake duration
	if !t.tlsHandshakeStart.IsZero() && !t.tlsHandshakeDone.IsZero() {
		metrics.TLSHandshakeDuration = t.tlsHandshakeDone.Sub(t.tlsHandshakeStart)
	}

	// Time to first byte
	if !t.requestStartTime.IsZero() && !t.gotFirstResponseByte.IsZero() {
		metrics.TimeToFirstByte = t.gotFirstResponseByte.Sub(t.requestStartTime)
	}

	// Total connection time (from request start to first byte)
	if !t.requestStartTime.IsZero() && !t.gotFirstResponseByte.IsZero() {
		metrics.TotalConnectionTime = t.gotFirstResponseByte.Sub(t.requestStartTime)
	}

	return metrics
}

// updateMinMaxAvg updates min, max, and average values for a duration metric.
// The running average is calculated as: (previousAvg * (n-1) + newValue) / n
func updateMinMaxAvg(min, max, avg *time.Duration, totalMeasurements int64, newValue time.Duration) {
	// Update minimum
	if *min < 0 || newValue < *min {
		*min = newValue
	}
	// Update maximum
	if newValue > *max {
		*max = newValue
	}
	// Update running average
	*avg = (*avg*time.Duration(totalMeasurements-1) + newValue) / time.Duration(totalMeasurements)
}

// updateConnectionMetricsSummary updates the ClientMetrics with new connection metrics.
// This aggregates per-request metrics into summary statistics.
func (c *HTTPClient) updateConnectionMetricsSummary(metrics ConnectionMetrics) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.metrics.ConnectionMetricsSummary == nil {
		c.metrics.ConnectionMetricsSummary = &ConnectionMetricsSummary{
			MinDNSLookupDuration:    -1, // Use -1 as sentinel for infinity
			MinTCPConnectDuration:   -1,
			MinTLSHandshakeDuration: -1,
			MinTimeToFirstByte:      -1,
			MinTotalConnectionTime:  -1,
		}
	}

	summary := c.metrics.ConnectionMetricsSummary
	summary.TotalMeasurements++

	if metrics.DNSLookupDuration > 0 {
		updateMinMaxAvg(
			&summary.MinDNSLookupDuration,
			&summary.MaxDNSLookupDuration,
			&summary.AvgDNSLookupDuration,
			summary.TotalMeasurements,
			metrics.DNSLookupDuration,
		)
	}

	if metrics.TCPConnectDuration > 0 {
		updateMinMaxAvg(
			&summary.MinTCPConnectDuration,
			&summary.MaxTCPConnectDuration,
			&summary.AvgTCPConnectDuration,
			summary.TotalMeasurements,
			metrics.TCPConnectDuration,
		)
	}

	if metrics.TLSHandshakeDuration > 0 {
		updateMinMaxAvg(
			&summary.MinTLSHandshakeDuration,
			&summary.MaxTLSHandshakeDuration,
			&summary.AvgTLSHandshakeDuration,
			summary.TotalMeasurements,
			metrics.TLSHandshakeDuration,
		)
	}

	if metrics.TimeToFirstByte > 0 {
		updateMinMaxAvg(
			&summary.MinTimeToFirstByte,
			&summary.MaxTimeToFirstByte,
			&summary.AvgTimeToFirstByte,
			summary.TotalMeasurements,
			metrics.TimeToFirstByte,
		)
	}

	if metrics.TotalConnectionTime > 0 {
		updateMinMaxAvg(
			&summary.MinTotalConnectionTime,
			&summary.MaxTotalConnectionTime,
			&summary.AvgTotalConnectionTime,
			summary.TotalMeasurements,
			metrics.TotalConnectionTime,
		)
	}
}
