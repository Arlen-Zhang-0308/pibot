---
name: PiBot AI Assistant
overview: Build a Go-based AI assistant for Raspberry Pi with a unified AI provider interface, sandboxed command execution, file operations, and a web-based UI for interaction and configuration.
todos:
  - id: project-setup
    content: Initialize Go module, create directory structure, and add dependencies
    status: completed
  - id: config-system
    content: Implement YAML-based configuration with API keys and settings
    status: completed
  - id: ai-providers
    content: Build unified AI provider interface with OpenAI, Anthropic, Gemini, and Ollama implementations
    status: completed
  - id: command-executor
    content: Create sandboxed command executor with safe/dangerous command categorization
    status: completed
  - id: file-operations
    content: Implement file read/write/list/delete with path restrictions
    status: completed
  - id: api-router
    content: Set up HTTP router with REST endpoints and WebSocket handler
    status: completed
  - id: webui
    content: Build HTML/TypeScript frontend with chat interface and settings page
    status: completed
  - id: integration
    content: Wire all components together and test end-to-end
    status: completed
---

# PiBot - AI-Driven Raspberry Pi Assistant

## Architecture Overview

```mermaid
flowchart TB
    subgraph webui [WebUI]
        HTML[HTML Pages]
        TS[TypeScript Client]
    end
    
    subgraph backend [Go Backend]
        Router[HTTP Router]
        AIService[AI Service]
        CmdExecutor[Command Executor]
        FileOps[File Operations]
        WSHandler[WebSocket Handler]
    end
    
    subgraph providers [AI Providers]
        OpenAI[OpenAI]
        Anthropic[Anthropic]
        Google[Gemini]
        Ollama[Ollama]
    end
    
    HTML --> Router
    TS --> WSHandler
    Router --> AIService
    Router --> CmdExecutor
    Router --> FileOps
    AIService --> OpenAI
    AIService --> Anthropic
    AIService --> Google
    AIService --> Ollama
```

## Project Structure

```
pibot/
├── cmd/
│   └── pibot/
│       └── main.go              # Application entry point
├── internal/
│   ├── ai/
│   │   ├── provider.go          # Unified AI provider interface
│   │   ├── openai.go            # OpenAI implementation
│   │   ├── anthropic.go         # Anthropic implementation
│   │   ├── google.go            # Google Gemini implementation
│   │   └── ollama.go            # Ollama implementation
│   ├── executor/
│   │   ├── executor.go          # Command execution with sandboxing
│   │   └── sandbox.go           # Command validation and confirmation
│   ├── fileops/
│   │   └── fileops.go           # File read/write operations
│   ├── config/
│   │   └── config.go            # Configuration management
│   └── api/
│       ├── router.go            # HTTP router setup
│       ├── handlers.go          # API endpoint handlers
│       └── websocket.go         # WebSocket for real-time chat
├── web/
│   ├── static/
│   │   ├── index.html           # Main chat interface
│   │   ├── settings.html        # Settings/config page
│   │   ├── css/
│   │   │   └── style.css        # Styling
│   │   └── js/
│   │       └── app.ts           # TypeScript client code
│   └── embed.go                 # Embed static files
├── config.yaml                  # Configuration file
├── go.mod
├── go.sum
└── README.md
```

## Key Components

### 1. Unified AI Provider Interface

A common interface that all AI providers implement:

```go
type Provider interface {
    Name() string
    Chat(ctx context.Context, messages []Message) (string, error)
    StreamChat(ctx context.Context, messages []Message, ch chan<- string) error
}
```

Configuration allows switching between providers or setting a default.

### 2. Sandboxed Command Executor

Commands are categorized into:

- **Safe**: Execute immediately (ls, pwd, cat, echo, etc.)
- **Moderate**: Execute with logging (mkdir, cp, mv, etc.)
- **Dangerous**: Require confirmation via WebUI (rm -rf, sudo, etc.)
- **Blocked**: Never execute (format, dd, etc.)

### 3. File Operations

Restricted to a configurable base directory (default: `~/pibot-workspace`):

- Read files
- Write/create files
- List directory contents
- Delete files (with confirmation for non-empty directories)

### 4. WebUI

Simple, functional interface with:

- Chat interface for AI interaction
- Real-time streaming responses via WebSocket
- Command output display
- Settings page for API keys and provider selection
- Dark/light theme toggle

### 5. Configuration

YAML-based configuration for:

- AI provider API keys
- Default provider selection
- Allowed file operation paths
- Command sandbox rules
- Server port and host

## API Endpoints

| Method | Endpoint | Description |

|--------|----------|-------------|

| GET | `/` | Serve WebUI |

| GET | `/api/config` | Get current config (sans secrets) |

| POST | `/api/config` | Update configuration |

| POST | `/api/chat` | Send message to AI |

| WS | `/api/ws` | WebSocket for streaming |

| POST | `/api/exec` | Execute command |

| POST | `/api/exec/confirm` | Confirm dangerous command |

| GET | `/api/files` | List files in directory |

| GET | `/api/files/*path` | Read file content |

| POST | `/api/files/*path` | Write file content |

| DELETE | `/api/files/*path` | Delete file |

## Dependencies

- `github.com/gorilla/mux` - HTTP router
- `github.com/gorilla/websocket` - WebSocket support
- `gopkg.in/yaml.v3` - YAML config parsing
- `github.com/sashabaranov/go-openai` - OpenAI client
- `github.com/liushuangls/go-anthropic` - Anthropic client
- `github.com/google/generative-ai-go` - Google Gemini client

## Messaging (Future)

The architecture includes a `messaging` package placeholder for future iMessage/alternative messaging integration. This will be skipped in the initial implementation.