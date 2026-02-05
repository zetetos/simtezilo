package synthesizer

import (
	"sync"
	"time"
)

// AdaptiveBuffer implements an adaptive buffer that can handle varying read/write patterns
// It combines ring buffer efficiency with overflow protection.
type AdaptiveBuffer struct {
	buffer   []float64
	mu       sync.RWMutex
	writePos int // current write position
	readPos  int // current read position
	capacity int // buffer size
	used     int // number of samples currently in buffer

	// Adaptive features
	overflows  int       // count of buffer overflows
	underruns  int       // count of buffer underruns
	lastAccess time.Time // last time buffer was accessed
	readDelay  int       // delay between buffer write and read
}

// NewAdaptiveBuffer creates a new adaptive buffer
// bufferDuration: duration of audio the buffer should hold
// sampleRateHz: sample rate in Hz to calculate buffer size in samples.
func NewAdaptiveBuffer(length time.Duration, sampleRateHz int) *AdaptiveBuffer {
	capacity := int(length.Seconds() * float64(sampleRateHz))

	// Calculate read delay as 24ms worth of samples, but cap to 25% of capacity
	// to ensure there's always room for actual content
	readDelay := (sampleRateHz / 1000) * 24

	maxReadDelay := capacity / 4
	if readDelay > maxReadDelay {
		readDelay = maxReadDelay
	}

	buffer := &AdaptiveBuffer{
		buffer:    make([]float64, capacity),
		writePos:  readDelay,
		readPos:   0,
		capacity:  capacity,
		used:      readDelay,
		readDelay: readDelay,
	}

	buffer.updateLastAccess()

	buffer.Clear()

	return buffer
}

// Clear zeros out the entire buffer and resets state.
func (b *AdaptiveBuffer) Clear() {
	b.mu.Lock()

	for i := range b.capacity {
		b.buffer[i] = 0
	}

	b.writePos = b.readDelay
	b.readPos = 0
	b.used = b.readDelay
	b.overflows = 0
	b.underruns = 0

	b.mu.Unlock()

	b.updateLastAccess()
}

// Length returns the total buffer capacity.
func (b *AdaptiveBuffer) Length() int {
	return b.capacity
}

// Used returns the number of samples currently in the buffer.
func (b *AdaptiveBuffer) Used() int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.used
}

// Available returns space available for writing.
func (b *AdaptiveBuffer) Available() int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.capacity - b.used
}

// Health returns buffer health metrics.
func (b *AdaptiveBuffer) Health() (overflows, underruns int, fillRatio float64) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	fillRatio = float64(b.used) / float64(b.capacity)

	return b.overflows, b.underruns, fillRatio
}

// Inspect returns a copy of samples from the buffer without consuming them.
// offset: position relative to write position (negative values read historical samples)
// length: number of samples to read
// If offset + length exceeds available samples, returns only available samples.
func (b *AdaptiveBuffer) Inspect(length int, offset int) []float64 {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if length <= 0 {
		return nil
	}

	// Calculate the actual read position with offset applied
	readPosition := (b.writePos + offset) % b.capacity
	if readPosition < 0 {
		readPosition += b.capacity
	}

	// Determine maximum available samples considering the offset
	var maxAvailable int
	if offset >= 0 {
		// Reading forward from write position
		maxAvailable = b.capacity - offset
		if maxAvailable > (b.capacity - b.used) {
			maxAvailable = b.capacity - b.used
		}
	} else {
		// Reading backward (historical samples)
		maxAvailable = max(b.used+offset, 0)
	}

	if length > maxAvailable {
		length = maxAvailable
	}

	if length <= 0 {
		return nil
	}

	result := make([]float64, length)

	end := readPosition + length
	if end <= b.capacity {
		copy(result, b.buffer[readPosition:end])
	} else {
		firstPart := b.capacity - readPosition
		copy(result[:firstPart], b.buffer[readPosition:])
		copy(result[firstPart:], b.buffer[:end-b.capacity])
	}

	return result
}

// Read returns samples and consumes them.
func (b *AdaptiveBuffer) Read(length int) []float64 {
	return b.readFromBuffer(length, true)
}

// Write adds samples to the buffer with overflow protection.
func (b *AdaptiveBuffer) Write(samples []float64, offset int, overwrite bool) {
	if len(samples) == 0 {
		return
	}

	b.updateLastAccess()

	b.mu.Lock()
	defer b.mu.Unlock()

	b.applyOffset(offset)

	if overwrite {
		b.writeOverwriteMode(samples)

		return
	}

	b.writeMixMode(samples)
}

// IsStarved returns true if buffer is running low.
func (b *AdaptiveBuffer) IsStarved() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.used <= b.readDelay
}

// IsOverfull returns true if buffer is too full.
func (b *AdaptiveBuffer) IsOverfull() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.used > (b.capacity * 3 / 4)
}

// GetOptimalReadSize suggests optimal read size based on current state.
func (b *AdaptiveBuffer) GetOptimalReadSize(requestedSize int) int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// If we're overfull, suggest reading more to drain buffer
	if b.used > (b.capacity * 3 / 4) {
		return min(requestedSize*2, b.used)
	}

	// If we're starved, suggest reading less to preserve buffer
	if b.used < (b.readDelay / 2) {
		return min(requestedSize/2, b.used)
	}

	// Normal case
	return min(requestedSize, b.used)
}

// applyOffset applies the write position offset if specified.
func (b *AdaptiveBuffer) applyOffset(offset int) {
	if offset != 0 {
		b.writePos = (b.writePos + offset) % b.capacity
	}
}

// writeOverwriteMode writes samples in overwrite mode, replacing existing content.
func (b *AdaptiveBuffer) writeOverwriteMode(samples []float64) {
	b.handleOverflow(samples, true)

	for _, sample := range samples {
		b.buffer[b.writePos] = sample
		b.writePos = (b.writePos + 1) % b.capacity

		if b.used < b.capacity {
			b.used++
		} else {
			b.readPos = (b.readPos + 1) % b.capacity
		}
	}
}

// writeMixMode writes samples in mix mode, combining with existing content.
func (b *AdaptiveBuffer) writeMixMode(samples []float64) {
	peak := 0.0

	for index, inputSample := range samples {
		mixedSample := b.mixSampleAtIndex(index, inputSample, &peak)
		mixPos := (b.readPos + index) % b.capacity
		b.buffer[mixPos] = mixedSample
	}

	b.syncBufferState(len(samples))
	b.applyPeakLimiting(samples, peak)
}

// mixSampleAtIndex mixes an input sample with existing content at the given index.
func (b *AdaptiveBuffer) mixSampleAtIndex(index int, inputSample float64, peak *float64) float64 {
	if index < b.used {
		mixPos := (b.readPos + index) % b.capacity
		existingSample := b.buffer[mixPos]

		return mixSampleSum(inputSample, existingSample, peak)
	}

	return inputSample
}

// syncBufferState updates used count and writePos if we wrote beyond existing content.
func (b *AdaptiveBuffer) syncBufferState(samplesWritten int) {
	if samplesWritten > b.used {
		b.used = samplesWritten
		// Update writePos to maintain consistency with used count
		b.writePos = (b.readPos + b.used) % b.capacity
	}
}

// applyPeakLimiting applies peak limiting if the peak exceeds 1.0.
func (b *AdaptiveBuffer) applyPeakLimiting(samples []float64, peak float64) {
	if peak > 1.0 {
		for i := range samples {
			pos := (b.readPos + i) % b.capacity
			b.buffer[pos] /= peak
		}
	}
}

// handleOverflow handles buffer overflow situations.
func (b *AdaptiveBuffer) handleOverflow(samples []float64, overwrite bool) {
	if len(samples) > (b.capacity-b.used) && overwrite {
		b.overflows++

		if !overwrite {
			samplesToDrop := len(samples) - (b.capacity - b.used)
			b.dropOldestSamples(samplesToDrop)
		}
	}
}

// readFromBuffer internal implementation for reading.
func (b *AdaptiveBuffer) readFromBuffer(length int, consume bool) []float64 {
	b.updateLastAccess()

	b.mu.Lock()
	defer b.mu.Unlock()

	// Check for underrun
	if length > b.used {
		if consume {
			b.underruns++
		}

		length = b.used
	}

	samples := make([]float64, length)

	for i := range length {
		samples[i] = b.buffer[b.readPos]

		if consume {
			b.buffer[b.readPos] = 0 // Clear consumed samples
			b.used--
		}

		b.readPos = (b.readPos + 1) % b.capacity
	}

	return samples
}

// dropOldestSamples removes oldest samples to prevent overflow.
func (b *AdaptiveBuffer) dropOldestSamples(count int) {
	for i := 0; i < count && b.used > 0; i++ {
		b.buffer[b.readPos] = 0
		b.readPos = (b.readPos + 1) % b.capacity
		b.used--
	}
}

// mixSamplesInternal mixes input samples with existing buffer content
// func (b *AdaptiveBuffer) mixSamplesInternal(inSamples []float64) []float64 {
// 	outSamples := make([]float64, len(inSamples))
// 	peak := 0.0

// 	for i := range inSamples {
// 		inputSample := inSamples[i]

// 		var bufferSample float64
// 		if i < b.used {
// 			bufferPos := (b.readPos + i) % b.size
// 			bufferSample = b.buffer[bufferPos]
// 		}

// 		outSamples[i] = mixSampleSum(inputSample, bufferSample, &peak)
// 	}

// 	if peak > 1.0 {
// 		scaleSamplesPeak(&outSamples, peak)
// 	}

// 	return outSamples
// }

func (b *AdaptiveBuffer) updateLastAccess() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.lastAccess = time.Now()
}
