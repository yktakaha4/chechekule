package main

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"text/template"
	"time"

	"gopkg.in/yaml.v3"
)

type TimeoutConfig struct {
	Connect time.Duration `yaml:"connect"`
	Read    time.Duration `yaml:"read"`
}

type CookieConfig struct {
	Key   string `yaml:"key"`
	Value string `yaml:"value"`
}

type LogConfig struct {
	Path   string `yaml:"path"`
	Format string `yaml:"format"`
}

type Config struct {
	URL        string         `yaml:"url"`
	Interval   time.Duration  `yaml:"interval"`
	Timeout    TimeoutConfig  `yaml:"timeout"`
	Cookies    []CookieConfig `yaml:"cookies"`
	CookieFile string         `yaml:"cookie_file"`
	Log        *LogConfig     `yaml:"log"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	config := &Config{
		Interval: time.Second,
		Timeout: TimeoutConfig{
			Connect: 3 * time.Second,
			Read:    7 * time.Second,
		},
	}

	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, err
	}

	if config.URL == "" {
		return nil, fmt.Errorf("url is required")
	}

	return config, nil
}

func (c *Config) SetupCookies(jar *cookiejar.Jar) error {
	targetURL, err := url.Parse(c.URL)
	if err != nil {
		return err
	}

	var cookies []*http.Cookie

	// Add cookies from config
	for _, cookie := range c.Cookies {
		cookies = append(cookies, &http.Cookie{
			Name:  cookie.Key,
			Value: cookie.Value,
		})
	}

	// Add cookies from file
	if c.CookieFile != "" {
		fileCookies, err := loadCookiesFromFile(c.CookieFile)
		if err != nil {
			return err
		}
		cookies = append(cookies, fileCookies...)
	}

	if len(cookies) > 0 {
		jar.SetCookies(targetURL, cookies)
	}

	return nil
}

func loadCookiesFromFile(path string) ([]*http.Cookie, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var cookies []*http.Cookie
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 7 {
			continue
		}

		cookies = append(cookies, &http.Cookie{
			Name:  fields[5],
			Value: fields[6],
		})
	}

	return cookies, scanner.Err()
}

func (c *Config) WriteLog(requestedAt time.Time, statusCode int, duration time.Duration) error {
	if c.Log == nil {
		return nil
	}

	data := struct {
		RequestedAt string
		StatusCode  int
		Duration    time.Duration
	}{
		RequestedAt: requestedAt.Format(time.RFC3339),
		StatusCode:  statusCode,
		Duration:    duration,
	}

	// Parse log path template
	pathTmpl, err := template.New("path").Parse(c.Log.Path)
	if err != nil {
		return err
	}

	var pathBuf bytes.Buffer
	if err := pathTmpl.Execute(&pathBuf, map[string]string{
		"ymdhms": time.Now().Format("20060102150405"),
	}); err != nil {
		return err
	}

	// Parse log format template
	formatTmpl, err := template.New("format").Parse(c.Log.Format)
	if err != nil {
		return err
	}

	var formatBuf bytes.Buffer
	if err := formatTmpl.Execute(&formatBuf, data); err != nil {
		return err
	}

	// Open log file in append mode
	f, err := os.OpenFile(pathBuf.String(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// Write log entry
	if _, err := fmt.Fprintln(f, formatBuf.String()); err != nil {
		return err
	}

	return nil
}
