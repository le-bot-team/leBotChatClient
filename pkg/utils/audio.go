// Package utils provides utility functions
package utils

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math"
	"time"
)

// GenerateRequestID generates a request ID
func GenerateRequestID(deviceSN string) string {
	return fmt.Sprintf("%s-%d", deviceSN, time.Now().UnixNano())
}

// GenerateUUID generates a UUID v4
func GenerateUUID() string {
	uuid := make([]byte, 16)
	rand.Read(uuid)

	// Set version (4) and variant bits
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // Version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // Variant 10

	return fmt.Sprintf("%x-%x-%x-%x-%x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}

// GenerateWAVHeader generates a WAV file header
func GenerateWAVHeader(dataSize int, sampleRate int, channels int, bitDepth int) []byte {
	header := make([]byte, 44)

	// RIFF chunk
	copy(header[0:4], "RIFF")
	fileSize := dataSize + 36
	binary.LittleEndian.PutUint32(header[4:8], uint32(fileSize))
	copy(header[8:12], "WAVE")

	// fmt chunk
	copy(header[12:16], "fmt ")
	binary.LittleEndian.PutUint32(header[16:20], 16)                 // fmt chunk size
	binary.LittleEndian.PutUint16(header[20:22], 1)                  // audio format (PCM)
	binary.LittleEndian.PutUint16(header[22:24], uint16(channels))   // channels
	binary.LittleEndian.PutUint32(header[24:28], uint32(sampleRate)) // sample rate

	byteRate := sampleRate * channels * bitDepth
	binary.LittleEndian.PutUint32(header[28:32], uint32(byteRate)) // byte rate

	blockAlign := channels * bitDepth
	binary.LittleEndian.PutUint16(header[32:34], uint16(blockAlign)) // block align
	binary.LittleEndian.PutUint16(header[34:36], uint16(bitDepth*8)) // bits per sample

	// data chunk
	copy(header[36:40], "data")
	binary.LittleEndian.PutUint32(header[40:44], uint32(dataSize)) // data size

	return header
}

// ConvertSamplesToWAV converts int16 sample data to WAV format byte data
func ConvertSamplesToWAV(audioSamples []int16, sampleRate int, channels int, bitDepth int) []byte {
	// PCM data
	pcmData := make([]byte, len(audioSamples)*2)
	for i, sample := range audioSamples {
		binary.LittleEndian.PutUint16(pcmData[i*2:], uint16(sample))
	}

	// Generate WAV header
	header := GenerateWAVHeader(len(pcmData), sampleRate, channels, bitDepth)

	// Merge header and data
	wavData := make([]byte, 0, len(header)+len(pcmData))
	wavData = append(wavData, header...)
	wavData = append(wavData, pcmData...)

	return wavData
}

// ResampleAudio resamples audio from one sample rate to another using linear interpolation
// This is a simple but effective resampling method suitable for speech audio
func ResampleAudio(input []int16, fromRate, toRate int) []int16 {
	if fromRate == toRate {
		return input
	}

	// Calculate output length
	ratio := float64(fromRate) / float64(toRate)
	outputLength := int(float64(len(input)) / ratio)
	output := make([]int16, outputLength)

	for i := 0; i < outputLength; i++ {
		// Calculate source position
		srcPos := float64(i) * ratio
		srcIdx := int(srcPos)

		// Boundary check
		if srcIdx >= len(input)-1 {
			output[i] = input[len(input)-1]
			continue
		}

		// Linear interpolation between two samples
		fraction := srcPos - float64(srcIdx)
		sample1 := float64(input[srcIdx])
		sample2 := float64(input[srcIdx+1])
		interpolated := sample1 + (sample2-sample1)*fraction

		output[i] = int16(interpolated)
	}

	return output
}

// AudioStats contains audio statistics
type AudioStats struct {
	RMS           float64 // Root mean square value
	Peak          int16   // Peak value
	SilentSamples int     // Number of silent samples
	TotalSamples  int     // Total number of samples
	SilenceRatio  float64 // Silence ratio
}

// CalculateRMS calculates the RMS (root mean square) value of audio
func CalculateRMS(samples []int16) float64 {
	if len(samples) == 0 {
		return 0
	}

	var sum float64
	for _, sample := range samples {
		val := float64(sample)
		sum += val * val
	}

	return math.Sqrt(sum / float64(len(samples)))
}

// CalculateAudioStats calculates audio statistics
func CalculateAudioStats(samples []int16, silenceThreshold int16) AudioStats {
	stats := AudioStats{
		TotalSamples: len(samples),
	}

	if len(samples) == 0 {
		return stats
	}

	// Calculate RMS and peak value
	var sum float64
	var peak int16
	silentCount := 0

	for _, sample := range samples {
		val := float64(sample)
		sum += val * val

		abs := sample
		if abs < 0 {
			abs = -abs
		}
		if abs > peak {
			peak = abs
		}

		if abs <= silenceThreshold {
			silentCount++
		}
	}

	stats.RMS = math.Sqrt(sum / float64(len(samples)))
	stats.Peak = peak
	stats.SilentSamples = silentCount
	stats.SilenceRatio = float64(silentCount) / float64(len(samples))

	return stats
}

// IsSilent determines if audio is silent
// rmsThreshold: RMS threshold, typically between 100-500
// silenceRatioThreshold: silence ratio threshold, between 0-1
func IsSilent(samples []int16, rmsThreshold float64, silenceRatioThreshold float64) bool {
	if len(samples) == 0 {
		return true
	}

	rms := CalculateRMS(samples)

	// If RMS is below threshold, consider it silent
	if rms < rmsThreshold {
		return true
	}

	// Check silence ratio
	silenceThreshold := int16(rmsThreshold * 0.5) // Use half of RMS threshold as sample silence threshold
	stats := CalculateAudioStats(samples, silenceThreshold)

	return stats.SilenceRatio > silenceRatioThreshold
}
