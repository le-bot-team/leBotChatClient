# leBot (乐宝) AI Robot - Chat Client

## Project Overview

This is the **leBotChatClient** repository, one of three repositories in the "leBot (乐宝) AI Robot" project:

| Repository | Description | Tech Stack |
|---|---|---|
| **leBotChatClient** (this repo) | Embedded voice chat client running on the robot | Go 1.21, PortAudio, gorilla/websocket |
| **le-bot-backend** | Backend server interfacing with both this client and the web frontend | - |
| **le-bot-frontend** | Web frontend for device management, user management, and data analytics | - |

## Hardware & Embedded Environment

- **Platform**: TinaLinux (embedded Linux) on the leBot robot hardware
- **Architecture**: ARM v7 (linux/arm/v7), cross-compiled via Docker toolchain
- **Audio**: PortAudio for audio capture and playback
- **Wake Word Detection**: An embedded ASR module on the robot detects the wake phrase "你好乐宝" (Hello LeBot) and triggers a **falling edge signal on GPIO 200** (PG8)

## External Services (Volcengine)

The backend uses two Volcengine (火山引擎) APIs:

- **ASR API** (Speech-to-Text): https://www.volcengine.com/docs/6561/1354869?lang=zh
- **TTS API** (Text-to-Speech): https://www.volcengine.com/docs/6561/1329505?lang=zh

## Project Structure

```
leBotChatClient/
├── cmd/                        # Application entry point
│   ├── main.go                 # CLI flag parsing (-mode gpio|stdin|file), signal handling
│   └── app.go                  # Core application logic, state machine, MessageHandler impl
├── internal/                   # Internal packages
│   ├── config/
│   │   └── config.go           # Config structs and defaults (env: ACCESS_TOKEN, DEBUG, WEBSOCKET_URL)
│   ├── websocket/
│   │   ├── client.go           # WebSocket client with auto-reconnect, heartbeat, message routing
│   │   └── types.go            # Request/response type definitions for all WebSocket actions
│   ├── audio/
│   │   ├── recorder.go         # PortAudio-based streaming audio recorder
│   │   └── player.go           # Ring buffer-based streaming audio player
│   └── control/
│       ├── gpio.go             # GPIO falling-edge detection for wake word trigger
│       ├── monitor.go          # File-based control (/tmp/chat-control)
│       └── stdin.go            # Standard input control for development
├── pkg/                        # Public reusable packages
│   ├── buffer/
│   │   └── ring.go             # Thread-safe ring buffer with atomic operations
│   └── utils/
│       └── audio.go            # Audio utilities (IsSilent, CalculateRMS, GenerateRequestID, etc.)
├── build/                      # Cross-compilation build system
│   ├── Dockerfile              # Docker build image for ARM cross-compilation
│   ├── build_toolchain.sh      # Toolchain setup script
│   ├── build_dependencies.sh   # Dependency build script (PortAudio, etc.)
│   └── build_app.sh            # Application build script
├── .devcontainer/              # Dev container config (GoLand, ARM cross-compile env)
├── compose.yaml                # Docker Compose for cross-compilation pipeline
├── go.mod                      # Module: websocket_client_chat
└── go.sum
```

## Architecture & State Machine

### Control Modes

Selectable via `-mode` CLI flag:

1. **GPIO mode** (default, production): Continuous recording with wake word detection via GPIO 200
2. **stdin mode** (development): Manual control via terminal input (`1`/`start`, `2`/`stop`, `q`/`quit`)
3. **file mode**: Control via writing to `/tmp/chat-control`

### State Machine (GPIO mode)

```
Sleeping (0) ──GPIO wake──> WaitingResponse (1) ──server responds──> Active (2)
    ^                            |                                       |
    |                     30s timeout                             silence detected
    |                            |                                       |
    └────────────────────────────┴───────────────────────────────────────┘
```

- **Sleeping**: Continuously recording into a circular wake buffer; waiting for GPIO trigger
- **WaitingResponse**: Wake detected, buffered audio sent to server, waiting for AI response (30s timeout)
- **Active**: Conversation in progress, streaming audio I/O, silence detection running

### Silence Detection

- Runs every 2 seconds during Active state
- RMS threshold: 200.0, silence ratio: 0.95
- 3 seconds of silence triggers transition back to Sleeping

### Smart Interruption

- New user speech (detected via `outputTextComplete` with `role: "user"`, text length >= 2) stops current playback
- GPIO wake during Active state interrupts current session

## WebSocket Protocol

### Client -> Server

| Action | Description |
|---|---|
| `inputAudioStream` | Streaming audio chunk (base64 encoded, 200ms/chunk) |
| `inputAudioComplete` | Audio recording complete signal |
| `inputWakeAudio` | Wake buffer audio (sent on GPIO trigger) |
| `cancelOutput` | Cancel current AI output |
| `clearContext` | Clear conversation context |
| `updateConfig` | Update device configuration |

### Server -> Client

| Action | Description |
|---|---|
| `establishConnection` | Connection established confirmation |
| `outputAudioStream` | Streaming audio response chunk |
| `outputAudioComplete` | Audio response complete |
| `outputTextStream` | Streaming text response chunk |
| `outputTextComplete` | Text response complete |
| `chatComplete` | Full chat turn complete (may contain errors) |
| `cancelOutput` | Output cancelled (cancelType: "manual" or "voice") |
| `updateConfig` | Config update confirmation |

## Build & Deployment

- **Cross-compilation**: ARM v7 target via Docker (see `compose.yaml`)
- **Build command**: `docker compose run builder` or use the dev container
- **Environment variables**: `CGO_ENABLED=1`, `GOOS=linux`, `GOARCH=arm`, `GOARM=7`
- **Debug mode**: Set `DEBUG=1` environment variable for verbose logging

## Key Configuration Defaults

| Category | Parameter | Default |
|---|---|---|
| Audio | Output sample rate | 16000 Hz |
| Audio | Capture sample rate | 48000 Hz |
| Audio | Channels / Bit depth | 1 (mono) / 16-bit |
| Audio | Chunk duration | 200ms |
| WebSocket | Reconnect delay | 5s |
| WebSocket | Ping interval | 30s |
| GPIO | Pin number | 200 (PG8) |
| Wake | Buffer duration | 8s |
| Device | Voice ID | `xiaole` |
| Device | Timezone | `Asia/Shanghai` |

## Code Conventions

- Follow standard Go project layout (`cmd/`, `internal/`, `pkg/`)
- Interface-driven design for testability (e.g., `MessageHandler`, `control.Handler`, `control.GpioHandler`, `audio.Handler`)
- Concurrency managed via `context.Context`, `sync.Mutex`, and `sync/atomic`
- All code, comments, and documentation in English

## Maintaining This Document

IMPORTANT: This `AGENTS.md` documents the architecture of all three repositories in the leBot project (leBotChatClient, le-bot-backend, le-bot-frontend). After every code change or file structure modification, check whether this document needs to be updated. Specifically:

- When files or directories are added, renamed, moved, or deleted, update the **Project Structure** section
- When the state machine, control modes, or WebSocket protocol changes, update the **Architecture & State Machine** section
- When configuration defaults or structs change, update the **Key Configuration Defaults** section
- When new interfaces, design patterns, or conventions are introduced, update the **Code Conventions** section
- When build/deployment processes change, update the **Build & Deployment** section
- When external service integrations change, update the **External Services** section
