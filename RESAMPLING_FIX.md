# Audio Resampling Fix

## Problem
The application was configured to record audio at 16000 Hz, but the hardware audio system (PipeWire/PulseAudio) operates natively at 48000 Hz. This caused a `paInvalidSampleRate` error when trying to open the audio stream.

## Latest Updates (2025-12-04)

### Update 2: Critical Fix - Device Default Sample Rate Priority
**Problem**: Even after tracking actual sample rate, embedded devices still produced accelerated audio (3x faster).

**Root Cause**:
- Embedded device's default sample rate: **16000 Hz** (matches target output rate!)
- Config tried to force: **48000 Hz**
- PortAudio may silently accept 48kHz request but actually use 16kHz
- Code assumed 48kHz was used → resampled "16000→16000" as if it was "48000→16000"
- Result: 3:1 compression → audio plays 3x faster

**Example from logs**:
```
选中录音设备: default (默认采样率: 16000 Hz)
使用已知的实际采样率: 48000 Hz  ← Wrong assumption!
重采样: 48000 Hz -> 16000 Hz      ← Compressing already-16kHz audio!
```

**The Fix**:
Changed sample rate selection strategy with smart priority order:

1. **If device default rate == target rate (e.g., both 16kHz)**: Use device default directly → **no resampling needed!**
2. **Otherwise**: Try configured capture rate (48kHz)
3. **If that fails**: Fall back to device default rate

This ensures optimal behavior for all devices:
- ✅ PC with 48kHz device → capture at 48kHz, resample to 16kHz (quality improvement)
- ✅ Embedded with 16kHz device → capture at 16kHz directly, no resampling (perfect match, no CPU waste!)
- ✅ Any device → always uses the correct actual sample rate for any resampling operations

### Update 1: Actual Sample Rate Tracking
Fixed bug where `TestRecording()` always used configured `CaptureSampleRate` for resampling, ignoring what the device actually opened with.

**The Fix**: Added `actualCaptureSampleRate` field to track real device sample rate.

## Solution
Implemented audio resampling to bridge the gap between hardware capabilities and server requirements:

1. **Capture at hardware native rate (48000 Hz)** - Record audio at the rate your hardware supports
2. **Resample to server required rate (16000 Hz)** - Convert the audio to the rate the server expects
3. **Send resampled data** - Transmit the 16kHz audio to the server

## Changes Made

### 1. Configuration (`internal/config/config.go`)
- Added `CaptureSampleRate` field to `AudioConfig` (48000 Hz)
- Kept `SampleRate` as the output/server rate (16000 Hz)
- Updated chunk calculations to be based on output rate

### 2. Audio Utils (`pkg/utils/audio.go`)
- Added `ResampleAudio()` function that uses linear interpolation
- Converts audio samples from one sample rate to another
- Efficient algorithm suitable for speech audio

### 3. Recorder (`internal/audio/recorder.go`)
- Added `resampleBuffer` to store resampled data
- Updated audio stream to use `CaptureSampleRate` (48000 Hz)
- Modified `audioCallback` to:
  - Accumulate captured samples (at 48kHz)
  - Resample to target rate (16kHz) when enough data is available
  - Send chunks at the correct output size (3200 samples @ 16kHz for 200ms)
- Updated `StopRecording` to resample any remaining data

## Technical Details

### Resampling Ratio
- **Capture Rate**: 48000 Hz
- **Output Rate**: 16000 Hz  
- **Ratio**: 3:1 (downsampling by factor of 3)

### Chunk Sizes
- **Capture chunk** (200ms @ 48kHz): 9600 samples
- **Output chunk** (200ms @ 16kHz): 3200 samples
- **Capture byte size**: 19200 bytes
- **Output byte size**: 6400 bytes

### Resampling Method
Linear interpolation is used for resampling:
- Fast and efficient
- Good quality for speech
- No external dependencies required

## Testing
Build and run the application:
```bash
go build -o leBotChatClient ./cmd/
./leBotChatClient
```

The audio recorder should now:
1. Successfully open the audio stream at 48kHz
2. Capture audio without sample rate errors
3. Automatically resample to 16kHz
4. Send correctly formatted chunks to the server

## Performance Impact
- Minimal CPU overhead (linear interpolation is very fast)
- Slightly increased memory usage for resampling buffers
- No noticeable latency added to the audio pipeline

