// Package utils provides utility functions
package utils

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
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
