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

type FollowRedirectsConfig struct {
	Enabled  bool `yaml:"enabled"`
	MaxCount int  `yaml:"max_count"`
}

type StatusCodeAssert struct {
	Values []int  `yaml:"values"`
	Regex  string `yaml:"regex"`
}

type BodyAssert struct {
	Regex string `yaml:"regex"`
}

type AssertsConfig struct {
	StatusCode StatusCodeAssert `yaml:"status_code"`
	Body       BodyAssert       `yaml:"body"`
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
	URL             string                `yaml:"url"`
	Interval        time.Duration         `yaml:"interval"`
	Timeout         TimeoutConfig         `yaml:"timeout"`
	FollowRedirects FollowRedirectsConfig `yaml:"follow_redirects"`
	Asserts         AssertsConfig         `yaml:"asserts"`
	Cookies         []CookieConfig        `yaml:"cookies"`
	CookieFile      string                `yaml:"cookie_file"`
	Log             *LogConfig            `yaml:"log"`
	startTime       time.Time             // 開始時間を保持するフィールドを追加
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
		FollowRedirects: FollowRedirectsConfig{
			Enabled:  true,
			MaxCount: 10,
		},
		Asserts: AssertsConfig{
			StatusCode: StatusCodeAssert{
				Values: []int{200},
			},
		},
		startTime: time.Now(), // 開始時間を設定
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

	// Parse log path template
	pathTmpl, err := template.New("path").Parse(c.Log.Path)
	if err != nil {
		return fmt.Errorf("failed to parse path template: %w", err)
	}

	var pathBuf bytes.Buffer
	if err := pathTmpl.Execute(&pathBuf, map[string]string{
		"ymdhms": c.startTime.Format("20060102150405"), // 開始時間を使用
	}); err != nil {
		return fmt.Errorf("failed to execute path template: %w", err)
	}

	// Parse log format template
	formatTmpl, err := template.New("format").Option("missingkey=error").Parse(c.Log.Format)
	if err != nil {
		return fmt.Errorf("failed to parse format template: %w", err)
	}

	// 実際のデータでテンプレートを実行
	var formatBuf bytes.Buffer
	data := map[string]interface{}{
		"requestedAt": requestedAt.Format("2006-01-02T15:04:05.000Z07:00"),
		"statusCode":  statusCode,
		"duration":    duration,
	}
	if err := formatTmpl.Execute(&formatBuf, data); err != nil {
		return fmt.Errorf("failed to execute format template: %w", err)
	}

	// タブ文字のエスケープシーケンスを実際のタブ文字に変換
	logEntry := strings.ReplaceAll(formatBuf.String(), "\\t", "\t")

	// Open log file in append mode
	f, err := os.OpenFile(pathBuf.String(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer f.Close()

	// Write log entry
	if _, err := fmt.Fprintln(f, logEntry); err != nil {
		return fmt.Errorf("failed to write log: %w", err)
	}

	return nil
}
