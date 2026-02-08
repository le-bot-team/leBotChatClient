# Debug Mode Usage Guide

## Overview

The program supports debug mode for outputting detailed debugging information. During normal operation, these debug logs will not be output to maintain good performance and clean log output.

## Enabling Debug Mode

Enable debug mode via the `DEBUG=1` environment variable:

```bash
DEBUG=1 ./leBotChatClient
```

## Debug Mode Output

### 1. Audio Recording

- **Device Initialization Info**
  ```
  Selected recording device: <device name> (Input channels: <count>, Default sample rate: <Hz>, Capture sample rate: <Hz>, Output sample rate: <Hz>)
  ```

- **Recording Start/Stop**
  ```
  Recording started (Device: <device name>, Capture sample rate: <Hz>, Output sample rate: <Hz>)
  Recording stopped
  ```

- **Audio Diagnostics** (per audio chunk)
  ```
  Audio diagnostics - RMS: <value>, Peak: <value>, Silence ratio: <percentage>%, Is silent: <true/false>
  ```
  
  - **RMS (Root Mean Square)**: Reflects the average energy level of the audio
    - Speech is typically in the 500-5000 range
    - If RMS < 200, it may be silence or the noise is too low
  
  - **Peak**: Maximum amplitude of the audio
    - Maximum value for 16-bit audio is 32767
    - Speech peak is typically in the 1000-20000 range
  
  - **Silence Ratio**: Percentage of silent samples
    - If > 95%, it is essentially silence
    - Normal speech is typically in the 20-60% range

- **Last Audio Chunk Diagnostics**
  ```
  Last audio chunk diagnostics - RMS: <value>, Peak: <value>, Silence ratio: <percentage>%, Is silent: <true/false>, Samples: <count>
  Sending last audio data: <sample count> samples
  ```

### 2. Audio Sending

- **Audio Data Chunk Sending**
  ```
  Sending WAV audio data chunk: <bytes> bytes
  ```

- **Completion Request Sending**
  ```
  Sending completion request (including last <bytes> bytes WAV audio)
  Sending completion request (no remaining audio)
  ```

### 3. WebSocket Communication

- **Connection Status**
  ```
  WebSocket connected successfully
  WebSocket connection disconnected
  ```

- **Configuration Update**
  ```
  Received config update response: Success=<true/false>, Message=<message>
  Update request sent
  Update response succeeded, starting streaming recording
  ```

### 4. Server Response

- **Audio Stream Response**
  ```
  Received audio stream response: ID=<request ID>, Session ID=<session ID>, Chat ID=<chat ID>
  Audio data size: <bytes> bytes
  Audio output complete: Session ID=<session ID>, Chat ID=<chat ID>
  ```

- **Text Stream Response**
  ```
  Received text stream: ID=<chat ID>, Role=<role>, Text=<content>
  Text output complete: ID=<chat ID>, Role=<role>, Text=<content>
  ```

- **Chat Complete**
  ```
  Chat complete: ID=<chat ID>, Success=<true/false>, Message=<message>
  ```

- **Interruption Logic**
  ```
  New user message detected, executing interruption logic
  ```

### 5. Audio Playback

- **Buffer Write**
  ```
  Buffer write: <bytes> bytes, Current buffer: <bytes> bytes
  ```

- **Playback Control**
  ```
  Starting playback...
  Received playback complete signal
  Audio buffer cleared
  Interrupting playback, stopping audio stream...
  Playback ended
  Audio playback started...
  ```

## Normal Mode vs Debug Mode

### Normal Mode (Default)
```bash
./leBotChatClient
```
- Only outputs essential logs (errors, warnings, critical states)
- Better performance
- Cleaner logs

### Debug Mode
```bash
DEBUG=1 ./leBotChatClient
```
- Outputs detailed debugging information
- Suitable for troubleshooting
- May have slight performance impact

## Use Cases

### When to Use Normal Mode
- Production environment
- No need for detailed logs
- Best performance required

### When to Use Debug Mode
- **Debugging Audio Issues**: Check if recording contains valid speech
- **Troubleshooting Communication Issues**: View WebSocket message exchanges
- **Analyzing Playback Issues**: Track audio playback flow
- **Development Testing**: Verify functionality works correctly

## Notes

1. Debug mode increases log output volume, which may affect terminal display speed
2. It is recommended to disable debug mode in production for optimal performance
3. Audio diagnostics are output every 200ms (based on chunk duration configuration)
