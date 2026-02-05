package synthesizer

// Buffer defines the interface for audio buffers used in the synthesizer.
type Buffer interface {
	Clear()
	Length() int
	Inspect(samples int, offset int) []float64
	Read(length int) []float64
	Write(samples []float64, offset int, overwrite bool)
}
