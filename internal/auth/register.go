package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"nexus-probe/internal/models"
)

type RegisterRequest struct {
	ProbeInfo models.ProbeInfo `json:"probe_info"`
}

type RegisterResponse struct {
	APIKey string `json:"api_key"`
	Error  string `json:"error,omitempty"`
}

func Register(ctx context.Context, serverURL string, probeInfo models.ProbeInfo) (string, error) {
	if serverURL == "" {
		return "", fmt.Errorf("ServerURL is not configured")
	}

	req := RegisterRequest{
		ProbeInfo: probeInfo,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %v", err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		"POST",
		serverURL+"/api/v1/register",
		bytes.NewReader(data),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "NexusProbe/0.1")

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("server is unavailable: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	var registerResp RegisterResponse
	err = json.Unmarshal(body, &registerResp)
	if err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to register: server returned status %d, err: %s", resp.StatusCode, registerResp.Error)
	}

	if registerResp.APIKey == "" {
		return "", fmt.Errorf("server did not return API key")
	}

	log.Printf("Probe successfully registered on server %s", serverURL)
	return registerResp.APIKey, nil
}

func GetOrRegister(ctx context.Context, serverURL string, probeInfo models.ProbeInfo) (string, error) {
	apiKey, err := LoadKey()
	if err == nil {
		log.Printf("Using saved API key")
		return apiKey, nil
	}

	log.Printf("Saved key not found, registering on server...")

	apiKey, err = Register(ctx, serverURL, probeInfo)
	if err != nil {
		log.Printf("Failed to register: %v", err)
		log.Printf("Working in offline mode")
		return "", err
	}

	err = SaveKey(apiKey)
	if err != nil {
		log.Printf("Failed to save key: %v", err)
	}

	return apiKey, nil
}
