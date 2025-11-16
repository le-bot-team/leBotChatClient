// Package utils provides utility functions
package utils

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math"
	"time"
)

// GenerateRequestID 生成请求ID
func GenerateRequestID(deviceSN string) string {
	return fmt.Sprintf("%s-%d", deviceSN, time.Now().UnixNano())
}

// GenerateUUID 生成UUID v4
func GenerateUUID() string {
	uuid := make([]byte, 16)
	rand.Read(uuid)

	// 设置版本 (4) 和变体位
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // Version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // Variant 10

	return fmt.Sprintf("%x-%x-%x-%x-%x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}

// GenerateWAVHeader 生成WAV文件头部
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

// ConvertSamplesToWAV 将int16采样数据转换为WAV格式的字节数据
func ConvertSamplesToWAV(audioSamples []int16, sampleRate int, channels int, bitDepth int) []byte {
	// PCM数据
	pcmData := make([]byte, len(audioSamples)*2)
	for i, sample := range audioSamples {
		binary.LittleEndian.PutUint16(pcmData[i*2:], uint16(sample))
	}

	// 生成WAV头部
	header := GenerateWAVHeader(len(pcmData), sampleRate, channels, bitDepth)

	// 合并头部和数据
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

// AudioStats 音频统计信息
type AudioStats struct {
	RMS           float64 // 均方根值
	Peak          int16   // 峰值
	SilentSamples int     // 静音采样点数
	TotalSamples  int     // 总采样点数
	SilenceRatio  float64 // 静音比例
}

// CalculateRMS 计算音频的RMS（均方根）值
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

// CalculateAudioStats 计算音频统计信息
func CalculateAudioStats(samples []int16, silenceThreshold int16) AudioStats {
	stats := AudioStats{
		TotalSamples: len(samples),
	}

	if len(samples) == 0 {
		return stats
	}

	// 计算RMS和峰值
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

// IsSilent 判断音频是否为静音
// rmsThreshold: RMS阈值，通常100-500之间
// silenceRatioThreshold: 静音比例阈值，0-1之间
func IsSilent(samples []int16, rmsThreshold float64, silenceRatioThreshold float64) bool {
	if len(samples) == 0 {
		return true
	}

	rms := CalculateRMS(samples)

	// 如果RMS低于阈值，认为是静音
	if rms < rmsThreshold {
		return true
	}

	// 检查静音比例
	silenceThreshold := int16(rmsThreshold * 0.5) // 使用RMS阈值的一半作为采样点静音阈值
	stats := CalculateAudioStats(samples, silenceThreshold)

	return stats.SilenceRatio > silenceRatioThreshold
}
