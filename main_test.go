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

func TestFollowRedirect(t *testing.T) {
	tests := []struct {
		name            string
		followRedirects FollowRedirectsConfig
		expectedCode    int
		setupServers    func() ([]*httptest.Server, string)
	}{
		{
			name: "follow redirects",
			followRedirects: FollowRedirectsConfig{
				Enabled:  true,
				MaxCount: 10,
			},
			expectedCode: http.StatusOK,
			setupServers: func() ([]*httptest.Server, string) {
				targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
				redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					http.Redirect(w, r, targetServer.URL, http.StatusFound)
				}))
				return []*httptest.Server{targetServer, redirectServer}, redirectServer.URL
			},
		},
		{
			name: "do not follow redirects",
			followRedirects: FollowRedirectsConfig{
				Enabled:  false,
				MaxCount: 10,
			},
			expectedCode: http.StatusFound,
			setupServers: func() ([]*httptest.Server, string) {
				targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
				redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					http.Redirect(w, r, targetServer.URL, http.StatusFound)
				}))
				return []*httptest.Server{targetServer, redirectServer}, redirectServer.URL
			},
		},
		{
			name: "follow recursive redirects",
			followRedirects: FollowRedirectsConfig{
				Enabled:  true,
				MaxCount: 10,
			},
			expectedCode: http.StatusOK,
			setupServers: func() ([]*httptest.Server, string) {
				// Create 3 servers with circular redirects
				servers := make([]*httptest.Server, 3)
				for i := range servers {
					i := i // Capture loop variable
					servers[i] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if i == 2 {
							w.WriteHeader(http.StatusOK)
							return
						}
						http.Redirect(w, r, servers[i+1].URL, http.StatusFound)
					}))
				}
				return servers, servers[0].URL
			},
		},
		{
			name: "too many redirects",
			followRedirects: FollowRedirectsConfig{
				Enabled:  true,
				MaxCount: 5,
			},
			expectedCode: StatusRedirectLoop,
			setupServers: func() ([]*httptest.Server, string) {
				// Create 6 servers with circular redirects
				servers := make([]*httptest.Server, 6)
				for i := range servers {
					i := i // Capture loop variable
					servers[i] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if i == len(servers)-1 {
							w.WriteHeader(http.StatusOK)
							return
						}
						http.Redirect(w, r, servers[i+1].URL, http.StatusFound)
					}))
				}
				return servers, servers[0].URL
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			servers, startURL := tt.setupServers()
			defer func() {
				for _, server := range servers {
					server.Close()
				}
			}()

			config := &Config{
				URL:             startURL,
				Interval:        100 * time.Millisecond,
				FollowRedirects: tt.followRedirects,
				Timeout: TimeoutConfig{
					Connect: 1 * time.Second,
					Read:    1 * time.Second,
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
		})
	}
}

func TestAsserts(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		handler     http.HandlerFunc
		expectError bool
	}{
		{
			name: "status code match",
			config: &Config{
				Asserts: AssertsConfig{
					StatusCode: StatusCodeAssert{
						Values: []int{200, 201},
					},
				},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			expectError: false,
		},
		{
			name: "status code mismatch",
			config: &Config{
				Asserts: AssertsConfig{
					StatusCode: StatusCodeAssert{
						Values: []int{200, 201},
					},
				},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			expectError: true,
		},
		{
			name: "status code regex match",
			config: &Config{
				Asserts: AssertsConfig{
					StatusCode: StatusCodeAssert{
						Regex: "^2..$",
					},
				},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			expectError: false,
		},
		{
			name: "status code regex mismatch",
			config: &Config{
				Asserts: AssertsConfig{
					StatusCode: StatusCodeAssert{
						Regex: "^2..$",
					},
				},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			expectError: true,
		},
		{
			name: "body regex match",
			config: &Config{
				Asserts: AssertsConfig{
					Body: BodyAssert{
						Regex: "success",
					},
				},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("operation success"))
			},
			expectError: false,
		},
		{
			name: "body regex mismatch",
			config: &Config{
				Asserts: AssertsConfig{
					Body: BodyAssert{
						Regex: "success",
					},
				},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("operation failed"))
			},
			expectError: true,
		},
		{
			name: "multiple asserts",
			config: &Config{
				Asserts: AssertsConfig{
					StatusCode: StatusCodeAssert{
						Values: []int{200},
					},
					Body: BodyAssert{
						Regex: "success",
					},
				},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("operation success"))
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			tt.config.URL = server.URL
			tt.config.Interval = 100 * time.Millisecond
			tt.config.Timeout = TimeoutConfig{
				Connect: 1 * time.Second,
				Read:    1 * time.Second,
			}

			done := make(chan bool)
			go func() {
				time.Sleep(250 * time.Millisecond)
				done <- true
			}()

			if err := runCheck(tt.config, done); err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})
	}
}

func TestHooks(t *testing.T) {
	// Create a temporary script file for testing
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test_hook.sh")
	scriptContent := `#!/bin/sh
echo "hook executed" > "` + filepath.Join(tmpDir, "hook_output.txt") + `"
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to write test script: %v", err)
	}

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
		Hooks: HooksConfig{
			OnStart: scriptPath,
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

	// Verify that the hook was executed
	outputPath := filepath.Join(tmpDir, "hook_output.txt")
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Errorf("Failed to read hook output: %v", err)
	}
	if string(content) != "hook executed\n" {
		t.Errorf("Expected hook output 'hook executed', got %s", string(content))
	}
}
