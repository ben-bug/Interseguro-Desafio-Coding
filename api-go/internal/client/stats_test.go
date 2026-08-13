package client

import (
	"testing"
	"time"
)

func TestNewStatsClientNormalizesBaseURL(t *testing.T) {
	client := NewStatsClient("  http://localhost:3000/  ", time.Second, 0, nil)

	if client.baseURL != "http://localhost:3000" {
		t.Fatalf("baseURL = %q, se esperaba http://localhost:3000", client.baseURL)
	}
}
