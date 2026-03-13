# PiBot - AI-Driven Raspberry Pi Assistant

PiBot is an AI-powered assistant designed to run on a Raspberry Pi, providing a web interface for natural language interactions, command execution, and file management.

## Features

- **Multi-Provider AI Support**: OpenAI, Anthropic (Claude), Google Gemini, and Ollama (local models)
- **Sandboxed Command Execution**: Safe, moderate, dangerous, and blocked command categories
- **File Operations**: Read, write, list, and delete files within a restricted directory
- **Real-time WebUI**: Chat interface with streaming responses, terminal, and file browser
- **WebSocket Support**: Real-time communication for responsive interactions

## Quick Start

### Prerequisites

- Go 1.21 or later
- (Optional) Node.js for TypeScript compilation

### Installation

1. Clone or download the project
2. Navigate to the project directory
3. Build the application:

```bash
go mod tidy
go build -o pibot ./cmd/pibot
```

4. Run the application:

```bash
./pibot
```

5. Open your browser to `http://localhost:8080`

### Configuration

Edit `config.yaml` to configure:

- Server host and port
- AI provider API keys
- Default AI provider
- Command execution rules
- File operation base directory

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/` | Web UI |
| GET | `/api/config` | Get configuration (without secrets) |
| POST | `/api/config` | Update configuration |
| POST | `/api/chat` | Send chat message |
| GET | `/api/providers` | List available AI providers |
| WS | `/api/ws` | WebSocket for streaming |
| POST | `/api/exec` | Execute command |
| POST | `/api/exec/confirm/{id}` | Confirm dangerous command |
| POST | `/api/exec/cancel/{id}` | Cancel pending command |
| GET | `/api/exec/pending` | List pending commands |
| GET | `/api/files` | List files |
| GET | `/api/files/{path}` | Read file |
| POST | `/api/files/{path}` | Write file |
| DELETE | `/api/files/{path}` | Delete file |

## Command Safety Levels

- **Safe**: Execute immediately (ls, pwd, cat, echo, etc.)
- **Moderate**: Execute with logging (mkdir, cp, mv, etc.)
- **Dangerous**: Require confirmation (rm, sudo, apt, etc.)
- **Blocked**: Never execute (dd, mkfs, etc.)

## Project Structure

```
pibot/
├── cmd/pibot/          # Application entry point
├── internal/
│   ├── ai/             # AI provider implementations
│   ├── api/            # HTTP handlers and WebSocket
│   ├── config/         # Configuration management
│   ├── executor/       # Command execution with sandboxing
│   └── fileops/        # File operations
├── web/
│   └── static/         # Web UI files (HTML, CSS, JS)
├── config.yaml         # Configuration file
└── README.md
```

## Building for Raspberry Pi

Cross-compile for ARM:

```bash
GOOS=linux GOARCH=arm64 go build -o pibot-arm64 ./cmd/pibot
```

Or for 32-bit ARM:

```bash
GOOS=linux GOARCH=arm GOARM=7 go build -o pibot-arm ./cmd/pibot
```

## Running as a Service

Create a systemd service file `/etc/systemd/system/pibot.service`:

```ini
[Unit]
Description=PiBot AI Assistant
After=network.target

[Service]
Type=simple
User=pi
WorkingDirectory=/home/pi/pibot
ExecStart=/home/pi/pibot/pibot -config /home/pi/pibot/config.yaml
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl enable pibot
sudo systemctl start pibot
```

## License

MIT License
