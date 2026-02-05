// Package discord provides a Discord bot implementation for sending text and audio messages to a Discord server.
package discord

import (
	"bytes"
	"context"
	_ "embed" // required for embedding files
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog"
	"github.com/zetetos/simtezilo/app/cache"
	"github.com/zetetos/simtezilo/app/codec"
	"github.com/zetetos/simtezilo/app/pitradio"
	"github.com/zetetos/simtezilo/app/pitradio/tts"
	"github.com/zetetos/simtezilo/app/synthesizer"
)

// Config holds the configuration for creating a new Discord bot.
type Config struct {
	Enabled        bool                           // Indicates if the bot is enabled
	Token          string                         // Discord bot token
	GuildID        string                         // Discord guild ID
	ChannelID      string                         // Optional Discord channel ID for text messages
	VoiceChannelID string                         // Optional Discord voice channel ID for audio, requires GuildID
	MessageGap     time.Duration                  // Delay between sending messages to avoid rate limiting
	Cache          *cache.Cache                   // Cache manager for storing processed TTS audio
	SampleBank     *synthesizer.EffectsSampleBank // Pre-generated audio samples
	Logger         zerolog.Logger                 // Logger instance for logging
}

// Discord represents a Discord bot instance.
type Discord struct {
	enabled        bool                           // Indicates if the bot is enabled
	channelID      string                         // Discord channel ID where messages will be sent
	voiceChannelID string                         // Discord voice channel ID for audio
	guildID        string                         // Discord guild ID for voice connections
	messageGap     time.Duration                  // Delay between sending messages to avoid rate limiting
	cache          *cache.Cache                   // Cache manager for storing processed TTS audio
	sampleBank     *synthesizer.EffectsSampleBank // Pre-generated audio samples // TODO: is there a better way to do this?
	session        *discordgo.Session             // Currently connected Discord session
	voiceConn      *discordgo.VoiceConnection     // Voice connection for audio
	voiceMu        sync.Mutex                     // Mutex to protect websocket writes to voice connection
	queue          chan pitradio.Message          // Message queue for sending messages
	log            zerolog.Logger                 // Logger instance for logging

	// Lifecycle management
	ctx    context.Context //nolint:containedctx // Context for managing lifecycle
	cancel context.CancelFunc
}

// New creates a new Discord bot instance.
func New(config Config) (*Discord, error) {
	ctx, cancel := context.WithCancel(context.Background())

	bot := Discord{
		enabled:        config.Enabled,
		channelID:      config.ChannelID,
		voiceChannelID: config.VoiceChannelID,
		guildID:        config.GuildID,
		messageGap:     config.MessageGap,
		cache:          config.Cache,
		sampleBank:     config.SampleBank,
		queue:          make(chan pitradio.Message, 100),
		log:            config.Logger.With().Str("component", "discord").Logger(),
		ctx:            ctx,
		cancel:         cancel,
	}

	textEnabled := true
	voiceEnabled := true

	if config.Token == "" {
		bot.enabled = false

		bot.log.Info().
			Str("reason", "api token not provided").
			Msg("Discord bot disabled")
	}

	if config.ChannelID == "" {
		textEnabled = false

		bot.log.Info().
			Str("reason", "channel ID not provided").
			Msg("Discord text messaging disabled")
	}

	if config.GuildID == "" {
		voiceEnabled = false

		bot.log.Info().
			Str("reason", "voice guild ID not provided").
			Msg("Discord voice messaging disabled")
	}

	if config.VoiceChannelID == "" {
		voiceEnabled = false

		bot.log.Info().
			Str("reason", "voice channel ID not provided").
			Msg("Discord voice messaging disabled")
	}

	if !textEnabled && !voiceEnabled {
		bot.enabled = false
	}

	if !bot.enabled {
		return &bot, nil
	}

	var err error

	bot.session, err = discordgo.New("Bot " + config.Token)
	if err != nil {
		return &Discord{}, err
	}

	return &bot, nil
}

func (d *Discord) BackgroundTask() {
	ticker := time.NewTicker(d.messageGap)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			if !d.enabled {
				return
			}

			if !d.isConnected() || d.isDead() {
				d.handleReconnection()

				continue
			}

			d.dispatchMessages()
		}
	}
}

// Close terminates the connection to the Discord WebSocket.
// TODO: ensure speaking is stopped before closing session.
func (d *Discord) Close() error {
	if d.session == nil {
		return errors.New("discord session not initialized")
	}

	d.enabled = false

	// Cancel context to stop background task
	if d.cancel != nil {
		d.cancel()
	}

	// Leave voice channel if connected
	if d.voiceConn != nil {
		_ = d.leaveVoiceChannel()
	}

	err := d.session.Close()
	if err != nil {
		return fmt.Errorf("close Discord session: %w", err)
	}

	return nil
}

// Send sends a message to the Discord channel.
func (d *Discord) Send(message pitradio.Message) error {
	if d.session == nil {
		return errors.New("discord session not initialized")
	}

	d.queue <- message

	return nil
}

// connect establishes a connection to the Discord WebSocket and joins the voice channel.
func (d *Discord) connect() error {
	if !d.enabled {
		d.log.Info().
			Str("reason", "bot not enabled").
			Msg("Discord connection skipped")

		return nil
	}

	if d.session == nil {
		return errors.New("discord session not initialized")
	}

	if !d.sessionIsConnected() {
		err := d.session.Open()
		if err != nil {
			return fmt.Errorf("open Discord session: %w", err)
		}

		d.session.AddHandler(ready)
	}

	err := d.joinVoiceChannel()
	if err != nil {
		return fmt.Errorf("join voice channel: %w", err)
	}

	return nil
}

// handleReconnection handles the reconnection logic.
func (d *Discord) handleReconnection() {
	err := d.connect()
	if err != nil {
		d.log.Error().
			Err(err).
			Str("result", "failure").
			Msg("voice channel connect")

		// Use select to allow context cancellation during sleep
		select {
		case <-d.ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}

		return
	}

	d.sendTalkPermitTone()

	d.log.Info().
		Str("result", "success").
		Msg("voice channel connect")

	// connect anti-spam just in case
	if d.messageGap < 1*time.Second {
		select {
		case <-d.ctx.Done():
			return
		case <-time.After(1 * time.Second):
		}
	}
}

// sendTalkPermitTone sends a talk permit tone after successful connection.
func (d *Discord) sendTalkPermitTone() {
	effectSample := d.sampleBank.GetSample("talkPermitTone", codec.OpusSampleRate/2)

	dcaData, err := effectSample.ToDCA()
	if err != nil {
		d.log.Error().
			Err(err).
			Str("result", "failure").
			Msg("generate talk permit tone")

		return
	}

	err = d.Send(pitradio.Message{
		MessageType: pitradio.AudioMessage,
		Text:        "talk permit tone",
		Audio:       dcaData,
		NoCache:     false,
	})
	if err != nil {
		d.log.Error().
			Err(err).
			Str("result", "failure").
			Msg("send message")
	}
}

// MessageDispatcher processes the message queue and sends voice messages to Discord.
func (d *Discord) dispatchMessages() {
	select {
	case message := <-d.queue:
		if d.channelID != "" && message.MessageType == pitradio.TextMessage {
			_, err := d.session.ChannelMessageSend(d.channelID, message.Text)
			if err != nil {
				d.log.Error().
					Err(err).
					Str("package", "discord").
					Msg("send message to Discord channel")
			}

			d.log.Debug().
				Str("text", message.Text).
				Str("type", "text").
				Str("channel_id", d.channelID).
				Msg("Discord send message")
		}

		if !d.isReady() {
			return
		}

		var dcaData []byte

		if message.MessageType == pitradio.TextMessage {
			var err error

			dcaData, err = d.messageToDCA(message)
			if err != nil {
				d.log.Error().
					Err(err).
					Str("package", "discord").
					Msg("generate TTS audio from message")

				return
			}
		} else {
			dcaData = message.Audio
		}

		err := d.voiceMessageSend(dcaData)
		if err != nil {
			d.log.Error().
				Err(err).
				Msg("send DCA audio message to Discord channel")

			return
		}

		d.log.Debug().
			Str("text", message.Text).
			Str("type", "audio").
			Str("channel_id", d.voiceChannelID).
			Msg("Discord send message")
	default:
		return
	}
}

// isConnected returns true if the bot is connected to Discord.
func (d *Discord) isConnected() bool {
	return d.session != nil && d.session.DataReady
}

// isReady returns true if the voice connection is ready to send audio.
func (d *Discord) isReady() bool {
	return d.voiceConn != nil &&
		d.voiceConn.Status == discordgo.VoiceConnectionStatusReady &&
		d.voiceConn.OpusSend != nil
}

// isDead returns true if the voice connection is in a dead state.
func (d *Discord) isDead() bool {
	if d.voiceConn == nil {
		return true
	}

	if d.voiceConn.Err != nil {
		d.log.Error().
			Err(d.voiceConn.Err).
			Int("status", int(d.voiceConn.Status)).
			Str("error_type", fmt.Sprintf("%T", d.voiceConn.Err)).
			Msg("voice connection error detected")

		return true
	}

	// Check if status indicates a problem (not ready)
	if d.voiceConn.Status != discordgo.VoiceConnectionStatusReady {
		d.log.Warn().
			Int("status", int(d.voiceConn.Status)).
			Bool("has_opus_send", d.voiceConn.OpusSend != nil).
			Msg("voice connection not in ready state")

		return true
	}

	return false
}

// joinVoiceChannel joins the configured voice channel for audio streaming.
func (d *Discord) joinVoiceChannel() error {
	if d.session == nil {
		return errors.New("discord session not initialized")
	}

	if d.voiceChannelIsConnected() {
		return nil
	}

	// Use a derived context that respects both the 10s timeout and the main context
	ctx, cancel := context.WithTimeout(d.ctx, 10*time.Second)
	defer cancel()

	vc, err := d.session.ChannelVoiceJoin(ctx, d.guildID, d.voiceChannelID, false, false)
	if err != nil {
		return fmt.Errorf("join voice channel: %w", err)
	}

	d.voiceConn = vc

	// Wait for voice connection to be ready
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for voice connection to be ready: %w", ctx.Err())
		case <-ticker.C:
			if d.voiceConn.Status == discordgo.VoiceConnectionStatusReady && d.voiceConn.OpusSend != nil {
				d.log.Debug().
					Str("guild_id", d.guildID).
					Str("channel_id", d.voiceChannelID).
					Msg("voice connection ready")

				return nil
			}
		}
	}
}

// leaveVoiceChannel leaves the current voice channel.
func (d *Discord) leaveVoiceChannel() error {
	if d.voiceConn == nil {
		return nil
	}

	d.log.Debug().
		Int("status", int(d.voiceConn.Status)).
		Bool("has_error", d.voiceConn.Err != nil).
		Msg("leaving voice channel")

	err := d.voiceConn.Disconnect(context.Background())
	if err != nil {
		return fmt.Errorf("leave voice channel: %w", err)
	}

	d.voiceConn = nil

	return nil
}

// sessionIsConnected returns true if the Discord session is connected.
func (d *Discord) sessionIsConnected() bool {
	return d.session != nil && d.session.DataReady
}

// voiceChannelIsConnected returns true if the bot is connected to a voice channel.
func (d *Discord) voiceChannelIsConnected() bool {
	return d.voiceConn != nil &&
		d.voiceConn.Status == discordgo.VoiceConnectionStatusReady &&
		d.voiceConn.OpusSend != nil
}

// voiceMessageSend sends the given DCA audio data to the voice channel.
func (d *Discord) voiceMessageSend(dcaData []byte) error {
	if d.voiceConn == nil {
		return errors.New("not connected to voice channel")
	}

	if d.voiceConn.Status != discordgo.VoiceConnectionStatusReady {
		return errors.New("voice connection not ready")
	}

	if d.voiceConn.OpusSend == nil {
		return errors.New("voice connection opus channel not available")
	}

	return d.sendVoiceAudio(dcaData)
}

// sendVoiceAudio plays a DCA audio buffer to the voice channel.
func (d *Discord) sendVoiceAudio(dca []byte) error {
	// Lock to prevent concurrent websocket writes
	d.voiceMu.Lock()
	defer d.voiceMu.Unlock()

	if d.voiceConn == nil {
		return errors.New("not connected to voice channel")
	}

	if d.voiceConn.Status != discordgo.VoiceConnectionStatusReady {
		return errors.New("voice connection not ready")
	}

	if d.voiceConn.OpusSend == nil {
		return errors.New("voice connection opus channel not available")
	}

	if len(dca) == 0 {
		return errors.New("no audio data to send")
	}

	// Send speaking packet
	err := d.voiceConn.Speaking(true)
	if err != nil {
		d.log.Error().
			Err(err).
			Int("status", int(d.voiceConn.Status)).
			Bool("has_opus_send", d.voiceConn.OpusSend != nil).
			Bool("has_error", d.voiceConn.Err != nil).
			Msg("failed to start speaking - connection state details")

		return fmt.Errorf("start speaking: %w", err)
	}

	// Small delay to ensure Discord processes the speaking state
	time.Sleep(50 * time.Millisecond)

	defer func() {
		// Always stop speaking when done
		_ = d.voiceConn.Speaking(false)
	}()

	var opusLen int16

	reader := bytes.NewReader(dca)

	for {
		// Read opus frame length from dca file.
		err = binary.Read(reader, binary.LittleEndian, &opusLen)

		// If this is the end of the file, just return.
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil
		}

		if err != nil {
			d.log.Error().
				Err(err).
				Msg("read Opus frame length from DCA file")

			return err
		}

		// Read encoded pcm from dca file.
		buf := make([]byte, opusLen)
		err = binary.Read(reader, binary.LittleEndian, &buf)
		// Should not be any end of file errors
		if err != nil {
			d.log.Error().
				Err(err).
				Msg("read DCA file")

			return err
		}

		d.voiceConn.OpusSend <- buf
	}
}

// messageToDCA generates DCA audio data for a message.
// If the audio data is cached, it retrieves it from the cache instead of generating it again.
func (d *Discord) messageToDCA(message pitradio.Message) (dcaData []byte, err error) {
	itemID := fmt.Sprintf("%s_%s_%s", message.Lang, message.Accent, message.Text)

	// Try to read from cache first
	if !message.NoCache {
		dcaData, err = d.cache.Read(itemID)
		if err == nil && len(dcaData) > 0 {
			return dcaData, nil
		}
	}

	// Cache miss - generate TTS audio data using TextToSpeech
	if message.MessageType == pitradio.TextMessage {
		mp3Data, err := tts.TextToSpeech(message)
		if err != nil {
			return []byte{}, fmt.Errorf("generate TTS audio: %w", err)
		}

		// dcaData, err = TranscodeMP3toDCA(mpegData)
		dcaData, err = mp3Data.ToDCA()
		if err != nil {
			return []byte{}, fmt.Errorf("transcode MP3 to DCA: %w", err)
		}
	} else {
		dcaData = message.Audio
	}

	// Cache the DCA data for future use
	if !message.NoCache {
		err = d.cache.Write(itemID, dcaData)
		if err != nil {
			d.log.Error().
				Err(err).
				Msgf("cache DCA data")
		}
	}

	return dcaData, nil
}

// ready updates the watch status of the bot user.
func ready(s *discordgo.Session, _ *discordgo.Event) {
	_ = s.UpdateWatchStatus(0, "Gran Turismo 7")
}
