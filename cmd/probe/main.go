package main

import (
	"context"
	"crypto/tls"
	"flag"
	"log"
	"net/http"
	"net/http/httptrace"
	"nexus-probe/internal/config"
	"nexus-probe/internal/models"
	"nexus-probe/internal/probe"
	"nexus-probe/internal/storage"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var (
	configPath string
	interval   time.Duration
	outputPath string
)

func init() {
	flag.StringVar(&configPath, "config", "configs/targets.yaml", "path to config")
	flag.DurationVar(&interval, "interval", 60*time.Second, "measurement interval (e.g., 10s, 30s, 1m, 5m)")
	flag.StringVar(&outputPath, "output", "measurements.jsonl", "output file path")
}

func main() {
	flag.Parse()

	printConfig()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("Демон запущен выполняю задачу каждую минуту")

	probeInfo, err := probe.DetectInfo(ctx)

	if err != nil {
		log.Printf("Не удалось определить локацию: %v", err)
		probeInfo = models.ProbeInfo{ConnectionType: probe.DetectConnectionType()}
	}

	log.Println(probeInfo)

	doTask(ctx, probeInfo)
	for {
		select {
		case <-ticker.C:
			doTask(ctx, probeInfo)

		case <-ctx.Done():
			log.Printf("Завершаемся")
			return
		}
	}
}

func printConfig() {
	log.Println(strings.Repeat("=", 50))
	log.Println("Nexus Probe Configuration")
	log.Println(strings.Repeat("=", 50))
	log.Printf("%-20s: %s", "Config file", configPath)
	log.Printf("%-20s: %v", "Interval", interval)
	log.Printf("%-20s: %s", "Output path", outputPath)
	log.Println(strings.Repeat("=", 50))
}

func doTask(ctx context.Context, probeInfo models.ProbeInfo) {
	log.Printf("\n=== Задача начата в %s ===\n", time.Now().Format("15:04:05"))

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 100,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	urls := getTargetUrls()

	results := fetchByUrls(urls, client, ctx, &probeInfo)
	if err := storage.SaveResults(outputPath, results); err != nil {
		log.Printf("Ошибка записи: %v\n", err)
	}
	log.Printf("Сохранено %d замеров\n", len(results))
}

func getTargetUrls() []models.Target {
	cnf, _ := config.Load(configPath)
	return cnf.Targets
}

func fetchByUrls(urls []models.Target, client *http.Client, ctx context.Context, probe *models.ProbeInfo) []models.Measurement {
	var results []models.Measurement
	var dnsStart, dnsEnd, connectStart, connectEnd, tlsStart, tlsEnd, firstByte time.Time
	trace := getHttpTraceClient(&dnsStart, &dnsEnd, &connectStart, &connectEnd, &tlsStart, &tlsEnd, &firstByte)

	for _, target := range urls {

		log.Println("Fetching " + target.URL)
		req, _ := http.NewRequestWithContext(
			httptrace.WithClientTrace(ctx, trace),
			"GET", target.URL, nil,
		)
		req.Header.Set("User-Agent", "NexusProbe/0.1")

		start := time.Now()

		log.Printf("[%s] Начало запроса\n", start.Format("15:04:05.000"))

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("Ошибка: %v\n", err)
			return results
		}
		resp.Body.Close()

		meas := models.Measurement{
			Probe:        *probe, // заполнен при старте
			Target:       target,
			Success:      err == nil && resp.StatusCode < 400,
			StatusCode:   resp.StatusCode,
			DNSLookup:    float64(dnsEnd.Sub(dnsStart).Milliseconds()),
			TCPConnect:   float64(connectEnd.Sub(connectStart).Milliseconds()),
			TLSHandshake: float64(tlsEnd.Sub(tlsStart).Milliseconds()),
			FirstByte:    float64(firstByte.Sub(tlsEnd).Milliseconds()),
			Total:        float64(time.Since(start).Milliseconds()),
			Timestamp:    time.Now(),
		}

		results = append(results, meas)
		log.Printf("%+v\n", meas)
	}

	return results
}

func getHttpTraceClient(dnsStart *time.Time, dnsEnd *time.Time,
	connectStart *time.Time, connectEnd *time.Time,
	tlsStart *time.Time, tlsEnd *time.Time, firstByte *time.Time) *httptrace.ClientTrace {

	return &httptrace.ClientTrace{
		DNSStart: func(_ httptrace.DNSStartInfo) {
			*dnsStart = time.Now()
		},
		DNSDone: func(_ httptrace.DNSDoneInfo) {
			*dnsEnd = time.Now()
		},
		ConnectStart: func(_, _ string) {
			*connectStart = time.Now()
		},
		ConnectDone: func(_, _ string, _ error) {
			*connectEnd = time.Now()
		},
		TLSHandshakeStart: func() {
			*tlsStart = time.Now()
		},
		TLSHandshakeDone: func(_ tls.ConnectionState, _ error) {
			*tlsEnd = time.Now()
		},

		GotConn: func(info httptrace.GotConnInfo) {
			if info.Reused {
				log.Printf("[%s] Использовано существующее соединение\n",
					time.Now().Format("15:04:05.000"))
			} else {
				log.Printf("[%s] Создано новое соединение\n",
					time.Now().Format("15:04:05.000"))
			}
		},

		WroteRequest: func(info httptrace.WroteRequestInfo) {
			log.Printf("[%s] Запрос отправлен\n", time.Now().Format("15:04:05.000"))
		},

		GotFirstResponseByte: func() {
			*firstByte = time.Now()
			log.Printf("[%s] Получен первый байт ответа\n", time.Now().Format("15:04:05.000"))
		},
	}
}
