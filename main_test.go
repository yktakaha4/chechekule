package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBasicRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := &Config{
		URL:      server.URL,
		Interval: 100 * time.Millisecond,
		Timeout: TimeoutConfig{
			Connect: 1 * time.Second,
			Read:    1 * time.Second,
		},
	}

	done := make(chan bool)
	go func() {
		time.Sleep(250 * time.Millisecond) // Wait for 2-3 requests
		done <- true
	}()

	if err := runCheck(config, done); err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := &Config{
		URL:      server.URL,
		Interval: 100 * time.Millisecond,
		Timeout: TimeoutConfig{
			Connect: 100 * time.Millisecond,
			Read:    100 * time.Millisecond,
		},
	}

	done := make(chan bool)
	go func() {
		time.Sleep(250 * time.Millisecond)
		done <- true
	}()

	if err := runCheck(config, done); err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestCookies(t *testing.T) {
	expectedCookie := "test-cookie"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie := r.Header.Get("Cookie")
		if !strings.Contains(cookie, expectedCookie) {
			t.Errorf("Expected cookie %s, got %s", expectedCookie, cookie)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := &Config{
		URL:      server.URL,
		Interval: 100 * time.Millisecond,
		Timeout: TimeoutConfig{
			Connect: 1 * time.Second,
			Read:    1 * time.Second,
		},
		Cookies: []CookieConfig{
			{
				Key:   "session",
				Value: expectedCookie,
			},
		},
	}

	done := make(chan bool)
	go func() {
		time.Sleep(250 * time.Millisecond)
		done <- true
	}()

	if err := runCheck(config, done); err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestLogging(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := &Config{
		URL:      server.URL,
		Interval: 100 * time.Millisecond,
		Timeout: TimeoutConfig{
			Connect: 1 * time.Second,
			Read:    1 * time.Second,
		},
		Log: &LogConfig{
			Path:   logPath,
			Format: "{{.statusCode}}",
		},
	}

	done := make(chan bool)
	go func() {
		time.Sleep(250 * time.Millisecond)
		done <- true
	}()

	if err := runCheck(config, done); err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Check if log file exists and contains correct status code
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Errorf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "200") {
		t.Errorf("Expected log to contain status code 200, got %s", string(content))
	}
}

func TestConfigValidation(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte(`url: http://example.com
interval: 1s
timeout:
  connect: 3s
  read: 7s`)

	if err := os.WriteFile(tempFile, content, 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	config, err := LoadConfig(tempFile)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if config.URL != "http://example.com" {
		t.Errorf("Expected URL http://example.com, got %s", config.URL)
	}

	if config.Interval != time.Second {
		t.Errorf("Expected interval 1s, got %v", config.Interval)
	}

	if config.Timeout.Connect != 3*time.Second {
		t.Errorf("Expected connect timeout 3s, got %v", config.Timeout.Connect)
	}

	if config.Timeout.Read != 7*time.Second {
		t.Errorf("Expected read timeout 7s, got %v", config.Timeout.Read)
	}
}

func TestErrorStatus(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{
			name:     "DNS lookup failed",
			err:      fmt.Errorf("dial tcp: lookup example.com: no such host"),
			expected: StatusDNSLookupFailed,
		},
		{
			name:     "Connection refused",
			err:      fmt.Errorf("dial tcp 127.0.0.1:8080: connect: connection refused"),
			expected: StatusConnectionFailed,
		},
		{
			name:     "Timeout",
			err:      fmt.Errorf("context deadline exceeded"),
			expected: StatusTimeout,
		},
		{
			name:     "Unknown error",
			err:      fmt.Errorf("some other error"),
			expected: StatusUnknown,
		},
		{
			name:     "No error",
			err:      nil,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getErrorStatus(tt.err)
			if got != tt.expected {
				t.Errorf("getErrorStatus() = %v, want %v", got, tt.expected)
			}
		})
	}
}
