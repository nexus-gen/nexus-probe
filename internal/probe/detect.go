package probe

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"nexus-probe/internal/models"
	"os"
	"strings"
)

func DetectConnectionType() string {
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

func DetectInfo(ctx context.Context) (models.ProbeInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://ip-api.com/json/", nil)
	if err != nil {
		return models.ProbeInfo{}, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return models.ProbeInfo{}, fmt.Errorf("ip-api unavailable: %w", err)
	}
	defer resp.Body.Close()

	var info models.ProbeInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return models.ProbeInfo{}, err
	}

	info.ConnectionType = DetectConnectionType()
	return info, nil
}
