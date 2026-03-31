package checker

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"net/http/httptrace"
	"nexus-probe/internal/models"
	"time"
)

type traceResult struct {
	dnsStart, dnsEnd         time.Time
	connectStart, connectEnd time.Time
	tlsStart, tlsEnd         time.Time
	firstByte                time.Time
}

func newTrace() (*traceResult, *httptrace.ClientTrace) {
	t := &traceResult{}
	trace := &httptrace.ClientTrace{
		DNSStart:          func(_ httptrace.DNSStartInfo) { t.dnsStart = time.Now() },
		DNSDone:           func(_ httptrace.DNSDoneInfo) { t.dnsEnd = time.Now() },
		ConnectStart:      func(_, _ string) { t.connectStart = time.Now() },
		ConnectDone:       func(_, _ string, _ error) { t.connectEnd = time.Now() },
		TLSHandshakeStart: func() { t.tlsStart = time.Now() },
		TLSHandshakeDone:  func(_ tls.ConnectionState, _ error) { t.tlsEnd = time.Now() },

		GotConn: func(info httptrace.GotConnInfo) {
			if info.Reused {
				log.Printf("[%s] Reused existing connection",
					time.Now().Format("15:04:05.000"))
			} else {
				log.Printf("[%s] New connection established",
					time.Now().Format("15:04:05.000"))
			}
		},

		WroteRequest: func(_ httptrace.WroteRequestInfo) {
			log.Printf("[%s] Request sent", time.Now().Format("15:04:05.000"))
		},

		GotFirstResponseByte: func() {
			t.firstByte = time.Now()
			log.Printf("[%s] Received first response byte",
				time.Now().Format("15:04:05.000"))
		},
	}
	return t, trace
}

func CheckURL(ctx context.Context, client *http.Client, target models.Target, probe models.ProbeInfo) models.Measurement {
	t, trace := newTrace()

	req, _ := http.NewRequestWithContext(
		httptrace.WithClientTrace(ctx, trace),
		"GET", target.URL, nil,
	)
	req.Header.Set("User-Agent", "NexusProbe/0.1")

	start := time.Now()
	log.Printf("[%s] Starting request \n", start.Format("15:04:05.000"))

	resp, err := client.Do(req)
	if err != nil {
		return models.Measurement{
			Probe: probe, Target: target, Success: false, Error: err.Error(), Timestamp: time.Now(),
		}
	}
	resp.Body.Close()

	return models.Measurement{
		Probe:        probe,
		Target:       target,
		Success:      resp.StatusCode < 400,
		StatusCode:   resp.StatusCode,
		DNSLookup:    float64(t.dnsEnd.Sub(t.dnsStart).Milliseconds()),
		TCPConnect:   float64(t.connectEnd.Sub(t.connectStart).Milliseconds()),
		TLSHandshake: float64(t.tlsEnd.Sub(t.tlsStart).Milliseconds()),
		FirstByte:    float64(t.firstByte.Sub(t.tlsEnd).Milliseconds()),
		Total:        float64(time.Since(start).Milliseconds()),
		Timestamp:    time.Now(),
	}
}

func CheckAll(ctx context.Context, client *http.Client, targets []models.Target, probe models.ProbeInfo) []models.Measurement {
	var results []models.Measurement

	for _, target := range targets {

		log.Println("Fetching " + target.URL)

		meas := CheckURL(ctx, client, target, probe)
		results = append(results, meas)
		log.Printf("%+v\n", meas)
	}

	return results
}
