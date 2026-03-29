package models

import (
	"time"
)

type Measurement struct {
	// Кто измерял
	Probe ProbeInfo `json:"probe"`

	// Что измерял
	Target Target `json:"target"`

	// Результат
	Success    bool   `json:"success"`
	StatusCode int    `json:"status_code"`
	Error      string `json:"error,omitempty"`

	// Фазы (в миллисекундах)
	DNSLookup    float64 `json:"dns_lookup_ms"`
	TCPConnect   float64 `json:"tcp_connect_ms"`
	TLSHandshake float64 `json:"tls_handshake_ms"`
	FirstByte    float64 `json:"first_byte_ms"`
	Total        float64 `json:"total_ms"`

	Timestamp time.Time `json:"timestamp"`
}
