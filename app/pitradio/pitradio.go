// Package pitradio provides an interface for sending text and voice messages as a pit radio analog.
package pitradio

import "github.com/rs/zerolog"

// MessageType defines the type of message to be sent.
type MessageType int

const (
	TextMessage  MessageType = iota // Plain text message to be converted to speech
	AudioMessage                    // Pre-recorded audio message to be played as-is
)

// Message represents a text message with an associated language code.
type Message struct {
	MessageType MessageType // Type of message (text or audio)
	Text        string      // The text message to be converted to speech
	Lang        string      // Speech language code (IETF BCP-47)
	Accent      string      // Optional regional accent locale code
	Audio       []byte      // Pre-generated audio data
	NoCache     bool        // Disable caching of the result when true (typically for highly unique messages)
}

// PitRadio defines the interface for a service that can send text and voice messages.
type PitRadio interface {
	BackgroundTask()            // Management task to run in the background
	Close() error               // Disconnect from the pit radio service
	Send(message Message) error // Enqueue a message to the dispatcher
}

type LogOutput struct {
	logger zerolog.Logger
}

// NewLogOutput creates a new PitRadio implementation that outputs messages to the logging system.
func NewLogOutput(logger *zerolog.Logger) (*LogOutput, error) {
	return &LogOutput{
		logger: logger.With().Str("component", "pitradioLogOutput").Logger(),
	}, nil
}

// BackgroundTask runs any necessary background tasks for the log output.
func (p *LogOutput) BackgroundTask() {
	// No background task needed for log output
}

// Close cleans up resources used by the log output.
func (p *LogOutput) Close() error {
	// No resources to clean up for log output
	return nil
}

// Send logs the message to the logging system.
func (p *LogOutput) Send(message Message) error {
	messageType := "text"
	if message.MessageType == AudioMessage {
		messageType = "audio"
	}

	p.logger.Info().
		Str("type", messageType).
		Str("text", message.Text).
		Msg("Pit radio message sent")

	return nil
}
