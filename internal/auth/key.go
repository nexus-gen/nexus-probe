package auth

import (
	"fmt"
	"os"
)

const keyFile = ".nexus-key"
const keyFilePermissions = 0600

func LoadKey() (string, error) {
	data, err := os.ReadFile(keyFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("saved API key not found")
		}
		return "", fmt.Errorf("failed to read key file: %v", err)
	}
	return string(data), nil
}

func SaveKey(apiKey string) error {
	err := os.WriteFile(keyFile, []byte(apiKey), keyFilePermissions)
	if err != nil {
		return fmt.Errorf("failed to write key file: %v", err)
	}
	return nil
}

func DeleteKey() error {
	err := os.Remove(keyFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete key file: %v", err)
	}
	return nil
}
