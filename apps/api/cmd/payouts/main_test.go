package main

import "testing"

func TestParseRupees(t *testing.T) {
	tests := []struct {
		in   string
		want int64
	}{
		{"1500", 150_000},
		{"-1500.5", -150_050},
		{"1,500.00", 150_000},
		{"0.07", 7},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got, err := parseRupees(tt.in); err != nil || got != tt.want {
				t.Errorf("parseRupees(%q) = %d, %v; want %d", tt.in, got, err, tt.want)
			}
		})
	}
}

func TestParseRupeesRefusesPartCents(t *testing.T) {
	if _, err := parseRupees("10.005"); err == nil {
		t.Error("parseRupees(10.005) took a part cent")
	}
}
