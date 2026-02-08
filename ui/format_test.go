package ui

import (
	"strings"
	"testing"
)

func TestFormatCurrency(t *testing.T) {
	tests := []struct {
		amount float64
		want   string
	}{
		{0, "$0.00"},
		{0.5, "$0.50"},
		{9.99, "$9.99"},
		{1234.56, "$1,234.56"},
		{1000000, "$1,000,000.00"},
	}
	for _, tt := range tests {
		got := formatCurrency(tt.amount)
		if got != tt.want {
			t.Errorf("formatCurrency(%v) = %q, want %q", tt.amount, got, tt.want)
		}
	}
}

func TestFormatCurrencyNoBrokenVerbs(t *testing.T) {
	// Regression: %,. format verb caused "$%!,(float64=0).2f" output
	for _, v := range []float64{0, 1, 99.99, 12345.67} {
		got := formatCurrency(v)
		if strings.Contains(got, "%!") || strings.Contains(got, "float64") {
			t.Errorf("formatCurrency(%v) has broken format verb: %q", v, got)
		}
	}
}
