package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httptrace"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
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

	connType := detectConnectionType()
	fmt.Println("Демон запущен выполняю задачу каждую минуту")

	doTask(ctx, detectConnectionType())
	for {
		select {
		case <-ticker.C:
			doTask(ctx, connType)

		case <-ctx.Done():
			fmt.Println("Сигнал получен. Завершаемся")
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

func doTask(ctx context.Context, connType string) {
	fmt.Printf("\n=== Задача начата в %s ===\n", time.Now().Format("15:04:05"))
	req, _ := http.NewRequestWithContext(ctx, "GET", "http://ip-api.com/json/", nil)
	currentLocation, err := http.DefaultClient.Do(req)

	if err != nil {
		fmt.Printf("❌ Ошибка: %v\n", err)
		return
	}
	defer currentLocation.Body.Close()

	var probe ProbeInfo
	json.NewDecoder(currentLocation.Body).Decode(&probe)

	probe.ConnectionType = connType
	fmt.Println(probe)

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

	results := fetchByUrls(urls, client, ctx, &probe)
	if err := saveResults(results); err != nil {
		fmt.Printf("❌ Ошибка записи: %v\n", err)
	}
	fmt.Printf("💾 Сохранено %d замеров\n", len(results))
}

func getTargetUrls() []Target {
	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatal(err)
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		log.Fatal(err)
	}

	return config.Targets
}

func fetchByUrls(urls []Target, client *http.Client, ctx context.Context, probe *ProbeInfo) []Measurement {
	var results []Measurement
	var dnsStart, dnsEnd, connectStart, connectEnd, tlsStart, tlsEnd, firstByte time.Time
	trace := getHttpTraceClient(&dnsStart, &dnsEnd, &connectStart, &connectEnd, &tlsStart, &tlsEnd, &firstByte)

	for _, target := range urls {

		fmt.Println("Fetching " + target.URL)
		req, _ := http.NewRequestWithContext(
			httptrace.WithClientTrace(ctx, trace),
			"GET", target.URL, nil,
		)
		req.Header.Set("User-Agent", "NexusProbe/0.1")

		start := time.Now()

		fmt.Printf("🚀 [%s] Начало запроса\n", start.Format("15:04:05.000"))

		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("❌ Ошибка: %v\n", err)
			return results
		}
		resp.Body.Close()

		meas := Measurement{
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
		fmt.Printf("%+v\n", meas)
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
				fmt.Printf("♻️  [%s] Использовано существующее соединение\n",
					time.Now().Format("15:04:05.000"))
			} else {
				fmt.Printf("✨ [%s] Создано новое соединение\n",
					time.Now().Format("15:04:05.000"))
			}
		},

		WroteRequest: func(info httptrace.WroteRequestInfo) {
			fmt.Printf("📤 [%s] Запрос отправлен\n", time.Now().Format("15:04:05.000"))
		},

		GotFirstResponseByte: func() {
			*firstByte = time.Now()
			fmt.Printf("📥 [%s] Получен первый байт ответа\n", time.Now().Format("15:04:05.000"))
		},
	}
}

func detectConnectionType() string {
	data, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return "unknown"
	}

	for _, line := range strings.Split(string(data), "\n")[1:] {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == "00000000" {
			iface := fields[0]

			if _, err := os.Stat("/sys/class/net/" + iface + "/wireless"); err == nil {
				return "wifi"
			}
			return "ethernet"
		}
	}

	return "unknown"
}

func saveResults(measurements []Measurement) error {
	f, err := os.OpenFile(outputPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, m := range measurements {
		data, err := json.Marshal(m)
		if err != nil {
			continue
		}
		f.Write(data)
		f.Write([]byte("\n"))
	}

	return nil
}
