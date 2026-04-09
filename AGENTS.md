# leBot (乐宝) AI Robot - Chat Client

## Project Overview

This is the **leBotChatClient** repository, one of four repositories in the "leBot (乐宝) AI Robot" project:

| Repository | Description | Tech Stack |
|---|---|---|
| **leBotChatClient** (this repo) | Embedded voice chat client running on the robot | Go 1.21, PortAudio, gorilla/websocket, BurntSushi/toml |
| **le-bot-backend** | Backend server interfacing with both this client and the web frontend | Bun, Elysia, TypeScript |
| **le-bot-frontend** | Web frontend for device management, user management, and data analytics | Vue 3, Quasar |
| **le_bot_vpr** | Voice Print Recognition microservice for speaker identification | Python 3.14, FastAPI, PyTorch, Elasticsearch |

## Hardware & Embedded Environment

- **Platform**: TinaLinux (embedded Linux) on the leBot robot hardware
- **Architecture**: ARM v7 (linux/arm/v7), cross-compiled via Docker toolchain
- **Audio**: PortAudio for audio capture and playback
- **Wake Word Detection**: An embedded ASR module on the robot detects the wake phrase "你好乐宝" (Hello LeBot) and triggers a **falling edge signal on GPIO 200** (PG8)

## le_bot_vpr (Voice Print Recognition Microservice)

A FastAPI microservice that manages multi-tenant voiceprint registration and recognition backed by Elasticsearch vector search.

### Tech Stack

| Category | Technology |
|---|---|
| Language | Python 3.14 |
| Web Framework | FastAPI 0.128 + Uvicorn |
| Deep Learning | PyTorch 2.9.1 + TorchAudio 2.9.1 |
| Speaker Embedding Model | ERes2Net (from VoiceprintRecognition-Pytorch, git submodule) |
| Feature Extraction | Fbank (80 mel bins, 16kHz sample rate) |
| Vector Database | Elasticsearch 9.2.4 (native kNN / HNSW) |

### Speaker Embedding

| Parameter | Value |
|---|---|
| Model Architecture | ERes2Net (Enhanced Res2Net) |
| Embedding Dimension | 192 |
| Inference Device | CPU (default) |
| Audio Constraints | Min 0.3s, max 3s, 16kHz, dB normalized to -20dB |
| Similarity Metric | Cosine similarity |
| ANN Index | HNSW (M=16, ef_construction=100) |

### Data Hierarchy: User → Person → Voice

```
User (user_id)           # Multi-tenant isolation unit; each user gets a separate ES index
 └── Person (person_id)  # A physical person, identified by aggregating voices under same person_id
      ├── Voice (voice_id)   # Individual voice record with 192-dim feature_vector
      ├── Voice (voice_id)
      └── Voice (voice_id)
```

- **User** maps to an ES index (`voice_features_user_{user_id}`), providing tenant isolation
- **Person** is a virtual entity derived by aggregating Voice documents sharing the same `person_id`
- **Voice** is the actual ES document storing `feature_vector`, `person_id`, `is_temporal`, `expire_date`, etc.
- Auto-match on registration: kNN search (threshold 0.85) matches existing persons; creates new person (UUID v7) if no match
- Temporal voices: `is_temporal=True` voices expire after 7 days with batch cleanup

### Key API Endpoints

Base URL: `/api/v1/vpr`

| Method | Path | Description |
|---|---|---|
| `POST` | `/users/{user_id}/recognize` | Recognize speaker via kNN search, returns best matching person |
| `POST` | `/users/{user_id}/register` | Register voice (extract embedding, auto-match or create new person) |
| `GET` | `/users/{user_id}/persons` | List all persons under a user |
| `DELETE` | `/users/{user_id}/persons/{person_id}` | Delete person and all voices |
| `POST` | `/users/{user_id}/persons/{person_id}/voices/add` | Add voice to existing person |

Audio input: Base64 encoded string (data URI prefix supported), max 20MB, formats: wav, mp3, m4a, ogg.

## External Services (Volcengine)

The backend uses two Volcengine (火山引擎) APIs:

- **ASR API** (Speech-to-Text): https://www.volcengine.com/docs/6561/1354869?lang=zh
- **TTS API** (Text-to-Speech): https://www.volcengine.com/docs/6561/1329505?lang=zh

## Project Structure

```
leBotChatClient/
├── cmd/                            # Application entry point
│   ├── main.go                     # CLI flag parsing (-mode gpio|stdin|file), signal handling
│   └── app.go                      # Core application logic, state machine, MessageHandler impl
├── internal/                       # Internal packages
│   ├── config/
│   │   └── config.go               # Config structs and defaults (reads config.toml via BurntSushi/toml)
│   ├── websocket/
│   │   ├── client.go               # WebSocket client with auto-reconnect, heartbeat, message routing
│   │   └── types.go                # Request/response type definitions for all WebSocket actions
│   ├── audio/
│   │   ├── recorder.go             # PortAudio-based streaming audio recorder with device priority selection
│   │   ├── player.go               # Ring buffer-based streaming audio player with interruption support
│   │   ├── suppress_default.go     # No-op suppressCOutput() for non-ARM/Linux builds
│   │   └── suppress_linux_arm.go   # Suppresses ALSA/PortAudio C stderr on ARM Linux via fd redirect
│   └── control/
│       ├── gpio.go                 # GPIO falling-edge detection for wake word trigger
│       ├── monitor.go              # File-based control (/tmp/chat-control)
│       └── stdin.go                # Standard input control for development
├── pkg/                            # Public reusable packages
│   ├── buffer/
│   │   └── ring.go                 # Lock-free SPSC ring buffer with atomic operations (ARM32-safe)
│   └── utils/
│       └── audio.go                # Audio utilities (IsSilent, CalculateRMS, GenerateRequestID, etc.)
├── build/                          # Cross-compilation build system
│   ├── Dockerfile                  # Docker build image with OpenWrt ARM musl cross-compiler toolchain
│   ├── build_toolchain.sh          # Toolchain setup script (extracts OpenWrt ARM toolchain)
│   ├── build_dependencies.sh       # Dependency build script (ALSA-lib 1.2.8, PortAudio v190700)
│   ├── build_app.sh                # Application build script (outputs to dist/)
│   └── README.md                   # Docker build guide
├── dist/                           # Build output directory (gitignored)
├── .devcontainer/                  # Dev container config (GoLand, ARM cross-compile env)
├── config.toml                     # Runtime config (access_token, debug, websocket_url)
├── compose.yaml                    # Docker Compose for cross-compilation pipeline
├── docker_build_app.sh/.bat        # Shortcut: docker compose run --rm builder
├── docker_clear_cache.sh/.bat      # Shortcut: docker compose down --rmi all -v
├── go.mod                          # Module: websocket_client_chat
└── go.sum
```

## Architecture & State Machine

### Control Modes

Selectable via `-mode` CLI flag:

1. **GPIO mode** (default, production): Continuous recording with wake word detection via GPIO 200
2. **stdin mode** (development): Manual control via terminal input (`1`/`start`, `2`/`stop`, `3`/`test`, `q`/`quit`)
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
- 3 seconds of silence triggers transition back to WaitingResponse (NOT Sleeping)
- `inputAudioComplete` is NOT sent on Active → WaitingResponse; audio keeps streaming for interrupt detection
- `inputAudioComplete` is only sent on true session end (30s timeout → Sleeping)

### Multi-Person Conversation Design

The robot is designed for **multi-person scenarios** where different people may interact with it during a single session. Voice Print Recognition (VPR) is used to identify the current speaker and switch conversation context accordingly.

#### Speaker Tracking

- Each conversation turn tracks a `lastPersonId` (the person currently speaking)
- The backend's Chat API uses `personId` to load the correct person's profile and conversation history
- When a different person speaks, the backend updates `lastPersonId` and the Chat API switches context

#### End-to-End Conversation Turn Flow (GPIO mode)

A complete conversation turn consists of the following stages:

```
1. Client: GPIO wake → capture wake buffer → send inputWakeAudio to backend
2. Backend: Receive wake audio → run ASR + VPR in parallel:
   - ASR: Recognize wake phrase (e.g., "你好，乐宝。")
   - VPR: Identify speaker from wake audio → set lastPersonId
   - If VPR fails (new speaker): register new voice print (temporal)
3. Backend: Send wake response (personalized greeting via Wake API + TTS) → stream outputAudioStream to client
4. Client: Play wake response audio → on outputAudioComplete → transition to Active state
5. Client (Active): Stream inputAudioStream continuously to backend
6. Backend: ASR recognizes user speech → VPR identifies speaker → update lastPersonId if changed
7. Backend: Send recognized text to Chat API (with personId) → receive AI response → TTS → stream outputAudioStream
8. Client: Play response audio → on outputAudioComplete → resume listening
9. Repeat steps 5-8 until silence detected → WaitingResponse → 30s timeout → Sleeping
```

#### Interruption Rules

Interruption is only allowed under strict conditions to prevent cross-person interference:

1. **Same-person interruption (voice interrupt)**:
   - During an ongoing conversation turn (steps 6-8 above), if the backend detects a new ASR utterance
   - VPR must verify the speaker is the **same person** (`personId` matches `lastPersonId`)
   - If confirmed: cancel current TTS output, process new utterance immediately
   - If different person: the interruption is **rejected**; the different person must wait

2. **Wake word interruption (GPIO interrupt)**:
   - Any person can say "你好乐宝" at any time to trigger a GPIO wake
   - This forcefully interrupts the entire current session regardless of speaker identity
   - Client sends `cancelOutput` → starts a completely new session from step 1

3. **Person switching (between turns)**:
   - After a complete conversation turn finishes (client finishes playing response audio)
   - A different person can speak and will be identified by VPR
   - The backend updates `lastPersonId` and switches Chat API context to the new person

### Smart Interruption

- New user speech during Active state triggers VPR verification on the backend
- Backend sends `outputTextStream` with `role: "user"` back to client; client stops playback if text length >= 2
- Backend may also send `cancelOutput` with `cancelType: "voice"` to forcefully cancel output
- GPIO wake during Active/WaitingResponse state interrupts current session via `cancelOutput` with `cancelType: "manual"`

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
- **Toolchain**: OpenWrt ARM musl cross-compiler (`arm-openwrt-linux-muslgnueabi-gcc`)
- **Build dependencies**: ALSA-lib 1.2.8, PortAudio v190700 (cross-compiled for ARM)
- **Build command**: `docker compose run builder` or use the dev container
- **Build output**: `dist/chat_client_openwrt` (ARM ELF binary) + `dist/libportaudio.so.2`
- **Shortcut scripts**: `docker_build_app.sh/.bat` (build), `docker_clear_cache.sh/.bat` (cleanup)
- **Environment variables**: `CGO_ENABLED=1`, `GOOS=linux`, `GOARCH=arm`, `GOARM=7`
- **Configuration**: Place `config.toml` next to the executable or in the working directory
- **Debug mode**: Set `debug = true` in `config.toml` for verbose logging

## Key Configuration Defaults

| Category | Parameter | Default |
|---|---|---|
| Audio | Output sample rate | 16000 Hz |
| Audio | Capture sample rate | 48000 Hz |
| Audio | Channels / Bit depth | 1 (mono) / 16-bit |
| Audio | Chunk duration | 200ms |
| WebSocket | Reconnect delay | 5s (exponential backoff, max 160s) |
| WebSocket | Ping interval | 30s |
| WebSocket | Write / Read timeout | 10s / 60s |
| WebSocket | Max message size | 1 MB |
| GPIO | Pin number | 200 (PG8) |
| GPIO | Poll interval | 100ms |
| Wake | Buffer duration | 8s |
| Wake | Silence check interval | 2s |
| Wake | Silence RMS threshold | 200.0 |
| Wake | Silence ratio threshold | 0.95 |
| Wake | Silence buffer duration | 3s |
| Wake | WaitingResponse timeout | 30s (hardcoded) |
| Wake | Cancel cooldown | 300ms |
| Device | Serial number | `DEV-001` |
| Device | Voice ID | `xiaole` |
| Device | Timezone | `Asia/Shanghai` |

## Code Conventions

- Follow standard Go project layout (`cmd/`, `internal/`, `pkg/`)
- Interface-driven design for testability (e.g., `MessageHandler`, `control.Handler`, `control.GpioHandler`, `audio.Handler`)
- Concurrency managed via `context.Context`, `sync.Mutex`, and `sync/atomic`
- Lock-free SPSC ring buffer with `atomic.Int64` cursors (ARM32 8-byte alignment safe)
- Build-tag separation for platform-specific code (e.g., `suppress_linux_arm.go` / `suppress_default.go`)
- Exponential backoff for WebSocket reconnection (base 5s, cap 160s, resets on success)
- All code, comments, and documentation in English

## Maintaining This Document

IMPORTANT: This `AGENTS.md` documents the architecture of all four repositories in the leBot project (leBotChatClient, le-bot-backend, le-bot-frontend, le_bot_vpr). After every code change or file structure modification, check whether this document needs to be updated. Specifically:

- When files or directories are added, renamed, moved, or deleted, update the **Project Structure** section
- When the state machine, control modes, or WebSocket protocol changes, update the **Architecture & State Machine** section
- When configuration defaults or structs change, update the **Key Configuration Defaults** section
- When new interfaces, design patterns, or conventions are introduced, update the **Code Conventions** section
- When build/deployment processes change, update the **Build & Deployment** section
- When external service integrations change, update the **External Services** section
