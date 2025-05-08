package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    *Config
		wantErr bool
	}{
		{
			name:    "minimal config",
			content: `url: https://example.com`,
			want: &Config{
				URL:      "https://example.com",
				Interval: time.Second,
				Timeout: TimeoutConfig{
					Connect: 3 * time.Second,
					Read:    7 * time.Second,
				},
			},
			wantErr: false,
		},
		{
			name: "full config",
			content: `url: https://example.com
interval: 2s
timeout:
  connect: 5s
  read: 10s
cookies:
  - key: session
    value: abc123
cookie_file: /tmp/cookies.txt
log:
  path: /tmp/log{{.ymdhms}}.txt
  format: "{{.requestedAt}}\t{{.statusCode}}\t{{.duration}}"`,
			want: &Config{
				URL:      "https://example.com",
				Interval: 2 * time.Second,
				Timeout: TimeoutConfig{
					Connect: 5 * time.Second,
					Read:    10 * time.Second,
				},
				Cookies: []CookieConfig{
					{
						Key:   "session",
						Value: "abc123",
					},
				},
				CookieFile: "/tmp/cookies.txt",
				Log: &LogConfig{
					Path:   "/tmp/log{{.ymdhms}}.txt",
					Format: "{{.requestedAt}}\t{{.statusCode}}\t{{.duration}}",
				},
			},
			wantErr: false,
		},
		{
			name:    "empty config",
			content: ``,
			want: &Config{
				Interval: time.Second,
				Timeout: TimeoutConfig{
					Connect: 3 * time.Second,
					Read:    7 * time.Second,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary config file
			tmpFile := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(tmpFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to write test config: %v", err)
			}

			got, err := LoadConfig(tmpFile)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if got.URL != tt.want.URL {
					t.Errorf("URL = %v, want %v", got.URL, tt.want.URL)
				}
				if got.Interval != tt.want.Interval {
					t.Errorf("Interval = %v, want %v", got.Interval, tt.want.Interval)
				}
				if got.Timeout.Connect != tt.want.Timeout.Connect {
					t.Errorf("Timeout.Connect = %v, want %v", got.Timeout.Connect, tt.want.Timeout.Connect)
				}
				if got.Timeout.Read != tt.want.Timeout.Read {
					t.Errorf("Timeout.Read = %v, want %v", got.Timeout.Read, tt.want.Timeout.Read)
				}
				if len(got.Cookies) != len(tt.want.Cookies) {
					t.Errorf("Cookies length = %v, want %v", len(got.Cookies), len(tt.want.Cookies))
				}
				if got.CookieFile != tt.want.CookieFile {
					t.Errorf("CookieFile = %v, want %v", got.CookieFile, tt.want.CookieFile)
				}
				if (got.Log == nil) != (tt.want.Log == nil) {
					t.Errorf("Log = %v, want %v", got.Log, tt.want.Log)
				}
				if got.Log != nil && tt.want.Log != nil {
					if got.Log.Path != tt.want.Log.Path {
						t.Errorf("Log.Path = %v, want %v", got.Log.Path, tt.want.Log.Path)
					}
					if got.Log.Format != tt.want.Log.Format {
						t.Errorf("Log.Format = %v, want %v", got.Log.Format, tt.want.Log.Format)
					}
				}
			}
		})
	}
}

func TestLoadCookiesFromFile(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
		wantErr bool
	}{
		{
			name: "valid cookies",
			content: `# Netscape HTTP Cookie File
# https://curl.haxx.se/rfc/cookie_spec.html
# This is a generated file!  Do not edit.

.example.com	TRUE	/	FALSE	1735689600	session	abc123
.example.com	TRUE	/	FALSE	1735689600	user	xyz789`,
			want:    2,
			wantErr: false,
		},
		{
			name: "empty file",
			content: `# Netscape HTTP Cookie File
# https://curl.haxx.se/rfc/cookie_spec.html`,
			want:    0,
			wantErr: false,
		},
		{
			name: "invalid format",
			content: `invalid
format
here`,
			want:    0,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary cookie file
			tmpFile := filepath.Join(t.TempDir(), "cookies.txt")
			if err := os.WriteFile(tmpFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to write test cookie file: %v", err)
			}

			got, err := loadCookiesFromFile(tmpFile)
			if (err != nil) != tt.wantErr {
				t.Errorf("loadCookiesFromFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(got) != tt.want {
				t.Errorf("loadCookiesFromFile() got %d cookies, want %d", len(got), tt.want)
			}
		})
	}
}

func TestWriteLog(t *testing.T) {
	tests := []struct {
		name     string
		config   *LogConfig
		status   int
		duration time.Duration
		want     string
		wantErr  bool
	}{
		{
			name: "simple format",
			config: &LogConfig{
				Path:   "test.log",
				Format: "{{.StatusCode}}",
			},
			status:   200,
			duration: 100 * time.Millisecond,
			want:     "200",
			wantErr:  false,
		},
		{
			name: "full format",
			config: &LogConfig{
				Path:   "test.log",
				Format: "{{.RequestedAt}}\t{{.StatusCode}}\t{{.Duration}}",
			},
			status:   404,
			duration: 150 * time.Millisecond,
			want:     "404",
			wantErr:  false,
		},
		{
			name: "invalid template",
			config: &LogConfig{
				Path:   "test.log",
				Format: "{{.Invalid}}",
			},
			status:   200,
			duration: 100 * time.Millisecond,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory for log files
			tmpDir := t.TempDir()
			tt.config.Path = filepath.Join(tmpDir, tt.config.Path)

			config := &Config{
				Log: tt.config,
			}

			err := config.WriteLog(time.Now(), tt.status, tt.duration)
			if (err != nil) != tt.wantErr {
				t.Errorf("WriteLog() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				content, err := os.ReadFile(tt.config.Path)
				if err != nil {
					t.Errorf("Failed to read log file: %v", err)
					return
				}

				if !filepath.IsAbs(tt.config.Path) {
					t.Errorf("Log file path is not absolute: %s", tt.config.Path)
				}
				if !strings.Contains(string(content), tt.want) {
					t.Errorf("Log content = %s, want %s", string(content), tt.want)
				}
			}
		})
	}
}
