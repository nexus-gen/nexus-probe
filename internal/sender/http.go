package sender

import (
	"bytes"
	"encoding/json"
	"net/http"
	"nexus-probe/internal/models"
)

type Config struct {
	ServerUrl string
	ApiKey    string
}

func Send(measurements []models.Measurement, config Config) error {
	data, _ := json.Marshal(measurements)
	req, _ := http.NewRequest("POST", config.ServerUrl+"/api/v1/measurements", bytes.NewReader(data))
	req.Header.Set("X-API-Key", config.ApiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err // сервер недоступен — не страшно
	}
	defer resp.Body.Close()
	return nil
}
