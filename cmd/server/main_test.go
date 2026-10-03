package main

import (
	"strings"
	"testing"
	"time"
)

func TestAttributionCheckInterval(t *testing.T) {
	tests := []struct {
		env     string
		want    time.Duration
		wantErr bool
	}{
		{"", 168 * time.Hour, false},
		{"24h", 24 * time.Hour, false},
		{"0", 0, false},
		{"-1h", 0, true},
		{"weekly", 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.env, func(t *testing.T) {
			t.Setenv("ATTRIBUTION_CHECK_INTERVAL", tc.env)
			got, err := attributionCheckInterval()
			if tc.wantErr {
				if err == nil || !strings.Contains(err.Error(), tc.env) {
					t.Errorf("want an error naming %q, got %v", tc.env, err)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Errorf("attributionCheckInterval() = %v, %v; want %v, nil", got, err, tc.want)
			}
		})
	}
}
