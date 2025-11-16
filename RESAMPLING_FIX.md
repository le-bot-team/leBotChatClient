# Audio Resampling Fix

## Problem
The application was configured to record audio at 16000 Hz, but the hardware audio system (PipeWire/PulseAudio) operates natively at 48000 Hz. This caused a `paInvalidSampleRate` error when trying to open the audio stream.

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

