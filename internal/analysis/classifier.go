package analysis

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
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
	s3Ranges     []*net.IPNet
	dynamoRanges []*net.IPNet
	ecrRanges    []*net.IPNet
}

const (
	ipRangesURL   = "https://ip-ranges.amazonaws.com/ip-ranges.json"
	cacheTTL      = 24 * time.Hour
	cacheFileName = "aws-ip-ranges.json"
	cacheTimeFile = "aws-ip-ranges.timestamp"
)

func getCacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	cacheDir := filepath.Join(home, ".terminator", "cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", err
	}
	return cacheDir, nil
}

func isCacheValid(cacheDir string) bool {
	timestampPath := filepath.Join(cacheDir, cacheTimeFile)
	data, err := os.ReadFile(timestampPath)
	if err != nil {
		return false
	}

	var cacheTime time.Time
	if err := cacheTime.UnmarshalText(data); err != nil {
		return false
	}

	return time.Since(cacheTime) < cacheTTL
}

func loadFromCache(cacheDir string) ([]byte, error) {
	cachePath := filepath.Join(cacheDir, cacheFileName)
	return os.ReadFile(cachePath)
}

func saveToCache(cacheDir string, data []byte) error {
	cachePath := filepath.Join(cacheDir, cacheFileName)
	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		return err
	}

	timestampPath := filepath.Join(cacheDir, cacheTimeFile)
	timestamp, _ := time.Now().MarshalText()
	return os.WriteFile(timestampPath, timestamp, 0644)
}

func fetchIPRanges() ([]byte, error) {
	cacheDir, err := getCacheDir()
	if err == nil && isCacheValid(cacheDir) {
		if data, err := loadFromCache(cacheDir); err == nil {
			return data, nil
		}
	}

	resp, err := http.Get(ipRangesURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch AWS IP ranges: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read IP ranges: %w", err)
	}

	if cacheDir != "" {
		_ = saveToCache(cacheDir, data)
	}

	return data, nil
}

func NewTrafficClassifier() (*TrafficClassifier, error) {
	body, err := fetchIPRanges()
	if err != nil {
		return nil, err
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
		case "EC2":
			// ECR uses EC2 service IPs
			tc.ecrRanges = append(tc.ecrRanges, ipNet)
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

	for _, ipNet := range tc.ecrRanges {
		if ipNet.Contains(parsedIP) {
			return "ecr"
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
