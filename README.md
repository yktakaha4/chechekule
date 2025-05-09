# chechekule

A CLI tool that periodically sends HTTP requests to a specified URL and monitors the status code.

## Installation

```bash
go install github.com/yktakaha4/chechekule@latest
```

## Usage

### Basic Usage

```bash
chechekule https://example.com
```

### Using a Configuration File

```bash
chechekule -c config.yaml
```

Example configuration file:

```yaml
url: https://example.com
interval: 1s
timeout:
  connect: 3s
  read: 7s
follow_redirects: true
cookies:
  - key: _session
    value: abcde
cookie_file: "/tmp/cookie.txt" # curl cookie format
log:
  path: "/tmp/result{{.ymdhms}}.log"
  format: "{{.requestedAt}}\t{{.statusCode}}\t{{.duration}}"
```

Minimal configuration file:

```yaml
url: https://example.com
```

## Configuration Options

| Option | Description | Default |
|--------|-------------|---------|
| url | Target URL to monitor | Required |
| interval | Request interval | 1s |
| timeout.connect | Connection timeout | 3s |
| timeout.read | Read timeout | 7s |
| follow_redirects | Whether to follow HTTP redirects | true |
| cookies | Cookie settings | None |
| cookie_file | Path to curl format cookie file | None |
| log.path | Log file path (template available) | None |
| log.format | Log format (template available) | None |

### Log Template Variables

| Variable | Description |
|----------|-------------|
| {{.requestedAt}} | Request time (RFC3339 format) |
| {{.statusCode}} | HTTP status code |
| {{.duration}} | Request duration |
| {{.ymdhms}} | Current time for log filename (YYYYMMDDhhmmss format) |

## Development

### Requirements

- Go 1.22 or later
- Make

### Available Make Commands

```bash
# Install dependencies
make install

# Format code and run go vet
make fix

# Run tests
make test

# Build binary
make build

# Clean build artifacts
make clean
```

### Development Flow

1. Clone the repository
```bash
git clone https://github.com/yktakaha4/chechekule.git
cd chechekule
```

2. Install dependencies
```bash
make install
```

3. Make your changes

4. Format code and run static analysis
```bash
make fix
```

5. Run tests
```bash
make test
```

6. Build binary
```bash
make build
```

7. Run the built binary
```bash
./chechekule https://example.com
``` 