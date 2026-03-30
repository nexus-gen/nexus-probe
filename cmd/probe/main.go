package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"nexus-probe/internal/checker"
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

	results := checker.CheckAll(ctx, client, urls, probeInfo)
	if err := storage.SaveResults(outputPath, results); err != nil {
		log.Printf("Ошибка записи: %v\n", err)
	}
	log.Printf("Сохранено %d замеров\n", len(results))
}

func getTargetUrls() []models.Target {
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Не удалось загрузить конфиг: %v", err)
	}
	if len(cfg.Targets) == 0 {
		log.Fatal("Список целей пуст")
	}
	return cfg.Targets
}
