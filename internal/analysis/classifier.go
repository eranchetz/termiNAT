package analysis

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

type IPRanges struct {
	Prefixes []IPPrefix `json:"prefixes"`
}

type IPPrefix struct {
	IPPrefix string `json:"ip_prefix"`
	Region   string `json:"region"`
	Service  string `json:"service"`
}

type TrafficClassifier struct {
	s3Ranges       []*net.IPNet
	dynamoRanges   []*net.IPNet
}

func NewTrafficClassifier() (*TrafficClassifier, error) {
	resp, err := http.Get("https://ip-ranges.amazonaws.com/ip-ranges.json")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch AWS IP ranges: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read IP ranges: %w", err)
	}

	var ranges IPRanges
	if err := json.Unmarshal(body, &ranges); err != nil {
		return nil, fmt.Errorf("failed to parse IP ranges: %w", err)
	}

	tc := &TrafficClassifier{}
	for _, prefix := range ranges.Prefixes {
		_, ipNet, err := net.ParseCIDR(prefix.IPPrefix)
		if err != nil {
			continue
		}

		switch prefix.Service {
		case "S3":
			tc.s3Ranges = append(tc.s3Ranges, ipNet)
		case "DYNAMODB":
			tc.dynamoRanges = append(tc.dynamoRanges, ipNet)
		}
	}

	return tc, nil
}

func (tc *TrafficClassifier) ClassifyIP(ip string) string {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return "unknown"
	}

	for _, ipNet := range tc.s3Ranges {
		if ipNet.Contains(parsedIP) {
			return "s3"
		}
	}

	for _, ipNet := range tc.dynamoRanges {
		if ipNet.Contains(parsedIP) {
			return "dynamodb"
		}
	}

	return "other"
}

type FlowLogRecord struct {
	SrcAddr  string
	DstAddr  string
	SrcPort  string
	DstPort  string
	Protocol string
	Bytes    int64
}

func ParseFlowLogLine(line string) (*FlowLogRecord, error) {
	fields := strings.Fields(line)
	if len(fields) < 14 {
		return nil, fmt.Errorf("invalid flow log format")
	}

	var bytes int64
	fmt.Sscanf(fields[10], "%d", &bytes)

	return &FlowLogRecord{
		SrcAddr:  fields[3],
		DstAddr:  fields[4],
		SrcPort:  fields[5],
		DstPort:  fields[6],
		Protocol: fields[7],
		Bytes:    bytes,
	}, nil
}
