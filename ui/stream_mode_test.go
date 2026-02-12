package ui

import (
	"bufio"
	"context"
	"strings"
	"testing"

	"github.com/doitintl/terminator/pkg/types"
)

func TestRunQuickScanInvalidUIMode(t *testing.T) {
	err := RunQuickScan(context.Background(), nil, "invalid")
	if err == nil {
		t.Fatal("expected invalid UI mode error")
	}
	if !strings.Contains(err.Error(), "invalid --ui value") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunDeepScanInvalidUIMode(t *testing.T) {
	err := RunDeepScan(context.Background(), nil, "us-east-1", 5, nil, "", "invalid", false, false, "", "", "", "")
	if err == nil {
		t.Fatal("expected invalid UI mode error")
	}
	if !strings.Contains(err.Error(), "invalid --ui value") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunDemoScanInvalidUIMode(t *testing.T) {
	err := RunDemoScan("invalid")
	if err == nil {
		t.Fatal("expected invalid UI mode error")
	}
	if !strings.Contains(err.Error(), "invalid --ui value") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPromptNATSelectionAllByDefault(t *testing.T) {
	r := &streamDeepScanRunner{
		nats: []types.NATGateway{
			{ID: "nat-a", VPCID: "vpc-1"},
			{ID: "nat-b", VPCID: "vpc-2"},
		},
		reader: bufio.NewReader(strings.NewReader("\n")),
	}

	selected, err := r.promptNATSelection()
	if err != nil {
		t.Fatalf("promptNATSelection returned error: %v", err)
	}
	if len(selected) != 2 {
		t.Fatalf("expected 2 NATs selected, got %d", len(selected))
	}
}

func TestPromptNATSelectionByIndex(t *testing.T) {
	r := &streamDeepScanRunner{
		nats: []types.NATGateway{
			{ID: "nat-a", VPCID: "vpc-1"},
			{ID: "nat-b", VPCID: "vpc-2"},
			{ID: "nat-c", VPCID: "vpc-3"},
		},
		reader: bufio.NewReader(strings.NewReader("2,1\n")),
	}

	selected, err := r.promptNATSelection()
	if err != nil {
		t.Fatalf("promptNATSelection returned error: %v", err)
	}
	if len(selected) != 2 {
		t.Fatalf("expected 2 NATs selected, got %d", len(selected))
	}
	if selected[0].ID != "nat-b" || selected[1].ID != "nat-a" {
		t.Fatalf("unexpected selected order: %s, %s", selected[0].ID, selected[1].ID)
	}
}

func TestPromptNATSelectionInvalidIndex(t *testing.T) {
	r := &streamDeepScanRunner{
		nats: []types.NATGateway{
			{ID: "nat-a", VPCID: "vpc-1"},
			{ID: "nat-b", VPCID: "vpc-2"},
		},
		reader: bufio.NewReader(strings.NewReader("3\n")),
	}

	_, err := r.promptNATSelection()
	if err == nil {
		t.Fatal("expected invalid index error")
	}
}
