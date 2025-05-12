package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// バージョン情報
var (
	Version = "dev" // ビルド時に上書きされます
)

// エラーコードの定義
const (
	StatusDNSLookupFailed  = -1
	StatusConnectionFailed = -2
	StatusTimeout          = -3
	StatusRedirectLoop     = -4
	StatusAssertFailed     = -5
	StatusUnknown          = -999
)

// エラーメッセージの定義
var errorMessages = map[int]string{
	StatusDNSLookupFailed:  "DNS_LOOKUP_FAILED",
	StatusConnectionFailed: "CONNECTION_FAILED",
	StatusTimeout:          "TIMEOUT",
	StatusRedirectLoop:     "REDIRECT_LOOP_DETECTED",
	StatusAssertFailed:     "ASSERT_FAILED",
	StatusUnknown:          "UNKNOWN_ERROR",
}

func main() {
	configPath := flag.String("c", "", "config file path")
	version := flag.Bool("version", false, "show version")
	flag.Parse()

	if *version {
		fmt.Printf("chechekule version %s\n", Version)
		os.Exit(0)
	}

	var config *Config
	var err error

	if *configPath != "" {
		config, err = LoadConfig(*configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
			os.Exit(1)
		}
	} else {
		args := flag.Args()
		if len(args) != 1 {
			fmt.Fprintf(os.Stderr, "Usage: %s [-c config-file] [-version] <url>\n", os.Args[0])
			os.Exit(1)
		}
		config = &Config{
			URL:      args[0],
			Interval: time.Second,
			Timeout: TimeoutConfig{
				Connect: 3 * time.Second,
				Read:    7 * time.Second,
			},
		}
	}

	if err := runCheck(config, nil); err != nil {
		fmt.Fprintf(os.Stderr, "Error during execution: %v\n", err)
		os.Exit(1)
	}
}

func getErrorStatus(err error) int {
	if err == nil {
		return 0
	}

	errStr := err.Error()
	switch {
	case strings.Contains(errStr, "no such host"):
		return StatusDNSLookupFailed
	case strings.Contains(errStr, "connection refused"):
		return StatusConnectionFailed
	case strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded"):
		return StatusTimeout
	case strings.Contains(errStr, "stopped after") && strings.Contains(errStr, "redirects"):
		return StatusRedirectLoop
	default:
		return StatusUnknown
	}
}

func validateResponse(config *Config, resp *http.Response, body []byte) error {
	// ステータスコードの検証
	if len(config.Asserts.StatusCode.Values) > 0 {
		found := false
		for _, code := range config.Asserts.StatusCode.Values {
			if resp.StatusCode == code {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("status code %d not in expected values %v", resp.StatusCode, config.Asserts.StatusCode.Values)
		}
	}

	if config.Asserts.StatusCode.Regex != "" {
		re, err := regexp.Compile(config.Asserts.StatusCode.Regex)
		if err != nil {
			return fmt.Errorf("invalid status code regex: %w", err)
		}
		if !re.MatchString(strconv.Itoa(resp.StatusCode)) {
			return fmt.Errorf("status code %d does not match regex %s", resp.StatusCode, config.Asserts.StatusCode.Regex)
		}
	}

	// レスポンスボディの検証
	if config.Asserts.Body.Regex != "" {
		re, err := regexp.Compile(config.Asserts.Body.Regex)
		if err != nil {
			return fmt.Errorf("invalid body regex: %w", err)
		}
		if !re.Match(body) {
			return fmt.Errorf("body does not match regex %s", config.Asserts.Body.Regex)
		}
	}

	return nil
}

func runCheck(config *Config, done <-chan bool) error {
	// Execute hook if configured
	if config.Hooks.OnStart != "" {
		cmd := exec.Command(config.Hooks.OnStart)
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to execute hook: %v\n", err)
		}
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return fmt.Errorf("failed to create cookie jar: %w", err)
	}

	client := &http.Client{
		Jar: jar,
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if !config.FollowRedirects.Enabled {
				return http.ErrUseLastResponse
			}
			if len(via) >= config.FollowRedirects.MaxCount {
				return fmt.Errorf("stopped after %d redirects", config.FollowRedirects.MaxCount)
			}
			return nil
		},
	}

	if err := config.SetupCookies(jar); err != nil {
		return fmt.Errorf("failed to setup cookies: %w", err)
	}

	ticker := time.NewTicker(config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return nil
		case <-ticker.C:
			requestedAt := time.Now()
			start := time.Now()

			req, err := http.NewRequest("GET", config.URL, nil)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create request: %v\n", err)
				continue
			}

			ctx := req.Context()
			ctx, cancel := context.WithTimeout(ctx, config.Timeout.Connect+config.Timeout.Read)
			req = req.WithContext(ctx)
			defer cancel()

			resp, err := client.Do(req)
			duration := time.Since(start)

			var statusCode int
			var body []byte
			if err != nil {
				statusCode = getErrorStatus(err)
				fmt.Printf("%s\t%s\t%v\n", requestedAt.Format("2006-01-02T15:04:05.000Z07:00"), errorMessages[statusCode], duration)
			} else {
				body, err = io.ReadAll(resp.Body)
				resp.Body.Close()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Failed to read response body: %v\n", err)
					continue
				}

				if err := validateResponse(config, resp, body); err != nil {
					statusCode = StatusAssertFailed
					fmt.Printf("%s\t%s\t%v\n", requestedAt.Format("2006-01-02T15:04:05.000Z07:00"), errorMessages[statusCode], duration)
					fmt.Fprintf(os.Stderr, "Assert failed: %v\n", err)
					fmt.Fprintf(os.Stderr, "Response Headers:\n")
					for k, v := range resp.Header {
						fmt.Fprintf(os.Stderr, "  %s: %v\n", k, v)
					}
					fmt.Fprintf(os.Stderr, "Response Body:\n%s\n", string(body))
				} else {
					statusCode = resp.StatusCode
					fmt.Printf("%s\t%d\t%v\n", requestedAt.Format("2006-01-02T15:04:05.000Z07:00"), statusCode, duration)
				}
			}

			if config.Log != nil {
				if err := config.WriteLog(requestedAt, statusCode, duration); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to write log: %v\n", err)
				}
			}
		}
	}
}
