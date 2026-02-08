# Voice Intercom System - Refactored Version

## Project Overview

This is a real-time voice intercom system client based on WebSocket, supporting:
- Real-time voice recording and streaming
- Streaming audio playback
- Smart interruption (users can interrupt AI responses at any time)
- Text stream receiving and processing
- Auto-reconnection and heartbeat detection
- Multiple control modes (standard input / file control)

## Quick Start

### Install Dependencies

```bash
# Install PortAudio (audio library)
# Ubuntu/Debian
sudo apt-get install portaudio19-dev

# macOS
brew install portaudio

# Install Go dependencies
go mod download
```

### Run

```bash
# Development mode (using standard input control)
go run ./cmd

# Or build and run
go build -o chat-client ./cmd
./chat-client
```

### Usage

#### Standard Input Control Mode (Default)
```
Enter command:
  1 or start - Start recording
  2 or stop  - Stop recording and send
  q or quit  - Exit program
```

#### File Control Mode
Modify configuration to set `UseStdin` to `false`, then:
```bash
# Start recording
echo 1 > /tmp/chat-control

# Stop recording
echo 2 > /tmp/chat-control
```

The optimized project adopts a clean modular architecture following Go's standard project layout:

```
leBotChatClient/
├── cmd/                    # Application entry point
│   ├── main.go            # Main function
│   └── app.go             # Application core logic
├── internal/              # Internal packages (not exposed)
│   ├── config/            # Configuration management
│   │   └── config.go      # Config structures and defaults
│   ├── websocket/         # WebSocket client
│   │   ├── client.go      # WebSocket client implementation
│   │   └── types.go       # Message type definitions
│   ├── audio/             # Audio processing
│   │   ├── recorder.go    # Audio recorder
│   │   └── player.go      # Audio player
│   └── control/           # Controllers
│       └── monitor.go     # File monitor
├── pkg/                   # Public packages (externally accessible)
│   ├── buffer/            # Buffer utilities
│   │   └── ring.go        # Ring buffer
│   └── utils/             # Utility functions
│       └── audio.go       # Audio processing utilities
├── go.mod                 # Go module definition
├── go.sum                 # Dependency checksum
├── readme.md              # Project documentation
└── websocket_client_chat.go # Original file (can be deleted)
```

## Architecture Design Advantages

### 1. Modular Design
- **Single Responsibility Principle**: Each module handles only one specific function
- **Interface-Driven**: Uses interfaces to define component interactions, facilitating testing and extension
- **Dependency Injection**: Injects dependencies through constructors, reducing coupling

### 2. Clear Package Structure
- **cmd/**: Application entry point, contains main function and core application logic
- **internal/**: Internal implementation, not exposed externally
- **pkg/**: Reusable public packages, can be referenced by other projects

### 3. Configuration Management
- Centralized configuration management with default config support
- Structured configuration, easy to maintain and extend
- Type-safe configuration items

### 4. Error Handling
- Unified error handling mechanism
- Context propagation and graceful shutdown
- Detailed logging

### 5. Concurrency Safety
- Uses sync package to ensure concurrency safety
- Context controls goroutine lifecycle
- Atomic operations for shared state management

## Key Features

### Audio Processing
- **Streaming Recording**: Records audio in real-time and sends in chunks (200ms/chunk)
- **Streaming Playback**: Starts playback immediately upon receiving audio, reducing latency
- **Interruption Support**: Users can interrupt AI responses at any time, system automatically stops playback and clears buffer

### WebSocket Communication
- **Full Protocol Support**: Aligned with frontend implementation, supports all message types
  - `inputAudioStream` / `inputAudioComplete` - Audio input
  - `outputAudioStream` / `outputAudioComplete` - Audio output
  - `outputTextStream` / `outputTextComplete` - Text stream
  - `chatComplete` - Chat completion
  - `updateConfig` - Configuration update
  - `cancelOutput` - Cancel output
  - `clearContext` - Clear context
- **Auto-Reconnection**: Automatically reconnects on disconnection without manual intervention
- **Heartbeat Detection**: Keeps connection alive, detects network issues promptly

### Smart Interruption Logic
The system automatically detects new user messages:
1. Listens for `outputTextComplete` messages
2. When receiving a user message (`role: "user"` with text length >= 2)
3. Automatically stops currently playing audio
4. Clears audio buffer
5. Prepares to receive new response

### Flexible Configuration
- Supports custom sample rate, channels, and other audio parameters
- Configurable WebSocket connection parameters
- Supports device info and location configuration
- Supports timezone configuration (e.g., "Asia/Shanghai")

## Project Structure

### App (Application Core)
- Unified management of all component lifecycles
- Implements message passing interfaces between components
- Handles application startup and shutdown

### WebSocket Client
- Auto-reconnection mechanism
- Heartbeat detection
- Typed message handling
- Concurrency-safe message sending

### Audio Recorder
- PortAudio-based audio recording
- Streaming audio processing
- WAV format conversion support
- Asynchronous audio data processing

### Audio Player
- Ring buffer-based audio playback
- Streaming playback support
- Automatic playback state management
- Multiple stop condition detection

### Controller
- File monitoring mechanism
- Type-safe command handling
- Asynchronous command execution

### Ring Buffer
- Thread-safe ring buffer
- Atomic operation performance optimization
- Close state detection support

## Running

```bash
# Option 1: Run from cmd directory
cd cmd
go run .

# Option 2: Run from root directory
go run ./cmd

# Option 3: Build and run
go build -o chat-client ./cmd
./chat-client
```

## Usage

After system startup, operate via the control file:

```bash
# Start recording
echo 1 > /tmp/chat-control

# Stop recording and send
echo 2 > /tmp/chat-control
```

## Configuration

All configurations are defined in `internal/config/config.go`, including:

- **Audio Config**: Sample rate, channels, buffer size, etc.
- **WebSocket Config**: Connection URL, reconnect interval, timeout settings, etc.
- **Control Config**: Control file path, monitor interval, etc.
- **Device Config**: Device serial number, voice settings, etc.

## Extensibility

The optimized architecture supports the following extensions:

1. **New Audio Formats**: Add new conversion functions in the utils package
2. **New Transport Protocols**: Implement the MessageHandler interface
3. **New Control Methods**: Implement the Handler interface
4. **Config File Support**: Extend config package to support JSON/YAML configuration
5. **Plugin System**: Interface-based plugin architecture

## Testing Support

Thanks to the interface-driven design, each component can be tested independently:

- Mock implementations of interfaces for unit testing
- Dependency injection for integration testing
- Clear module boundaries for performance testing

## Performance Optimization

1. **Memory Optimization**: Buffer reuse, reduced memory allocation
2. **Concurrency Optimization**: Asynchronous processing, avoid blocking
3. **Network Optimization**: Connection pool and reconnection mechanism
4. **Audio Optimization**: Streaming processing, reduced latency
