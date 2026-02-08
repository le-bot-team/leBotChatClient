package audio

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"sync"
	"time"
	"websocket_client_chat/internal/config"
	"websocket_client_chat/pkg/utils"

	"github.com/gordonklaus/portaudio"
)

// AudioHandler defines the audio data handler interface
type AudioHandler interface {
	OnAudioChunk(requestID string, samples []int16, isLast bool)
	OnRecordingComplete(requestID string, samples []int16)
}

// Recorder is the audio recorder
type Recorder struct {
	config  *config.AudioConfig
	handler AudioHandler

	// Audio device state
	targetDevice            *portaudio.DeviceInfo
	isPortAudioInit         bool
	deviceInitialized       bool
	actualCaptureSampleRate int // Actual capture sample rate in use

	// Recording state
	isRecording bool
	stream      *portaudio.Stream
	mutex       sync.RWMutex

	// Streaming processing
	streamingRequestID string
	streamingBuffer    []int16
	resampleBuffer     []int16 // Buffer for resampling
	streamingMutex     sync.Mutex

	// Context control
	ctx    context.Context
	cancel context.CancelFunc

	// Debug mode
	enableDebug bool
}

// NewRecorder creates a new audio recorder
func NewRecorder(cfg *config.AudioConfig, handler AudioHandler, enableDebug bool) *Recorder {
	ctx, cancel := context.WithCancel(context.Background())

	return &Recorder{
		config:                  cfg,
		handler:                 handler,
		ctx:                     ctx,
		cancel:                  cancel,
		enableDebug:             enableDebug,
		actualCaptureSampleRate: cfg.CaptureSampleRate,
	}
}

// Initialize initializes the audio device
func (r *Recorder) Initialize() error {
	if r.deviceInitialized {
		if r.enableDebug {
			log.Println("Audio device already initialized, skipping duplicate initialization")
		}
		return nil
	}

	// Initialize PortAudio
	if !r.isPortAudioInit {
		if err := portaudio.Initialize(); err != nil {
			return fmt.Errorf("PortAudio initialization failed: %v", err)
		}
		r.isPortAudioInit = true
	}

	// Find audio device
	if err := r.findAudioDevice(); err != nil {
		if r.isPortAudioInit {
			portaudio.Terminate()
			r.isPortAudioInit = false
		}
		return err
	}

	r.deviceInitialized = true
	return nil
}

// findAudioDevice finds a suitable audio input device
func (r *Recorder) findAudioDevice() error {
	// First try to get all available devices (without relying on Host API)
	devices, err := portaudio.Devices()
	if err != nil {
		return fmt.Errorf("Failed to get device list: %v", err)
	}

	if r.enableDebug {
		log.Printf("PortAudio version: %s", portaudio.VersionText())
		log.Printf("Found %d audio devices", len(devices))
	}

	// List all available devices in debug mode (including unavailable ones, for debugging)
	if r.enableDebug {
		log.Println("=== All Audio Devices (Including Output Devices) ===")
		for i, dev := range devices {
			log.Printf("[%d] %s (Input channels: %d, Output channels: %d, Sample rate: %.0f Hz)",
				i, dev.Name, dev.MaxInputChannels, dev.MaxOutputChannels, dev.DefaultSampleRate)
		}
		log.Println("=====================================")
	}

	if len(devices) == 0 {
		// In embedded environments, more time may be needed for initialization
		log.Println("Warning: no audio devices found, trying default device...")
		// Try to get default input device
		defDev, defErr := portaudio.DefaultInputDevice()
		if defErr == nil && defDev != nil {
			r.targetDevice = defDev
			log.Printf("Using default input device: %s", defDev.Name)
			return nil
		}
		return fmt.Errorf("No audio devices found (including default device)")
	}

	// Priority matching logic (from highest to lowest priority)
	var candidates []*portaudio.DeviceInfo
	var priorities []int

	for _, dev := range devices {
		// Must have input channels
		if dev.MaxInputChannels == 0 {
			continue
		}

		// In embedded environments, consider even if channel count doesn't match (may support mono conversion)
		if dev.MaxInputChannels < r.config.Channels && r.config.Channels > 1 {
			if r.enableDebug {
				log.Printf("  Device %s has insufficient channels (%d < %d), but may support conversion",
					dev.Name, dev.MaxInputChannels, r.config.Channels)
			}
		}

		devNameLower := strings.ToLower(dev.Name)
		priority := 0

		// Priority 1: PulseAudio/PipeWire (best choice for desktop environments)
		if strings.Contains(devNameLower, "pulse") {
			priority = 200
		} else if strings.Contains(devNameLower, "pipewire") {
			priority = 190
		}

		// Priority 2: Explicit microphone devices
		if strings.Contains(devNameLower, "microphone") ||
			strings.Contains(devNameLower, "mic") {
			priority += 100
		}

		// Priority 3: Digital microphone (usually better quality)
		if strings.Contains(devNameLower, "digital") {
			priority += 50
		}

		// Priority 4: sof-hda-dsp devices
		if strings.Contains(devNameLower, "sof-hda-dsp") {
			priority += 40
		}

		// Priority 5: Embedded hardware devices (preferred to avoid latency)
		if strings.Contains(devNameLower, "audiocodec") ||
			strings.Contains(devNameLower, "sunxi-codec") ||
			strings.Contains(devNameLower, "allwinner") ||
			strings.Contains(dev.Name, "hw:0,0") ||
			strings.Contains(dev.Name, "plughw:0,0") {
			priority += 180
		}

		// Priority 6: Capture devices (direct hardware recording, avoids dsnoop latency)
		if strings.HasPrefix(devNameLower, "capture") && !strings.Contains(devNameLower, "dsnoop") {
			priority += 170
		}

		// Priority 7: default device (generic but may have extra latency)
		if devNameLower == "default" {
			priority = 150
		}

		// Priority 8: plughw devices (plugin support, more flexible)
		if strings.Contains(devNameLower, "plughw") {
			priority += 25
		}

		// Exclude unwanted devices
		if strings.Contains(devNameLower, "monitor") ||
			strings.Contains(devNameLower, "loopback") ||
			strings.Contains(devNameLower, "sysdefault") ||
			strings.Contains(devNameLower, "lavrate") ||
			strings.Contains(devNameLower, "samplerate") ||
			strings.Contains(devNameLower, "speexrate") ||
			strings.Contains(devNameLower, "upmix") ||
			strings.Contains(devNameLower, "vdownmix") {
			continue
		}

		// Even if priority is 0, add to candidates as long as there are input channels (embedded environments may have non-standard device names)
		if priority == 0 && dev.MaxInputChannels > 0 {
			priority = 10 // Assign a base priority
			if r.enableDebug {
				log.Printf("  Unrecognized input device: %s (assigned base priority)", dev.Name)
			}
		}

		if priority > 0 {
			candidates = append(candidates, dev)
			priorities = append(priorities, priority)
			if r.enableDebug {
				log.Printf("  Candidate device: %s (priority: %d)", dev.Name, priority)
			}
		}
	}

	// Select the highest priority device
	maxPriority := -1
	for i, p := range priorities {
		if p > maxPriority {
			maxPriority = p
			r.targetDevice = candidates[i]
		}
	}

	// Fall back to default device
	if r.targetDevice == nil {
		if defDev, err := portaudio.DefaultInputDevice(); err == nil {
			r.targetDevice = defDev
			log.Println("Warning: no matching recording device found, using default input device")
		} else {
			return fmt.Errorf("No available recording device found")
		}
	}

	log.Printf("Selected recording device: %s (Input channels: %d, Default sample rate: %.0f Hz)",
		r.targetDevice.Name, r.targetDevice.MaxInputChannels, r.targetDevice.DefaultSampleRate)

	if r.enableDebug {
		log.Printf("Capture sample rate: %d Hz, Output sample rate: %d Hz",
			r.config.CaptureSampleRate, r.config.SampleRate)
	}

	return nil
}

// Terminate terminates the audio system
func (r *Recorder) Terminate() error {
	r.cancel()

	r.mutex.Lock()
	if r.stream != nil {
		stopErr := r.stream.Stop()
		if stopErr != nil {
			return stopErr
		}
		closeErr := r.stream.Close()
		if closeErr != nil {
			return closeErr
		}
		r.stream = nil
	}
	r.mutex.Unlock()

	if r.isPortAudioInit {
		err := portaudio.Terminate()
		r.isPortAudioInit = false
		r.deviceInitialized = false
		return err
	}
	return nil
}

// StartRecording starts recording
func (r *Recorder) StartRecording(requestID string) error {
	if !r.deviceInitialized {
		return fmt.Errorf("Audio device not initialized")
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.isRecording {
		return nil // Already recording
	}

	// Initialize streaming buffer
	r.streamingMutex.Lock()
	r.streamingRequestID = requestID
	// Capture buffer needs to be larger (based on capture sample rate)
	captureChunkSize := int(float64(r.config.CaptureSampleRate) * r.config.ChunkDuration.Seconds())
	r.streamingBuffer = make([]int16, 0, captureChunkSize*2)
	r.resampleBuffer = make([]int16, 0, r.config.ChunkSampleCount*2)
	r.streamingMutex.Unlock()

	// Determine the actual sample rate to use
	// Priority:
	// 1. If device default sample rate = target sample rate, use directly (no resampling needed)
	// 2. Otherwise use configured capture sample rate, fall back to device default on failure
	actualSampleRate := r.config.CaptureSampleRate
	if r.targetDevice.DefaultSampleRate > 0 {
		deviceDefaultRate := int(r.targetDevice.DefaultSampleRate)
		if deviceDefaultRate == r.config.SampleRate {
			// Device default sample rate matches target, use directly to avoid unnecessary resampling
			actualSampleRate = deviceDefaultRate
			if r.enableDebug {
				log.Printf("Device default sample rate %d Hz matches target, using directly (no resampling needed)", actualSampleRate)
			}
		} else if r.enableDebug {
			log.Printf("Device default sample rate %d Hz differs from configured %d Hz, will try configured rate",
				deviceDefaultRate, r.config.CaptureSampleRate)
		}
	}

	// Configure audio stream parameters
	params := portaudio.StreamParameters{
		Input: portaudio.StreamDeviceParameters{
			Device:   r.targetDevice,
			Channels: r.config.Channels,
			Latency:  r.targetDevice.DefaultLowInputLatency,
		},
		SampleRate:      float64(actualSampleRate),
		FramesPerBuffer: 1024,
	}

	var err error
	r.stream, err = portaudio.OpenStream(params, r.audioCallback)
	if err != nil {
		// If opening fails, try using the device's default sample rate
		if actualSampleRate != int(r.targetDevice.DefaultSampleRate) && r.targetDevice.DefaultSampleRate > 0 {
			log.Printf("Failed with sample rate %d Hz: %v, trying device default sample rate %d Hz",
				actualSampleRate, err, int(r.targetDevice.DefaultSampleRate))
			actualSampleRate = int(r.targetDevice.DefaultSampleRate)
			params.SampleRate = float64(actualSampleRate)
			r.stream, err = portaudio.OpenStream(params, r.audioCallback)
		}
		if err != nil {
			return fmt.Errorf("Failed to open audio stream: %v", err)
		}
	}

	// Record the actual sample rate in use
	r.actualCaptureSampleRate = actualSampleRate

	if err := r.stream.Start(); err != nil {
		err := r.stream.Close()
		if err != nil {
			return err
		}
		r.stream = nil
		return fmt.Errorf("Failed to start recording: %v", err)
	}

	r.isRecording = true
	if r.enableDebug {
		log.Printf("Recording started (Device: %s, Actual capture rate: %dHz, Output rate: %dHz)",
			r.targetDevice.Name, r.actualCaptureSampleRate, r.config.SampleRate)
	}
	return nil
}

// StopRecording stops recording
func (r *Recorder) StopRecording() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if !r.isRecording {
		return nil // Not recording
	}

	r.isRecording = false

	if r.stream != nil {
		stopErr := r.stream.Stop()
		if stopErr != nil {
			return stopErr
		}
		closeErr := r.stream.Close()
		if closeErr != nil {
			return closeErr
		}
		r.stream = nil
	}

	// Send remaining audio data
	r.streamingMutex.Lock()
	remainingBuffer := make([]int16, len(r.streamingBuffer))
	copy(remainingBuffer, r.streamingBuffer)
	resampleBuffer := make([]int16, len(r.resampleBuffer))
	copy(resampleBuffer, r.resampleBuffer)
	requestID := r.streamingRequestID
	r.streamingBuffer = nil
	r.resampleBuffer = nil
	r.streamingMutex.Unlock()

	// Resample remaining captured data
	if len(remainingBuffer) > 0 {
		resampled := utils.ResampleAudio(remainingBuffer, r.config.CaptureSampleRate, r.config.SampleRate)
		resampleBuffer = append(resampleBuffer, resampled...)
	}

	if len(resampleBuffer) > 0 {
		// Diagnostics for the last audio chunk (debug mode only)
		if r.enableDebug {
			rms := utils.CalculateRMS(resampleBuffer)
			stats := utils.CalculateAudioStats(resampleBuffer, 100)
			isSilent := utils.IsSilent(resampleBuffer, 200.0, 0.95)

			log.Printf("Last audio chunk diagnostics - RMS: %.2f, Peak: %d, Silence ratio: %.2f%%, Is silent: %v, Samples: %d",
				rms, stats.Peak, stats.SilenceRatio*100, isSilent, len(resampleBuffer))
			log.Printf("Sending last audio data: %d samples", len(resampleBuffer))
		}
		r.handler.OnAudioChunk(requestID, resampleBuffer, true)
	} else {
		// No remaining data, notify recording complete
		log.Println("Warning: recording ended but no audio data was captured")
		r.handler.OnRecordingComplete(requestID, nil)
	}

	if r.enableDebug {
		log.Println("Recording stopped")
	}
	return nil
}

// IsRecording checks if currently recording
func (r *Recorder) IsRecording() bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.isRecording
}

// audioCallback is the audio callback function
func (r *Recorder) audioCallback(in []int16) {
	if !r.isRecording {
		return
	}

	// Streaming processing - accumulate captured data
	r.streamingMutex.Lock()
	r.streamingBuffer = append(r.streamingBuffer, in...)

	// Calculate how many capture samples are needed to generate one output chunk (using actual sample rate)
	captureChunkSize := int(float64(r.actualCaptureSampleRate) * r.config.ChunkDuration.Seconds())

	// When capture buffer has enough data, perform resampling
	for len(r.streamingBuffer) >= captureChunkSize {
		// Extract one capture chunk
		captureChunk := r.streamingBuffer[:captureChunkSize]
		r.streamingBuffer = r.streamingBuffer[captureChunkSize:]

		// Resample to target sample rate (using actual sample rate)
		resampled := utils.ResampleAudio(captureChunk, r.actualCaptureSampleRate, r.config.SampleRate)
		r.resampleBuffer = append(r.resampleBuffer, resampled...)

		// Send when resample buffer reaches output chunk size
		for len(r.resampleBuffer) >= r.config.ChunkSampleCount {
			chunk := make([]int16, r.config.ChunkSampleCount)
			copy(chunk, r.resampleBuffer[:r.config.ChunkSampleCount])
			r.resampleBuffer = r.resampleBuffer[r.config.ChunkSampleCount:]

			// Audio diagnostics (debug mode only)
			if r.enableDebug {
				rms := utils.CalculateRMS(chunk)
				// Use adaptive threshold: 50% of RMS as sample silence threshold
				sampleThreshold := int16(rms * 0.5)
				if sampleThreshold < 100 {
					sampleThreshold = 100
				}
				stats := utils.CalculateAudioStats(chunk, sampleThreshold)
				isSilent := utils.IsSilent(chunk, 200.0, 0.95)

				log.Printf("Audio diagnostics - RMS: %.2f, Peak: %d, Silence ratio: %.2f%%, Threshold: %d, Is silent: %v",
					rms, stats.Peak, stats.SilenceRatio*100, sampleThreshold, isSilent)
			}

			requestID := r.streamingRequestID
			r.streamingMutex.Unlock()

			// Send asynchronously to avoid blocking recording
			go r.handler.OnAudioChunk(requestID, chunk, false)

			r.streamingMutex.Lock()
		}
	}
	r.streamingMutex.Unlock()
}

// ConvertToWAV converts sample data to WAV format
func (r *Recorder) ConvertToWAV(samples []int16) []byte {
	return utils.ConvertSamplesToWAV(
		samples,
		r.config.SampleRate,
		r.config.Channels,
		r.config.BitDepth,
	)
}

// TestRecording tests recording functionality, records for a specified duration and saves to file
func (r *Recorder) TestRecording(duration int, filename string) error {
	// Check if already recording
	if r.IsRecording() {
		return fmt.Errorf("Currently recording, cannot start test recording")
	}

	log.Printf("Starting test recording, duration: %d seconds, saving to: %s", duration, filename)

	// Initialize audio device
	if err := r.Initialize(); err != nil {
		return fmt.Errorf("Failed to initialize audio device: %v", err)
	}

	// Determine actual sample rate (prefer previously recorded actual sample rate)
	actualSampleRate := r.config.CaptureSampleRate
	if r.actualCaptureSampleRate > 0 {
		actualSampleRate = r.actualCaptureSampleRate
	} else if r.targetDevice.DefaultSampleRate > 0 {
		deviceDefaultRate := int(r.targetDevice.DefaultSampleRate)
		if deviceDefaultRate == r.config.SampleRate {
			// Device default sample rate matches target, use directly
			actualSampleRate = deviceDefaultRate
		}
	}

	// Create audio stream parameters (using blocking read instead of callback)
	streamParams := portaudio.StreamParameters{
		Input: portaudio.StreamDeviceParameters{
			Device:   r.targetDevice,
			Channels: r.config.Channels,
			Latency:  r.targetDevice.DefaultLowInputLatency,
		},
		SampleRate:      float64(actualSampleRate),
		FramesPerBuffer: 1024,
	}

	// Prepare buffers
	bufferSize := actualSampleRate * duration * r.config.Channels
	testBuffer := make([]int16, 0, bufferSize)
	readBuffer := make([]int16, 1024*r.config.Channels) // Buffer for Read()

	// Create audio stream (using blocking read)
	stream, err := portaudio.OpenStream(streamParams, &readBuffer)
	if err != nil {
		// Try using device default sample rate
		if r.targetDevice.DefaultSampleRate > 0 && actualSampleRate != int(r.targetDevice.DefaultSampleRate) {
			log.Printf("Failed with sample rate %d Hz: %v", actualSampleRate, err)
			actualSampleRate = int(r.targetDevice.DefaultSampleRate)
			log.Printf("Trying device default sample rate: %d Hz", actualSampleRate)
			streamParams.SampleRate = float64(actualSampleRate)
			bufferSize = actualSampleRate * duration * r.config.Channels
			testBuffer = make([]int16, 0, bufferSize)
			stream, err = portaudio.OpenStream(streamParams, &readBuffer)
		}
		if err != nil {
			return fmt.Errorf("Failed to open audio stream: %v", err)
		}
	}
	defer stream.Close()

	log.Printf("Test recording params: Sample rate=%d Hz, Channels=%d, Duration=%d seconds",
		actualSampleRate, r.config.Channels, duration)

	// Start recording
	if err := stream.Start(); err != nil {
		return fmt.Errorf("Failed to start audio stream: %v", err)
	}

	log.Printf("Recording...")
	startTime := time.Now()

	// Calculate the number of reads needed
	totalSamplesNeeded := actualSampleRate * duration
	readsNeeded := (totalSamplesNeeded + len(readBuffer) - 1) / len(readBuffer) // Round up
	totalRead := 0
	readCount := 0

	log.Printf("Planning %d reads, %d samples each, %d total samples needed",
		readsNeeded, len(readBuffer), totalSamplesNeeded)

	// Read exact number of times
	successReads := 0
	for readCount < readsNeeded {
		readStartTime := time.Now()
		err := stream.Read()
		readDuration := time.Since(readStartTime)

		if err != nil {
			log.Printf("Audio data read error (read %d/%d, took %.3f seconds): %v",
				readCount+1, readsNeeded, readDuration.Seconds(), err)

			// If too many consecutive failures, exit
			if readCount-successReads > 10 {
				log.Printf("Too many consecutive failures, stopping reads")
				break
			}
			readCount++
			continue
		}

		// Successful read
		successReads++

		// Copy data to buffer
		chunk := make([]int16, len(readBuffer))
		copy(chunk, readBuffer)
		testBuffer = append(testBuffer, chunk...)
		totalRead += len(readBuffer)
		readCount++

		// Debug logging and performance check
		if r.enableDebug {
			if readDuration > 150*time.Millisecond {
				log.Printf("Warning: read %d took %.3f seconds (possible device latency)",
					readCount, readDuration.Seconds())
			}
			if readCount%50 == 0 {
				log.Printf("Read %d/%d times, %d total samples (%.2f seconds)",
					readCount, readsNeeded, totalRead, float64(totalRead)/float64(actualSampleRate))
			}
		}
	}

	actualDuration := time.Since(startTime)
	log.Printf("Actual recording duration: %.2f seconds, Reads: %d/%d, Samples: %d/%d",
		actualDuration.Seconds(), readCount, readsNeeded, totalRead, totalSamplesNeeded)

	// Stop recording
	if err := stream.Stop(); err != nil {
		log.Printf("Stop audio stream warning: %v", err)
	}

	recordedSamples := testBuffer

	// Resample (if needed)
	if actualSampleRate != r.config.SampleRate {
		log.Printf("Resampling: %d Hz -> %d Hz", actualSampleRate, r.config.SampleRate)
		recordedSamples = utils.ResampleAudio(recordedSamples, actualSampleRate, r.config.SampleRate)
		log.Printf("Sample count after resampling: %d", len(recordedSamples))
	} else {
		log.Printf("Sample rate matches, no resampling needed (%d Hz)", actualSampleRate)
	}

	// Audio statistics
	if r.enableDebug {
		rms := utils.CalculateRMS(recordedSamples)
		stats := utils.CalculateAudioStats(recordedSamples, 100)
		log.Printf("Audio statistics - RMS: %.2f, Peak: %d, Silence ratio: %.2f%%",
			rms, stats.Peak, stats.SilenceRatio*100)
	}

	// Convert to WAV format
	wavData := r.ConvertToWAV(recordedSamples)

	// Save to file
	if err := ioutil.WriteFile(filename, wavData, 0644); err != nil {
		return fmt.Errorf("Failed to save file: %v", err)
	}

	log.Printf("Test recording complete, file saved: %s (%.2f KB)", filename, float64(len(wavData))/1024)

	return nil
}
