package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"os"
	"time"
)

func main() {
	configPath := flag.String("c", "", "config file path")
	flag.Parse()

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
			fmt.Fprintf(os.Stderr, "Usage: %s [-c config-file] <url>\n", os.Args[0])
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

func runCheck(config *Config, done <-chan bool) error {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return fmt.Errorf("failed to create cookie jar: %w", err)
	}

	client := &http.Client{
		Jar: jar,
		Transport: &http.Transport{
			DisableKeepAlives: true,
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
			if err != nil {
				statusCode = -1
				fmt.Printf("%s\tError: %v\t%v\n", requestedAt.Format(time.RFC3339), err, duration)
			} else {
				statusCode = resp.StatusCode
				fmt.Printf("%s\t%d\t%v\n", requestedAt.Format(time.RFC3339), statusCode, duration)
				resp.Body.Close()
			}

			if config.Log != nil {
				if err := config.WriteLog(requestedAt, statusCode, duration); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to write log: %v\n", err)
				}
			}
		}
	}
}
