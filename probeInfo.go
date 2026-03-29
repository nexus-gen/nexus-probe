package main

type ProbeInfo struct {
	IP             string  `json:"query"`
	ISP            string  `json:"isp"`
	AS             string  `json:"as"`
	City           string  `json:"city"`
	Country        string  `json:"country"`
	Lat            float64 `json:"lat"`
	Lon            float64 `json:"lon"`
	ConnectionType string
}
