package storage

import (
	"encoding/json"
	"nexus-probe/internal/models"
	"os"
)

func SaveResults(outputPath string, measurements []models.Measurement) error {
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
