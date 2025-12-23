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

// updateConnectionMetricsSummary updates the ClientMetrics with new connection metrics.
// This aggregates per-request metrics into summary statistics.
func (c *HTTPClient) updateConnectionMetricsSummary(metrics ConnectionMetrics) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.metrics.ConnectionMetricsSummary == nil {
		c.metrics.ConnectionMetricsSummary = &ConnectionMetricsSummary{
			MinDNSLookupDuration:     -1, // Use -1 as sentinel for infinity
			MinTCPConnectDuration:    -1,
			MinTLSHandshakeDuration:  -1,
			MinTimeToFirstByte:       -1,
			MinTotalConnectionTime:   -1,
		}
	}

	summary := c.metrics.ConnectionMetricsSummary
	summary.TotalMeasurements++

	// Update DNS lookup metrics
	if metrics.DNSLookupDuration > 0 {
		if summary.MinDNSLookupDuration < 0 || metrics.DNSLookupDuration < summary.MinDNSLookupDuration {
			summary.MinDNSLookupDuration = metrics.DNSLookupDuration
		}
		if metrics.DNSLookupDuration > summary.MaxDNSLookupDuration {
			summary.MaxDNSLookupDuration = metrics.DNSLookupDuration
		}
		summary.AvgDNSLookupDuration = (summary.AvgDNSLookupDuration*time.Duration(summary.TotalMeasurements-1) + metrics.DNSLookupDuration) / time.Duration(summary.TotalMeasurements)
	}

	// Update TCP connect metrics
	if metrics.TCPConnectDuration > 0 {
		if summary.MinTCPConnectDuration < 0 || metrics.TCPConnectDuration < summary.MinTCPConnectDuration {
			summary.MinTCPConnectDuration = metrics.TCPConnectDuration
		}
		if metrics.TCPConnectDuration > summary.MaxTCPConnectDuration {
			summary.MaxTCPConnectDuration = metrics.TCPConnectDuration
		}
		summary.AvgTCPConnectDuration = (summary.AvgTCPConnectDuration*time.Duration(summary.TotalMeasurements-1) + metrics.TCPConnectDuration) / time.Duration(summary.TotalMeasurements)
	}

	// Update TLS handshake metrics
	if metrics.TLSHandshakeDuration > 0 {
		if summary.MinTLSHandshakeDuration < 0 || metrics.TLSHandshakeDuration < summary.MinTLSHandshakeDuration {
			summary.MinTLSHandshakeDuration = metrics.TLSHandshakeDuration
		}
		if metrics.TLSHandshakeDuration > summary.MaxTLSHandshakeDuration {
			summary.MaxTLSHandshakeDuration = metrics.TLSHandshakeDuration
		}
		summary.AvgTLSHandshakeDuration = (summary.AvgTLSHandshakeDuration*time.Duration(summary.TotalMeasurements-1) + metrics.TLSHandshakeDuration) / time.Duration(summary.TotalMeasurements)
	}

	// Update TTFB metrics
	if metrics.TimeToFirstByte > 0 {
		if summary.MinTimeToFirstByte < 0 || metrics.TimeToFirstByte < summary.MinTimeToFirstByte {
			summary.MinTimeToFirstByte = metrics.TimeToFirstByte
		}
		if metrics.TimeToFirstByte > summary.MaxTimeToFirstByte {
			summary.MaxTimeToFirstByte = metrics.TimeToFirstByte
		}
		summary.AvgTimeToFirstByte = (summary.AvgTimeToFirstByte*time.Duration(summary.TotalMeasurements-1) + metrics.TimeToFirstByte) / time.Duration(summary.TotalMeasurements)
	}

	// Update total connection time metrics
	if metrics.TotalConnectionTime > 0 {
		if summary.MinTotalConnectionTime < 0 || metrics.TotalConnectionTime < summary.MinTotalConnectionTime {
			summary.MinTotalConnectionTime = metrics.TotalConnectionTime
		}
		if metrics.TotalConnectionTime > summary.MaxTotalConnectionTime {
			summary.MaxTotalConnectionTime = metrics.TotalConnectionTime
		}
		summary.AvgTotalConnectionTime = (summary.AvgTotalConnectionTime*time.Duration(summary.TotalMeasurements-1) + metrics.TotalConnectionTime) / time.Duration(summary.TotalMeasurements)
	}
}
