# Dead Man's Switch

A Go-based tool that monitors for regular HTTP POST updates from another application. If updates stop, it sends notifications via configurable channels (email, Slack, Teams, Telegram, etc.).

## Features

- Monitors HTTP POST updates
- Sends notifications via multiple, configurable channels
- Configurable via `config.yaml` or environment variables
- Runs natively (Windows/Linux) or in Docker
- Multi-platform builds and Docker image publishing
- Extensible notification system
- Includes tests

## Quick Start

### 1. Configuration

Create a `config.yaml` file in the working directory. Example:

```yaml
listen_addr: ":8080"
timeout_seconds: 600
notification_channels:
  - type: email
    to: "user@example.com"
    smtp_server: "smtp.example.com"
    smtp_user: "user"
    smtp_pass: "pass"
  - type: slack
    webhook_url: "https://hooks.slack.com/services/..."
```

You can override any config value with environment variables (e.g., `LISTEN_ADDR`, `TIMEOUT_SECONDS`).

### 2. Running the Server

#### Native (Windows/Linux)

```sh
mkdir -p build
GOOS=linux  GOARCH=amd64 go build -o build/dead-mans-switch-linux-amd64 .
GOOS=linux  GOARCH=arm64 go build -o build/dead-mans-switch-linux-arm64 .
GOOS=windows GOARCH=amd64 go build -o build/dead-mans-switch-windows-amd64.exe .
./build/dead-mans-switch-linux-amd64 # or the appropriate binary for your OS
```

All local build artifacts are written to the `/build` directory.

#### Docker

```sh
docker build -t ghcr.io/crashlooping/dead-mans-switch/dead-mans-switch:latest .
docker run -v $(pwd)/config.yaml:/app/config.yaml -p 8080:8080 ghcr.io/crashlooping/dead-mans-switch/dead-mans-switch:latest
```

### 3. Sending Updates

Send a heartbeat (HTTP POST) to keep the switch alive:

#### curl

```sh
curl -X POST http://localhost:8080/heartbeat
```

#### wget

```sh
wget --method=POST http://localhost:8080/heartbeat
```

#### PowerShell

```powershell
Invoke-WebRequest -Uri http://localhost:8080/heartbeat -Method POST
```

## Building and CI/CD

- Binaries for Windows (x64) and Linux (x64, ARM) are built via GitHub Actions.
- Docker images are built and pushed to the GitHub Container Registry.

## Extending Notifications

Notification channels are pluggable. Add new types by implementing the `Notifier` interface.

## Tests

Run tests with:

```sh
go test ./...
```

---

For more details, see inline comments and documentation.
