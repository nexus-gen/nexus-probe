package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"nexus-probe/internal/auth"
	"nexus-probe/internal/checker"
	"nexus-probe/internal/config"
	"nexus-probe/internal/models"
	"nexus-probe/internal/probe"
	"nexus-probe/internal/sender"
	"nexus-probe/internal/storage"
	"nexus-probe/internal/util"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var (
	configPath string
	interval   time.Duration
	outputPath string
	serverUrl  string
)

func init() {
	flag.StringVar(&configPath, "config", "configs/targets.yaml", "path to config")
	flag.DurationVar(&interval, "interval", 60*time.Second, "measurement interval (e.g., 10s, 30s, 1m, 5m)")
	flag.StringVar(&outputPath, "output", "measurements.jsonl", "output file path")
	flag.StringVar(&serverUrl, "server", "", "server URL for probe registration and metrics")
}

func main() {
	flag.Parse()

	printConfig()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("Daemon started, executing task every %v", interval)

	probeInfo, err := probe.DetectInfo(ctx)

	if err != nil {
		log.Printf("Failed to determine location: %v", err)
		probeInfo = models.ProbeInfo{ConnectionType: probe.DetectConnectionType()}
	}

	log.Println(probeInfo)

	doTask(ctx, probeInfo)
	for {
		select {
		case <-ticker.C:
			doTask(ctx, probeInfo)

		case <-ctx.Done():
			log.Printf("Shutting down")
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
	log.Printf("	Task started at %s", time.Now().Format("15:04:05"))

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 100,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	cfg := getCfg()

	results := checker.CheckAll(ctx, client, cfg.Targets, probeInfo)
	if err := storage.SaveResults(outputPath, results); err != nil {
		log.Printf("Failed to write file: %v\n", err)
	}

	priorityServerUrl := util.FirstNonEmpty(serverUrl, cfg.Server.URL)

	if priorityServerUrl != "" {
		apiKey, err := auth.GetOrRegister(ctx, priorityServerUrl, probeInfo)

		if err == nil {
			if sendErr := sender.Send(results, sender.Config{
				ServerUrl: priorityServerUrl,
				ApiKey:    apiKey,
			}); sendErr != nil {
				log.Printf("Failed to send to server: %v, server: %v", err, priorityServerUrl)
			}
		}
	} else {
		log.Printf("API not configured, working in offline mode")
	}

	log.Printf("Saved %d measurements\n", len(results))
}

func getCfg() *config.Config {
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	if len(cfg.Targets) == 0 {
		log.Fatalf("Target list is empty")
	}
	return cfg
}
