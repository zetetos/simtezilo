// Package webui implements a simple web server to serve a web-based user interface
package webui

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/zetetos/simtezilo/app/calibrator"
	appconfig "github.com/zetetos/simtezilo/app/config"
	"github.com/zetetos/simtezilo/app/exitcode"
	"github.com/zetetos/simtezilo/app/haptics"
	"github.com/zetetos/simtezilo/app/hardware"
	"github.com/zetetos/simtezilo/app/logstore"
	"github.com/zetetos/simtezilo/app/platform"
	"github.com/zetetos/simtezilo/app/setupmode"
	"github.com/zetetos/simtezilo/app/ui/webui/webcommon"
	"github.com/zetetos/simtezilo/app/updater"
)

// WSMessage represents a typed message envelope for the unified WebSocket.
type WSMessage struct {
	Type      string `json:"type"`      // Message type
	Timestamp int64  `json:"timestamp"` // Unix timestamp in milliseconds
	Data      any    `json:"data"`      // Message data
}

// wsMessage is an internal message sent to a client's write goroutine.
type wsMessage struct {
	msgType     string // Message type for subscription filtering (e.g., "telemetry", "vehicle")
	data        []byte // Pre-encoded message data
	isPing      bool   // True if this is a ping control message
	isInitState bool   // True if this is initial state (bypasses subscription check)
}

// wsClient represents a connected websocket client with its own write channel.
// Each client has a dedicated goroutine that handles all writes, eliminating
// the need for mutexes and following Go's "share memory by communicating" idiom.
type wsClient struct {
	conn          *websocket.Conn
	send          chan wsMessage  // Channel for outgoing messages
	subscriptions map[string]bool // What data types this client wants
	subMu         sync.RWMutex    // Protects subscriptions map
	done          chan struct{}   // Signals client shutdown
	closeOnce     sync.Once       // Ensures cleanup happens once
	log           zerolog.Logger  // Logger for this client
}

// newWSClient creates a new websocket client with default subscriptions.
func newWSClient(conn *websocket.Conn, log zerolog.Logger) *wsClient {
	return &wsClient{
		conn: conn,
		send: make(chan wsMessage, 64), // Buffered to handle bursts
		subscriptions: map[string]bool{
			"vehicle":     true,
			"gameState":   true,
			"circuit":     true,
			"race":        true,
			"logStats":    true,
			"calibration": true,
			"telemetry":   false, // Telemetry off by default
		},
		done: make(chan struct{}),
		log:  log,
	}
}

// Send queues a message to be sent to the client.
// Returns false if the client is closed or the send buffer is full.
func (c *wsClient) Send(msgType string, data []byte, isInitState bool) bool {
	select {
	case <-c.done:
		return false
	case c.send <- wsMessage{msgType: msgType, data: data, isInitState: isInitState}:
		return true
	default:
		// Buffer full, client is slow - drop message
		c.log.Debug().Str("msgType", msgType).Msg("dropping message, client buffer full")

		return false
	}
}

// UpdateSubscriptions updates what message types this client receives.
func (c *wsClient) UpdateSubscriptions(subs map[string]bool) {
	c.subMu.Lock()
	defer c.subMu.Unlock()

	maps.Copy(c.subscriptions, subs)
}

// Close gracefully shuts down the client.
func (c *wsClient) Close() {
	c.closeOnce.Do(func() {
		close(c.done)
	})
}

// IsClosed returns true if the client has been closed.
func (c *wsClient) IsClosed() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

// writePump handles all writes to the websocket connection.
// It runs in its own goroutine and is the only code that writes to the connection.
func (c *wsClient) writePump() {
	pingTicker := time.NewTicker(5 * time.Second)

	defer func() {
		pingTicker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case <-c.done:
			return

		case msg, ok := <-c.send:
			if !ok {
				// Channel closed, send close message and exit
				_ = c.conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})

				return
			}

			if msg.isPing {
				_ = c.conn.SetWriteDeadline(time.Now().Add(3 * time.Second))

				err := c.conn.WriteMessage(websocket.PingMessage, nil)
				if err != nil {
					c.log.Debug().Err(err).Msg("failed to send ping")

					return
				}

				continue
			}

			// Check subscription (unless it's initial state)
			if !msg.isInitState {
				c.subMu.RLock()
				subscribed := c.subscriptions[msg.msgType]
				c.subMu.RUnlock()

				if !subscribed {
					continue
				}
			}

			_ = c.conn.SetWriteDeadline(time.Now().Add(3 * time.Second))

			err := c.conn.WriteMessage(websocket.TextMessage, msg.data)
			if err != nil {
				c.log.Debug().Err(err).Msg("failed to send message")

				return
			}

		case <-pingTicker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(3 * time.Second))

			err := c.conn.WriteMessage(websocket.PingMessage, nil)
			if err != nil {
				c.log.Debug().Err(err).Msg("failed to send ping")

				return
			}
		}
	}
}

// WebUI defines the web user interface.
type WebUI struct {
	log                zerolog.Logger
	port               int
	webSocketClients   int
	telemetryChartFeed chan map[string]float32
	vehicleInfoFeed    chan map[string]any
	currentVehicleInfo map[string]any
	vehicleInfoMutex   sync.RWMutex
	gameStateFeed      chan string
	currentGameState   string
	gameStateMutex     sync.RWMutex
	circuitInfoFeed    chan map[string]string
	currentCircuitInfo map[string]string
	circuitInfoMutex   sync.RWMutex
	raceInfoFeed       chan map[string]any
	currentRaceInfo    map[string]any
	raceInfoMutex      sync.RWMutex
	config             *appconfig.Config
	calibrator         *calibrator.ToneGenerator
	upgrader           websocket.Upgrader
	shutdownChan       chan exitcode.Code
	setupMode          *setupmode.SetupMode
	logStore           *logstore.Store
	logStatsFeed       chan map[string]any
	currentLogStats    map[string]any
	logStatsMutex      sync.RWMutex
	buildVersion       string
	buildCommitHash    string
	buildTime          string
	buildPlatform      string
	// Unified WebSocket support
	unifiedClients     []*wsClient
	unifiedClientsMux  sync.RWMutex
	unifiedClientsChan chan *wsClient
	unifiedUnsubChan   chan *wsClient
	unifiedSessions    map[string]*wsClient // Track sessions to prevent duplicates
	unifiedSessionsMux sync.Mutex
	updater            *updater.Updater // Self-update manager (may be nil)
	// Shutdown support
	done      chan struct{}
	closeOnce sync.Once
}

type Config struct {
	Log                zerolog.Logger
	Port               int
	TelemetryChartFeed chan map[string]float32
	VehicleInfoFeed    chan map[string]any
	CircuitInfoFeed    chan map[string]string
	RaceInfoFeed       chan map[string]any
	GameStateFeed      chan string
	LogStatsFeed       chan map[string]any
	Config             *appconfig.Config
	Calibrator         *calibrator.ToneGenerator
	ShutdownChan       chan exitcode.Code
	SetupMode          *setupmode.SetupMode
	LogStore           *logstore.Store
	BuildVersion       string
	BuildCommitHash    string
	BuildTime          string
	BuildPlatform      string
	Updater            *updater.Updater // Self-update manager (may be nil)
}

// New creates a new instance of the WebUI.
func New(config Config) *WebUI {
	webUI := &WebUI{
		log:                config.Log.With().Str("component", "web ui").Logger(),
		port:               config.Port,
		webSocketClients:   0,
		telemetryChartFeed: config.TelemetryChartFeed,
		vehicleInfoFeed:    config.VehicleInfoFeed,
		currentVehicleInfo: make(map[string]any),
		gameStateFeed:      config.GameStateFeed,
		currentGameState:   "unknown",
		circuitInfoFeed:    config.CircuitInfoFeed,
		currentCircuitInfo: make(map[string]string),
		raceInfoFeed:       config.RaceInfoFeed,
		currentRaceInfo:    make(map[string]any),
		config:             config.Config,
		calibrator:         config.Calibrator,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(_ *http.Request) bool { return true },
		},
		shutdownChan:       config.ShutdownChan,
		setupMode:          config.SetupMode,
		logStore:           config.LogStore,
		logStatsFeed:       config.LogStatsFeed,
		currentLogStats:    make(map[string]any),
		buildVersion:       config.BuildVersion,
		buildCommitHash:    config.BuildCommitHash,
		buildTime:          config.BuildTime,
		buildPlatform:      config.BuildPlatform,
		unifiedClients:     make([]*wsClient, 0),
		unifiedClientsChan: make(chan *wsClient, 10),
		unifiedUnsubChan:   make(chan *wsClient, 10),
		unifiedSessions:    make(map[string]*wsClient),
		updater:            config.Updater,
		done:               make(chan struct{}),
	}

	// Start unified websocket broadcaster
	go webUI.unifiedWebSocketBroadcaster()

	return webUI
}

// GetHTTPHandler returns the HTTP handler for the web UI.
func (w *WebUI) GetHTTPHandler() http.Handler {
	// Create a new ServeMux with all routes
	mux := http.NewServeMux()
	mux.HandleFunc("/", w.htmlRouterHandlerFunc())
	mux.HandleFunc("/css/", w.cssHandlerFunc())
	mux.HandleFunc("/images/", w.imagesHandlerFunc())
	mux.HandleFunc("/js/", w.sciChartJSHandlerFunc())
	mux.HandleFunc("/ws", w.handleWebSocketConnection)
	mux.HandleFunc("/api/calibration/sweep", w.handleCalibrationSweep)
	mux.HandleFunc("/api/config", w.handleConfigAPI)
	mux.HandleFunc("/api/config/export", w.handleConfigExport)
	mux.HandleFunc("/api/config/import", w.handleConfigImport)
	mux.HandleFunc("/api/config/reset", w.handleConfigReset)
	mux.HandleFunc("/api/config/status", w.handleConfigStatus)
	mux.HandleFunc("/api/i18n", w.handleI18nAPI)
	mux.HandleFunc("/api/languages", w.handleLanguagesAPI)
	mux.HandleFunc("/api/logs", w.handleLogsAPI)
	mux.HandleFunc("/api/system/cache-clear", w.handleCacheClear)
	mux.HandleFunc("/api/system/cache-size", w.handleCacheSize)
	mux.HandleFunc("/api/system/info", w.handleSystemInfo)
	mux.HandleFunc("/api/system/restart", w.handleRestart)

	// Update management endpoints
	mux.HandleFunc("/api/updates/status", w.handleUpdatesStatus)
	mux.HandleFunc("/api/updates/check", w.handleUpdatesCheck)
	mux.HandleFunc("/api/updates/download", w.handleUpdatesDownload)
	mux.HandleFunc("/api/updates/upload", w.handleUpdatesUpload)
	mux.HandleFunc("/api/updates/install", w.handleUpdatesInstall)
	mux.HandleFunc("/api/updates/rollback", w.handleUpdatesRollback)

	if w.setupMode != nil && w.setupMode.IsAvailable() {
		mux.HandleFunc("/api/system/factory-reset", w.handleFactoryReset)
		mux.HandleFunc("/api/system/ssh/enable", w.handleSSHEnable)
		mux.HandleFunc("/api/system/ssh/disable", w.handleSSHDisable)
		mux.HandleFunc("/api/system/ssh/provision", w.handleSSHProvision)
		mux.HandleFunc("/api/mode/setup", w.handleSetupMode)
	}

	w.log.Debug().Msg("Web UI handler configured")

	// Wrap with CORS middleware
	return w.corsMiddleware(mux)
}

// HasActiveClients returns true if there are active WebSocket clients connected.
func (w *WebUI) HasActiveClients() bool {
	w.unifiedClientsMux.RLock()
	hasUnified := len(w.unifiedClients) > 0
	w.unifiedClientsMux.RUnlock()

	return hasUnified || w.webSocketClients > 0
}

// Close gracefully shuts down the WebUI, closing all WebSocket clients
// and stopping the broadcaster goroutine.
func (w *WebUI) Close() {
	w.closeOnce.Do(func() {
		w.log.Info().Msg("Closing WebUI")

		// Signal broadcaster to stop
		close(w.done)

		// Close all websocket clients
		w.unifiedClientsMux.Lock()

		for _, client := range w.unifiedClients {
			client.Close()
		}

		w.unifiedClients = nil
		w.unifiedClientsMux.Unlock()

		w.log.Debug().Msg("WebUI closed")
	})
}

//go:embed html/*
var htmlFiles embed.FS

// htmlRouterHandlerFunc serves HTML pages based on the request path.
func (w *WebUI) htmlRouterHandlerFunc() func(w http.ResponseWriter, r *http.Request) {
	return func(response http.ResponseWriter, request *http.Request) {
		path := request.URL.Path

		// Don't serve API paths as HTML - they should be handled by specific handlers
		if strings.HasPrefix(path, "/api/") {
			response.WriteHeader(http.StatusNotFound)
			w.log.Debug().Str("path", path).Msg("API endpoint not found")

			return
		}

		var filename string
		if path == "/" {
			filename = "index.html"
		} else {
			filename = path[1:] + ".html"
		}

		// Restrict access to dev.html if developer tools are not enabled
		if filename == "dev.html" && !w.config.GetDevToolsEnabled() {
			response.WriteHeader(http.StatusForbidden)
			w.log.Debug().Str("path", path).Msg("access to developer page denied - dev tools not enabled")

			return
		}

		content, err := htmlFiles.ReadFile("html/" + filename)
		if err != nil {
			response.WriteHeader(http.StatusNotFound)
			w.log.Error().Err(err).Str("file", filename).Str("path", path).Msg("HTML file not found")

			return
		}

		response.Header().Set("Content-Type", "text/html; charset=utf-8")

		length, err := response.Write(content)
		if err != nil {
			w.log.Error().Err(err).Int("bytes_written", length).Str("file", filename).Str("path", path).Msg("writing HTML response")

			return
		}

		w.log.Debug().Str("file", filename).Str("path", path).Int("bytes_written", length).Msg("served HTML page")
	}
}

// corsMiddleware adds CORS headers to all responses.
// TODO: figure out if this is needed, and if so perhaps make it more restrictive via config.
func (w *WebUI) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		// Set CORS headers
		response.Header().Set("Access-Control-Allow-Origin", "*")
		response.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		response.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight requests
		if request.Method == http.MethodOptions {
			response.WriteHeader(http.StatusOK)

			return
		}

		// Call the next handler
		next.ServeHTTP(response, request)
	})
}

//go:embed static/*
var staticFiles embed.FS

// staticFileHandlerFunc serves static files with automatic content type detection.
// It first tries to load from webui-specific static files, then falls back to shared files.
func (w *WebUI) staticFileHandlerFunc(fileType string) func(w http.ResponseWriter, r *http.Request) {
	return func(response http.ResponseWriter, request *http.Request) {
		filename := "static" + request.URL.Path

		// Try webui-specific files first
		content, err := staticFiles.ReadFile(filename)
		if err != nil {
			// Fall back to shared files
			content, err = webcommon.StaticFiles.ReadFile(filename)
			if err != nil {
				response.WriteHeader(http.StatusNotFound)
				w.log.Error().Err(err).Str("type", fileType).Msg("Invalid file")

				return
			}
		}

		contentType := getContentType(filename)
		response.Header().Set("Content-Type", contentType)

		length, err := response.Write(content)
		if err != nil {
			w.log.Error().Err(err).Int("bytes_written", length).Str("type", fileType).Msg("writing file")

			return
		}

		w.log.Debug().Str("file", filename).Str("mime-type", contentType).Msg("static file served")
	}
}

// imagesHandlerFunc serves static image files.
func (w *WebUI) imagesHandlerFunc() func(w http.ResponseWriter, r *http.Request) {
	return w.staticFileHandlerFunc("image")
}

// sciChartJSHandlerFunc serves static JavaScript files.
func (w *WebUI) sciChartJSHandlerFunc() func(w http.ResponseWriter, r *http.Request) {
	return w.staticFileHandlerFunc("javascript")
}

// cssHandlerFunc serves static CSS files.
func (w *WebUI) cssHandlerFunc() func(w http.ResponseWriter, r *http.Request) {
	return w.staticFileHandlerFunc("CSS")
}

// handleWebSocketConnection upgrades the HTTP connection to a unified WebSocket
// that can handle multiple message types: telemetry, vehicle, circuit, race, and gameState.
func (w *WebUI) handleWebSocketConnection(response http.ResponseWriter, request *http.Request) {
	// Get session ID from query parameter
	sessionID := request.URL.Query().Get("session")

	// Close any existing connection for this session
	if sessionID != "" {
		w.unifiedSessionsMux.Lock()

		if oldClient, exists := w.unifiedSessions[sessionID]; exists {
			w.log.Debug().Str("session", sessionID).Msg("closing old unified connection for session")

			oldClient.Close()
			// Remove from clients list immediately
			w.unifiedUnsubChan <- oldClient
		}

		w.unifiedSessionsMux.Unlock()
	}

	conn, err := w.upgrader.Upgrade(response, request, nil)
	if err != nil {
		w.log.Error().Err(err).Msg("error upgrading unified websocket connection")

		return
	}

	// Create client with its own write pump
	client := newWSClient(conn, w.log)

	w.log.Debug().Str("session", sessionID).Msg("unified websocket connection established")

	// Track this session
	if sessionID != "" {
		w.unifiedSessionsMux.Lock()
		w.unifiedSessions[sessionID] = client
		w.unifiedSessionsMux.Unlock()
	}

	// Start the write pump goroutine
	go client.writePump()

	// Subscribe this client
	w.unifiedClientsChan <- client

	// Send current state immediately via the client's channel
	w.sendInitialState(client)

	defer func() {
		// Remove from session map
		if sessionID != "" {
			w.unifiedSessionsMux.Lock()
			delete(w.unifiedSessions, sessionID)
			w.unifiedSessionsMux.Unlock()
		}

		// Unsubscribe on disconnect
		w.unifiedUnsubChan <- client

		// Close the client (stops write pump)
		client.Close()
	}()

	// Set up read handling
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))

	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))

		return nil
	})

	conn.SetCloseHandler(func(code int, text string) error {
		w.log.Debug().Int("code", code).Str("text", text).Msg("unified websocket close message received")

		return nil
	})

	// Read loop - handles incoming messages and detects disconnects
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			w.log.Debug().Err(err).Msg("unified websocket read error, closing connection")

			break
		}

		_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))

		// Try to parse as subscription message
		var subMsg struct {
			Type          string          `json:"type"`
			Subscriptions map[string]bool `json:"subscriptions"`
		}

		err = json.Unmarshal(message, &subMsg)
		if err == nil && subMsg.Type == "subscribe" {
			client.UpdateSubscriptions(subMsg.Subscriptions)

			w.log.Debug().
				Interface("subscriptions", subMsg.Subscriptions).
				Msg("client updated subscriptions")
		}
	}
}

// sendInitialState sends current cached state to a newly connected client.
func (w *WebUI) sendInitialState(client *wsClient) {
	// Send current vehicle info
	w.vehicleInfoMutex.RLock()

	if len(w.currentVehicleInfo) > 0 {
		data, err := json.Marshal(WSMessage{
			Type:      "vehicle",
			Timestamp: time.Now().UnixMilli(),
			Data:      w.currentVehicleInfo,
		})
		if err == nil {
			client.Send("vehicle", data, true)
		}
	}

	w.vehicleInfoMutex.RUnlock()

	// Send current game state
	w.gameStateMutex.RLock()
	gameState := w.currentGameState
	w.gameStateMutex.RUnlock()

	if gameState != "" {
		data, err := json.Marshal(WSMessage{
			Type:      "gameState",
			Timestamp: time.Now().UnixMilli(),
			Data:      map[string]any{"gamestate": gameState},
		})
		if err == nil {
			client.Send("gameState", data, true)
		}
	}

	// Send current circuit info
	w.circuitInfoMutex.RLock()

	if len(w.currentCircuitInfo) > 0 {
		data, err := json.Marshal(WSMessage{
			Type:      "circuit",
			Timestamp: time.Now().UnixMilli(),
			Data:      w.currentCircuitInfo,
		})
		if err == nil {
			client.Send("circuit", data, true)
		}
	}

	w.circuitInfoMutex.RUnlock()

	// Send current race info
	w.raceInfoMutex.RLock()

	if len(w.currentRaceInfo) > 0 {
		data, err := json.Marshal(WSMessage{
			Type:      "race",
			Timestamp: time.Now().UnixMilli(),
			Data:      w.currentRaceInfo,
		})
		if err == nil {
			client.Send("race", data, true)
		}
	}

	w.raceInfoMutex.RUnlock()

	// Send current log stats
	w.logStatsMutex.RLock()

	if len(w.currentLogStats) > 0 {
		data, err := json.Marshal(WSMessage{
			Type:      "logStats",
			Timestamp: time.Now().UnixMilli(),
			Data:      w.currentLogStats,
		})
		if err == nil {
			client.Send("logStats", data, true)
		}
	}

	w.logStatsMutex.RUnlock()
}

// unifiedWebSocketBroadcaster manages the unified websocket, handling all message types.
func (w *WebUI) unifiedWebSocketBroadcaster() {
	// Batch configuration for telemetry
	batchFrameRate := 30
	bufferSize := batchFrameRate / 60
	batchInterval := time.Duration(1000/batchFrameRate) * time.Millisecond
	batchBuffer := make([]map[string]float32, 0, bufferSize)

	// Track sequence ID for telemetry deduplication
	sid := 0

	// Ticker for batched telemetry sends
	ticker := time.NewTicker(batchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.done:
			w.log.Debug().Msg("Broadcaster received shutdown signal")

			return

		case client := <-w.unifiedClientsChan:
			// Add new client (subscriptions are managed by the client itself)
			w.unifiedClientsMux.Lock()
			w.unifiedClients = append(w.unifiedClients, client)
			w.unifiedClientsMux.Unlock()

			w.log.Debug().Int("unified_clients", len(w.unifiedClients)).Msg("unified client subscribed")

		case client := <-w.unifiedUnsubChan:
			// Remove client from unified clients list
			w.unifiedClientsMux.Lock()

			for i, c := range w.unifiedClients {
				if c == client {
					w.unifiedClients = append(w.unifiedClients[:i], w.unifiedClients[i+1:]...)

					break
				}
			}

			w.unifiedClientsMux.Unlock()

			w.log.Debug().Int("unified_clients", len(w.unifiedClients)).Msg("unified client unsubscribed")

		case data := <-w.telemetryChartFeed:
			// Handle telemetry data - add to batch buffer
			if sid != 0 {
				diff := int(data["seq"]) - sid
				if diff == 0 {
					continue // Skip duplicate
				}
			}

			sid = int(data["seq"])
			batchBuffer = append(batchBuffer, data)

		case <-ticker.C:
			// Send batched telemetry data
			if len(batchBuffer) == 0 {
				continue
			}

			msg := WSMessage{
				Type:      "telemetry",
				Timestamp: time.Now().UnixMilli(),
				Data:      batchBuffer,
			}

			encodedData, err := json.Marshal(msg)
			if err != nil {
				w.log.Error().Err(err).Msg("failed to encode batched telemetry JSON")

				batchBuffer = batchBuffer[:0]

				continue
			}

			// Broadcast to all unified clients
			w.broadcastToUnifiedClients(encodedData, "telemetry")

			// Clear buffer
			batchBuffer = batchBuffer[:0]

		case vehicleInfo := <-w.vehicleInfoFeed:
			// Store current state with mutex protection
			w.vehicleInfoMutex.Lock()
			w.currentVehicleInfo = vehicleInfo
			w.vehicleInfoMutex.Unlock()

			// Broadcast vehicle info
			msg := WSMessage{
				Type:      "vehicle",
				Timestamp: time.Now().UnixMilli(),
				Data:      vehicleInfo,
			}

			encodedData, err := json.Marshal(msg)
			if err != nil {
				w.log.Error().Err(err).Msg("failed to encode vehicle info JSON")

				continue
			}

			w.broadcastToUnifiedClients(encodedData, "vehicle")

			if len(vehicleInfo) > 0 {
				manufacturer, _ := vehicleInfo["manufacturer"].(string)
				model, _ := vehicleInfo["model"].(string)
				carID, _ := vehicleInfo["carID"].(uint32)

				w.log.Debug().
					Str("manufacturer", manufacturer).
					Str("model", model).
					Uint32("carID", carID).
					Int("clients", len(w.unifiedClients)).
					Msg("broadcast vehicle info to unified clients")
			}

		case gameState := <-w.gameStateFeed:
			// Store current state with mutex protection
			w.gameStateMutex.Lock()
			w.currentGameState = gameState
			w.gameStateMutex.Unlock()

			// Broadcast game state
			msg := WSMessage{
				Type:      "gameState",
				Timestamp: time.Now().UnixMilli(),
				Data:      map[string]any{"gamestate": gameState},
			}

			encodedData, err := json.Marshal(msg)
			if err != nil {
				w.log.Error().Err(err).Msg("failed to encode game state JSON")

				continue
			}

			w.broadcastToUnifiedClients(encodedData, "gameState")

			w.log.Debug().
				Str("gameState", gameState).
				Int("clients", len(w.unifiedClients)).
				Msg("broadcast game state to unified clients")

		case circuitInfo := <-w.circuitInfoFeed:
			w.log.Debug().Interface("circuitInfo", circuitInfo).Msg("received circuit info from channel")

			// Store current state with mutex protection
			w.circuitInfoMutex.Lock()
			w.currentCircuitInfo = circuitInfo
			w.circuitInfoMutex.Unlock()

			// Broadcast circuit info
			msg := WSMessage{
				Type:      "circuit",
				Timestamp: time.Now().UnixMilli(),
				Data:      circuitInfo,
			}

			encodedData, err := json.Marshal(msg)
			if err != nil {
				w.log.Error().Err(err).Msg("failed to encode circuit info JSON")

				continue
			}

			w.broadcastToUnifiedClients(encodedData, "circuit")

			if len(circuitInfo) > 0 {
				name := circuitInfo["name"]
				length := circuitInfo["length"]

				w.log.Debug().
					Str("circuit", name).
					Str("length", length).
					Int("clients", len(w.unifiedClients)).
					Msg("broadcast circuit info to unified clients")
			}

		case raceInfo := <-w.raceInfoFeed:
			// Store current state with mutex protection
			w.raceInfoMutex.Lock()
			w.currentRaceInfo = raceInfo
			w.raceInfoMutex.Unlock()

			// Broadcast race info
			msg := WSMessage{
				Type:      "race",
				Timestamp: time.Now().UnixMilli(),
				Data:      raceInfo,
			}

			encodedData, err := json.Marshal(msg)
			if err != nil {
				w.log.Error().Err(err).Msg("failed to encode race info JSON")

				continue
			}

			w.broadcastToUnifiedClients(encodedData, "race")

			if len(raceInfo) > 0 {
				lap, _ := raceInfo["lap"].(int)
				totalLaps, _ := raceInfo["totalLaps"].(int)

				w.log.Debug().
					Int("lap", lap).
					Int("totalLaps", totalLaps).
					Int("clients", len(w.unifiedClients)).
					Msg("broadcast race info to unified clients")
			}

		case logStats := <-w.logStatsFeed:
			// Store current state with mutex protection
			w.logStatsMutex.Lock()
			w.currentLogStats = logStats
			w.logStatsMutex.Unlock()

			// Broadcast log stats
			msg := WSMessage{
				Type:      "logStats",
				Timestamp: time.Now().UnixMilli(),
				Data:      logStats,
			}

			encodedData, err := json.Marshal(msg)
			if err != nil {
				w.log.Error().Err(err).Msg("failed to encode log stats JSON")

				continue
			}

			w.broadcastToUnifiedClients(encodedData, "logStats")

			totalCount, _ := logStats["totalCount"].(int)
			w.log.Debug().
				Int("totalCount", totalCount).
				Int("clients", len(w.unifiedClients)).
				Msg("broadcast log stats to unified clients")
		}
	}
}

// broadcastToUnifiedClients sends a message to all connected unified websocket clients.
// Each client's writePump handles subscription filtering and the actual write.
// messageType specifies what type of data is being sent (e.g., "telemetry", "vehicle", etc.)
func (w *WebUI) broadcastToUnifiedClients(encodedData []byte, messageType string) {
	w.unifiedClientsMux.Lock()
	defer w.unifiedClientsMux.Unlock()

	activeClients := make([]*wsClient, 0, len(w.unifiedClients))

	for _, client := range w.unifiedClients {
		if client.IsClosed() {
			continue
		}

		// Queue message to client's write channel (non-blocking)
		// The client's writePump handles subscription filtering
		if client.Send(messageType, encodedData, false) {
			activeClients = append(activeClients, client)
		} else {
			// Client is closed or buffer full, remove it
			w.log.Debug().Str("msgType", messageType).Msg("client unavailable, removing")
		}
	}

	w.unifiedClients = activeClients
}

// getContentType returns the appropriate MIME type based on file extension using the standard library.
func getContentType(filename string) string {
	ext := filepath.Ext(filename)
	contentType := mime.TypeByExtension(ext)

	if contentType == "" {
		return "application/octet-stream"
	}

	return contentType
}

// handleConfigAPI handles GET and POST requests for configuration management.
func (w *WebUI) handleConfigAPI(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Content-Type", "application/json")

	switch request.Method {
	case http.MethodGet:
		w.handleGetConfig(response, request)
	case http.MethodPost:
		w.handleSetConfig(response, request)
	default:
		response.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(response).Encode(map[string]string{"error": "Method not allowed"}) //nolint:errchkjson // simple encoding
	}
}

// handleGetConfig returns the current configuration as JSON.
func (w *WebUI) handleGetConfig(response http.ResponseWriter, _ *http.Request) {
	configData := map[string]any{
		"app": map[string]any{
			"language":       *w.config.GetAppLanguage(),
			"accent":         w.config.GetAppAccent(),
			"logLevel":       w.config.GetAppLogLevel(),
			"baseDir":        w.config.GetAppBaseDir(),
			"vehicleDBFile":  w.config.GetAppVehicleDBFile(),
			"enabledWebUI":   w.config.GetAppWebUIEnabled(),
			"webUIPort":      w.config.GetAppWebUIPort(),
			"enableDevTools": w.config.GetDevToolsEnabled(),
			"updates": map[string]any{
				"channel": w.config.GetAppUpdateChannel(),
			},
		},
		"discord": map[string]any{
			"token":          w.config.GetDiscordToken(),
			"guildID":        w.config.GetDiscordGuildID(),
			"channelID":      w.config.GetDiscordChannelID(),
			"voiceChannelID": w.config.GetDiscordVoiceChannelID(),
		},
		"hardware": map[string]any{
			"model":              w.config.GetHardwareModel(),
			"displayOrientation": w.config.GetDisplayOrientation(),
		},
		"haptics": map[string]any{
			"enableReplay":                 w.config.GetHapticsReplayEnabled(),
			"pitRadioOutput":               w.config.GetPitRadioOutput(),
			"dynamicTransmissionFeedback":  w.config.GethapticsDynamicTransFeedbackEnabled(),
			"dynamicTransmissionCurve":     w.config.GetHapticsTransmissionCurve(),
			"dynamicTransmissionGforceMax": w.config.GetHapticsTransmissionGforceMax(),
			"jerkCurve":                    w.config.GethapticsJerkCurve(),
			"jerkMax":                      w.config.GetHapticsJerkMax(),
			"snapCurve":                    w.config.GetHapticsSnapCurve(),
			"snapMax":                      w.config.GetHapticsSnapMax(),
			"pulseMaxAmplitude":            w.config.GetHapticsPulseMaxAmplitude(),
			"pulseMaxFrequencyHz":          w.config.GetHapticsPulseMaxHz(),
			"pulseMinFrequencyHz":          w.config.GetHapticsPulseMinHz(),
		},
		"pitRadio": map[string]any{
			"enabled":               w.config.PitRadioEnabled(),
			"messageSendIntervalMs": w.config.GetPitRadioMessageSendIntervalMs(),
			"notifications": map[string]any{
				"enableRaceProgress":      w.config.GetPitRadioNotifyRaceProgressEnabled(),
				"raceProgressMinLaps":     w.config.GetPitRadioNotifyRaceProgressMinLaps(),
				"raceProgressIntervalPc":  w.config.GetPitRadioNotifyRaceProgressIntervalPc(),
				"enableRaceLaps":          w.config.GetPitRadioNotifyRaceLapsEnabled(),
				"raceLapsIntervalLaps":    w.config.GetPitRadioNotifyRaceLapsIntervalLaps(),
				"raceLapsCountdownLaps":   w.config.GetPitRadioNotifyRaceLapsCountdownLaps(),
				"enableLapTimes":          w.config.GetPitRadioNotifyLapTimesEnabled(),
				"lapTimesMaxDeltaSeconds": w.config.GetPitRadioNotifyLapTimesMaxDeltaSeconds(),
				"enableCircuitMatching":   w.config.GetPitRadioNotifyCircuitMatchingEnabled(),
			},
			"fuelMonitoring": map[string]any{
				"enabled":                 w.config.GetPitRadioFuelMonitoringEnabled(),
				"preWarnNotifyLaps":       w.config.GetPitRadioFuelPreWarnNotifyLaps(),
				"strategyNotifyLaps":      w.config.GetPitRadioFuelStrategyNotifyLaps(),
				"rangeSafetyMarginLaps":   w.config.GetPitRadioFuelRangeSafetyMarginLaps(),
				"rangeSafetyMarginMetres": w.config.GetPitRadioFuelRangeSafetyMarginMetres(),
			},
			"tyreMonitoring": map[string]any{
				"enabled":                    w.config.GetPitRadioTyreMonitoringEnabled(),
				"temperatureOptimalCelsius":  w.config.GetPitRadioTyreTemperatureOptimalCelsius(),
				"temperatureOperatingWindow": w.config.GetPitRadioTyreTemperatureOperatingWindow(),
				"temperatureMarginCelsius":   w.config.GetPitRadioTyreTemperatureMarginCelsius(),
			},
			"discord": map[string]any{
				"token":          w.config.GetDiscordToken(),
				"guildID":        w.config.GetDiscordGuildID(),
				"channelID":      w.config.GetDiscordChannelID(),
				"voiceChannelID": w.config.GetDiscordVoiceChannelID(),
			},
		},
		"synthesizer": map[string]any{
			"internalSampleRateHz":      w.config.GetSynthInternalSampleRateHz(),
			"outputSampleRateHz":        w.config.GetSynthOutputSampleRateHz(),
			"outputFile":                w.config.GetSynthOutputFile(),
			"masterMute":                w.config.GetSynthMasterMute(),
			"masterGain":                w.config.GetSynthMasterGain(),
			"channel0Mute":              w.config.GetSynthChannelMute(0),
			"channel0Gain":              w.config.GetSynthChannelGain(0),
			"channel1Mute":              w.config.GetSynthChannelMute(1),
			"channel1Gain":              w.config.GetSynthChannelGain(1),
			"chassisMute":               w.config.GetSynthChassisMute(),
			"chassisGain":               w.config.GetSynthChassisGain(),
			"transmissionMute":          w.config.GetSynthTransmissionMute(),
			"transmissionGain":          w.config.GetSynthTransmissionGain(),
			"transmissionGainMinRace":   w.config.GetSynthTransmissionGainMinRace(),
			"transmissionGainMinStreet": w.config.GetSynthTransmissionGainMinStreet(),
			"engineMute":                w.config.GetSynthEngineMute(),
			"engineGain":                w.config.GetSynthEngineGain(),
			"gainIncrement":             w.config.GetSynthGainIncrement(),
			"engineProfiles":            w.config.GetSynthEngineProfiles(),
			"enableEQ":                  w.config.GetSynthChannelsEqEnabled(),
			"eq":                        w.config.GetSynthChannelsEq(),
		},
		"eqCurve": func() map[string]any {
			curves, minFreq, resolution := w.config.GetSynthChannelsEqCurve()

			return map[string]any{
				"curve":      curves,
				"minFreq":    minFreq,
				"resolution": resolution,
			}
		}(),
		"telemetry": map[string]any{
			"source": w.config.GetTelemetrySource(),
		},
		"calibration": map[string]any{
			"enabled":       w.calibrator.IsEnabled(),
			"frequency":     w.calibrator.GetSweepFrequency(),
			"sweeping":      w.calibrator.IsSweeping(),
			"sweepMin":      w.calibrator.GetSweepMin(),
			"sweepMax":      w.calibrator.GetSweepMax(),
			"sweepDuration": w.calibrator.GetSweepDuration(),
		},
	}

	err := json.NewEncoder(response).Encode(configData)
	if err != nil {
		w.log.Error().Err(err).Msg("failed to encode config JSON")
		response.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(response).Encode(map[string]string{"error": "Failed to encode configuration"}) //nolint:errchkjson // simple encoding
	}
}

// handleSetConfig updates the configuration from JSON data.
func (w *WebUI) handleSetConfig(response http.ResponseWriter, request *http.Request) {
	var configData map[string]any

	err := json.NewDecoder(request.Body).Decode(&configData)
	if err != nil {
		w.log.Error().Err(err).Msg("failed to decode config JSON")
		response.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(response).Encode(map[string]string{"error": "Invalid JSON data"}) //nolint:errchkjson // simple encoding

		return
	}

	// Debug: Log the incoming configuration data
	w.log.Debug().Interface("configData", configData).Msg("received config update request")

	// Check if restart is required BEFORE applying changes
	restartRequired := w.checkRestartRequired(configData)

	// Apply configuration changes using setter methods
	errors := w.applyConfigChanges(configData)

	if len(errors) > 0 {
		w.log.Error().Strs("errors", errors).Msg("failed to apply some configuration changes")
		response.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(response).Encode(map[string]any{ //nolint:errchkjson // simple encoding
			"status":  "partial_success",
			"message": "Some configuration changes failed",
			"errors":  errors,
		})

		return
	}

	w.log.Debug().Interface("config", configData).Msg("configuration updated successfully")

	// Save configuration to file
	err = w.config.SaveConfigToFile()
	if err != nil {
		w.log.Error().Err(err).Msg("failed to save configuration to file")
		response.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(response).Encode(map[string]any{ //nolint:errchkjson // simple encoding
			"status":  "error",
			"message": "Configuration updated but failed to save: " + err.Error(),
		})

		return
	}

	w.log.Debug().Msg("configuration saved to file")

	// Return success response with updated config including EQ curve and calibration state
	_ = json.NewEncoder(response).Encode(map[string]any{ //nolint:errchkjson // simple encoding
		"status":          "success",
		"message":         "Configuration updated and saved successfully",
		"restartRequired": restartRequired,
		"config": map[string]any{
			"eqCurve": func() map[string]any {
				curves, minFreq, resolution := w.config.GetSynthChannelsEqCurve()

				return map[string]any{
					"curve":      curves,
					"minFreq":    minFreq,
					"resolution": resolution,
				}
			}(),
			"calibration": map[string]any{
				"enabled":       w.calibrator.IsEnabled(),
				"frequency":     w.calibrator.GetSweepFrequency(),
				"sweeping":      w.calibrator.IsSweeping(),
				"sweepMin":      w.calibrator.GetSweepMin(),
				"sweepMax":      w.calibrator.GetSweepMax(),
				"sweepDuration": w.calibrator.GetSweepDuration(),
			},
		},
	})
}

// applyConfigChanges applies the configuration changes using appropriate setter methods.
func (w *WebUI) applyConfigChanges(configData map[string]any) []string {
	var errors []string

	// Process each section of the configuration
	for section, sectionData := range configData {
		sectionMap, ok := sectionData.(map[string]any)
		if !ok {
			errors = append(errors, "invalid data for section "+section)

			continue
		}

		switch section {
		case "app":
			errors = append(errors, w.applyAppConfig(sectionMap)...)
		case "synthesizer":
			errors = append(errors, w.applySynthesizerConfig(sectionMap)...)
		case "haptics":
			errors = append(errors, w.applyHapticsConfig(sectionMap)...)
		case "hardware":
			errors = append(errors, w.applyHardwareConfig(sectionMap)...)
		case "telemetry":
			errors = append(errors, w.applyTelemetryConfig(sectionMap)...)
		case "discord":
			errors = append(errors, w.applyDiscordConfig(sectionMap)...)
		case "calibration":
			errors = append(errors, w.applyCalibrationConfig(sectionMap)...)
		case "pitRadio":
			errors = append(errors, w.applyPitRadioConfig(sectionMap)...)
		// Add more sections as needed
		default:
			w.log.Debug().Str("section", section).Msg("configuration section not implemented for updates")
		}
	}

	return errors
}

// applyAppConfig applies application configuration changes.
func (w *WebUI) applyAppConfig(config map[string]any) []string {
	var errors []string

	if language, ok := config["language"]; ok {
		if langStr, ok := language.(string); ok {
			w.config.SetAppLanguage(langStr)
		} else {
			errors = append(errors, "invalid language value")
		}
	}

	if accent, ok := config["accent"]; ok {
		if accentStr, ok := accent.(string); ok {
			w.config.SetAppAccent(accentStr)
		} else {
			errors = append(errors, "invalid accent value")
		}
	}

	if logLevel, ok := config["logLevel"]; ok {
		if levelStr, ok := logLevel.(string); ok {
			w.config.SetAppLogLevel(levelStr)
		} else {
			errors = append(errors, "invalid log level value")
		}
	}

	if baseDir, ok := config["baseDir"]; ok {
		if baseDirStr, ok := baseDir.(string); ok {
			w.config.SetAppBaseDir(baseDirStr)
		} else {
			errors = append(errors, "invalid base directory value")
		}
	}

	if enableDevTools, ok := config["enableDevTools"]; ok {
		if enableDevToolsBool, ok := enableDevTools.(bool); ok {
			w.config.SetDevToolsEnabled(enableDevToolsBool)
		} else {
			errors = append(errors, "invalid enableDevTools value")
		}
	}

	// Handle updates config
	if updates, ok := config["updates"]; ok {
		if updatesMap, ok := updates.(map[string]any); ok {
			if autoCheck, ok := updatesMap["autoCheck"]; ok {
				if autoCheckBool, ok := autoCheck.(bool); ok {
					w.config.SetAppUpdateAutoCheck(autoCheckBool)
				} else {
					errors = append(errors, "invalid autoCheck value")
				}
			}

			if autoInstall, ok := updatesMap["autoInstall"]; ok {
				if autoInstallBool, ok := autoInstall.(bool); ok {
					w.config.SetAppUpdateAutoInstall(autoInstallBool)
				} else {
					errors = append(errors, "invalid autoInstall value")
				}
			}

			if checkIntervalMinutes, ok := updatesMap["checkIntervalMinutes"]; ok {
				if intervalFloat, ok := checkIntervalMinutes.(float64); ok {
					w.config.SetAppUpdateCheckIntervalMinutes(int(intervalFloat))
				} else {
					errors = append(errors, "invalid checkIntervalMinutes value")
				}
			}

			if channel, ok := updatesMap["channel"]; ok {
				if channelStr, ok := channel.(string); ok {
					w.config.SetAppUpdateChannel(channelStr)
					// Update the updater's channel immediately
					// TODO: this should probably be done outside of the config update function
					if w.updater != nil {
						w.updater.SetChannel(channelStr)
						// Check for existing downloads in the new channel
						w.updater.CheckExistingDownloads()
					}
				} else {
					errors = append(errors, "invalid channel value")
				}
			}
		} else {
			errors = append(errors, "invalid updates configuration")
		}
	}

	vehicleDBFile, fileOK := config["vehicleDBFile"]
	if !fileOK {
		return errors
	}

	vehicleDBFileStr, strOK := vehicleDBFile.(string)
	if !strOK {
		errors = append(errors, "invalid vehicle database file value")

		return errors
	}

	// If a file path is provided, validate that it exists
	if vehicleDBFileStr != "" {
		_, err := os.Stat(vehicleDBFileStr)
		if err != nil {
			if os.IsNotExist(err) {
				errors = append(errors, "vehicle database file not found: "+vehicleDBFileStr)
			} else {
				errors = append(errors, "cannot access vehicle database file: "+err.Error())
			}

			return errors
		}
	}

	w.config.SetAppVehicleDBFile(vehicleDBFileStr)

	return errors
}

// applySynthesizerConfig applies synthesizer configuration changes.
func (w *WebUI) applySynthesizerConfig(config map[string]any) []string {
	var errors []string

	w.log.Debug().Interface("synthConfig", config).Msg("applying synthesizer configuration")

	if internalSampleRate, ok := config["internalSampleRateHz"]; ok {
		w.log.Debug().Interface("value", internalSampleRate).Type("type", internalSampleRate).Msg("processing internalSampleRateHz")

		if rateFloat, ok := internalSampleRate.(float64); ok {
			w.config.SetSynthInternalSampleRateHz(int(rateFloat))
			w.log.Debug().Int("rate", int(rateFloat)).Msg("set internal sample rate")
		} else {
			errors = append(errors, "invalid internal sample rate value")

			w.log.Error().Interface("value", internalSampleRate).Msg("invalid internal sample rate value type")
		}
	}

	if outputSampleRate, ok := config["outputSampleRateHz"]; ok {
		w.log.Debug().Interface("value", outputSampleRate).Type("type", outputSampleRate).Msg("processing outputSampleRateHz")

		if rateFloat, ok := outputSampleRate.(float64); ok {
			w.config.SetSynthOutputSampleRateHz(int(rateFloat))
			w.log.Debug().Int("rate", int(rateFloat)).Msg("set output sample rate")
		} else {
			errors = append(errors, "invalid output sample rate value")

			w.log.Error().Interface("value", outputSampleRate).Msg("invalid output sample rate value type")
		}
	}

	if masterGain, ok := config["masterGain"]; ok {
		if gainFloat, ok := masterGain.(float64); ok {
			w.config.SetSynthMasterGain(gainFloat)
		} else {
			errors = append(errors, "invalid master gain value")
		}
	}

	if masterMute, ok := config["masterMute"]; ok {
		if mute, ok := masterMute.(bool); ok {
			w.config.SetSynthMasterMute(mute)
		} else {
			errors = append(errors, "invalid master gain mute value")
		}
	}

	if channel0Gain, ok := config["channel0Gain"]; ok {
		if gainFloat, ok := channel0Gain.(float64); ok {
			w.config.SetSynthChannelGain(0, gainFloat)
		} else {
			errors = append(errors, "invalid left channel gain value")
		}
	}

	if channel0Mute, ok := config["channel0Mute"]; ok {
		if mute, ok := channel0Mute.(bool); ok {
			w.config.SetSynthChannelMute(0, mute)
		} else {
			errors = append(errors, "invalid left channel mute value")
		}
	}

	if channel1Gain, ok := config["channel1Gain"]; ok {
		if gainFloat, ok := channel1Gain.(float64); ok {
			w.config.SetSynthChannelGain(1, gainFloat)
		} else {
			errors = append(errors, "invalid right channel gain value")
		}
	}

	if channel1Mute, ok := config["channel1Mute"]; ok {
		if mute, ok := channel1Mute.(bool); ok {
			w.config.SetSynthChannelMute(1, mute)
		} else {
			errors = append(errors, "invalid right channel mute value")
		}
	}

	if chassisGain, ok := config["chassisGain"]; ok {
		if gainFloat, ok := chassisGain.(float64); ok {
			w.config.SetSynthChassisGain(gainFloat)
		} else {
			errors = append(errors, "invalid chassis gain value")
		}
	}

	if chassisMute, ok := config["chassisMute"]; ok {
		if mute, ok := chassisMute.(bool); ok {
			w.config.SetSynthChassisMute(mute)
		} else {
			errors = append(errors, "invalid chassis gain mute value")
		}
	}

	if transmissionGain, ok := config["transmissionGain"]; ok {
		if gainFloat, ok := transmissionGain.(float64); ok {
			w.config.SetSynthTransmissionGain(gainFloat)
		} else {
			errors = append(errors, "invalid transmission gain value")
		}
	}

	if transmissionMute, ok := config["transmissionMute"]; ok {
		if mute, ok := transmissionMute.(bool); ok {
			w.config.SetSynthTransmissionMute(mute)
		} else {
			errors = append(errors, "invalid transmission gain mute value")
		}
	}

	if transmissionGainMinRace, ok := config["transmissionGainMinRace"]; ok {
		if gainFloat, ok := transmissionGainMinRace.(float64); ok {
			w.config.SetSynthTransmissionGainMinRace(gainFloat)
		} else {
			errors = append(errors, "invalid transmission gain min race value")
		}
	}

	if transmissionGainMinStreet, ok := config["transmissionGainMinStreet"]; ok {
		if gainFloat, ok := transmissionGainMinStreet.(float64); ok {
			w.config.SetSynthTransmissionGainMinStreet(gainFloat)
		} else {
			errors = append(errors, "invalid transmission gain min street value")
		}
	}

	if engineGain, ok := config["engineGain"]; ok {
		if gainFloat, ok := engineGain.(float64); ok {
			w.config.SetSynthEngineGain(gainFloat)
		} else {
			errors = append(errors, "invalid engine gain value")
		}
	}

	if engineMute, ok := config["engineMute"]; ok {
		if mute, ok := engineMute.(bool); ok {
			w.config.SetSynthEngineMute(mute)
		} else {
			errors = append(errors, "invalid engine gain mute value")
		}
	}

	if gainIncrement, ok := config["gainIncrement"]; ok {
		if incrementFloat, ok := gainIncrement.(float64); ok {
			w.config.SetSynthGainIncrement(incrementFloat)
		} else {
			errors = append(errors, "invalid gain increment value")
		}
	}

	// Handle engine profiles
	if engineProfiles, ok := config["engineProfiles"]; ok {
		if profilesMap, ok := engineProfiles.(map[string]any); ok {
			for name, profileData := range profilesMap {
				if profileMap, ok := profileData.(map[string]any); ok {
					profile := haptics.EngineProfile{}

					if pb, ok := profileMap["primaryBalance"].(float64); ok {
						profile.PrimaryBalance = pb
					}

					if sb, ok := profileMap["secondaryBalance"].(float64); ok {
						profile.SecondaryBalance = sb
					}

					if g, ok := profileMap["gain"].(float64); ok {
						profile.Gain = g
					}

					if ps, ok := profileMap["pulseScale"].(float64); ok {
						profile.PulseScale = ps
					}

					w.config.SetSynthEngineProfile(name, profile)
				}
			}
		} else {
			errors = append(errors, "invalid engine profiles format")
		}
	}

	if eqEnabled, ok := config["enableEQ"]; ok {
		if enabledArray, ok := eqEnabled.([]any); ok {
			for channel, val := range enabledArray {
				if enabled, ok := val.(bool); ok {
					w.config.SetSynthChannelEqEnabled(channel, enabled)
				} else {
					errors = append(errors, fmt.Sprintf("invalid EQ enabled value for channel %d", channel))
				}
			}
		} else {
			errors = append(errors, "invalid EQ enabled value (expected array)")
		}
	}

	// Handle EQ bands (per channel)
	if eq, ok := config["eq"]; ok {
		if channelArray, ok := eq.([]any); ok {
			for channel, channelVal := range channelArray {
				if eqArray, ok := channelVal.([]any); ok {
					eqBands := make([]appconfig.EQBand, 0, len(eqArray))
					for idx, val := range eqArray {
						if bandMap, ok := val.(map[string]any); ok {
							freq, freqOk := bandMap["frequency"].(float64)
							gain, gainOk := bandMap["gain"].(float64)
							qVal, qOk := bandMap["q"].(float64)

							if !freqOk || !gainOk || !qOk {
								errors = append(errors, fmt.Sprintf("invalid EQ band %d for channel %d: missing or invalid fields", idx+1, channel))

								continue // Skip this band but continue processing others
							}

							eqBands = append(eqBands, appconfig.EQBand{
								Frequency: freq,
								Gain:      gain,
								Q:         qVal,
							})
						} else {
							errors = append(errors, fmt.Sprintf("invalid EQ band %d format for channel %d", idx+1, channel))

							continue
						}
					}

					if len(eqBands) == 8 {
						w.config.SetSynthChannelEq(channel, eqBands)
					} else {
						errors = append(errors, fmt.Sprintf("EQ for channel %d must have exactly 8 bands, got %d", channel, len(eqBands)))
					}
				} else {
					errors = append(errors, fmt.Sprintf("invalid EQ format for channel %d", channel))
				}
			}
		} else {
			errors = append(errors, "invalid EQ format (expected array of channel arrays)")
		}
	}

	return errors
}

// applyHapticsConfig applies haptics configuration changes.
//
//nolint:cyclop // functio is easy to understand
func (w *WebUI) applyHapticsConfig(config map[string]any) []string {
	var errors []string

	// Helper for float64 conversion and error append
	parseFloat := func(val any, key string) (float64, bool) {
		f, ok := val.(float64)
		if !ok {
			errors = append(errors, "invalid "+key+" value")
		}

		return f, ok
	}

	// Helper for bool conversion
	parseBool := func(val any, key string) (bool, bool) {
		b, ok := val.(bool)
		if !ok {
			errors = append(errors, "invalid "+key+" value")
		}

		return b, ok
	}

	if dynamicTransmission, ok := config["dynamicTransmissionFeedback"]; ok {
		if dynamicBool, ok := parseBool(dynamicTransmission, "dynamic transmission feedback"); ok {
			w.config.SetHapticsDynamicTransFeedbackEnabled(dynamicBool)
		}
	}

	if jerkCurve, ok := config["jerkCurve"]; ok {
		if curveFloat, ok := parseFloat(jerkCurve, "jerk curve"); ok {
			w.config.SetHapticsJerkCurve(int(curveFloat * 1000.0))
		}
	}

	if jerkMax, ok := config["jerkMax"]; ok {
		if maxFloat, ok := parseFloat(jerkMax, "jerk max"); ok {
			w.config.SetHapticsJerkMax(int(maxFloat))
		}
	}

	if snapCurve, ok := config["snapCurve"]; ok {
		if curveFloat, ok := parseFloat(snapCurve, "snap curve"); ok {
			w.config.SetHapticsSnapCurve(int(curveFloat * 1000.0))
		}
	}

	if snapMax, ok := config["snapMax"]; ok {
		if maxFloat, ok := parseFloat(snapMax, "snap max"); ok {
			w.config.SetHapticsSnapMax(int(maxFloat))
		}
	}

	if transmissionCurve, ok := config["dynamicTransmissionCurve"]; ok {
		if curveFloat, ok := parseFloat(transmissionCurve, "transmission curve"); ok {
			w.config.SetHapticsTransmissionCurve(int(curveFloat * 1000.0))
		}
	}

	if transmissionGforceMax, ok := config["dynamicTransmissionGforceMax"]; ok {
		if gforceFloat, ok := parseFloat(transmissionGforceMax, "transmission G-force max"); ok {
			w.config.SetHapticsTransmissionGforceMax(gforceFloat)
		}
	}

	if pulseMaxAmplitude, ok := config["pulseMaxAmplitude"]; ok {
		if amplitudeFloat, ok := parseFloat(pulseMaxAmplitude, "pulse max amplitude"); ok {
			w.config.SetHapticsPulseMaxAmplitude(amplitudeFloat)
		}
	}

	if pulseMaxFreq, ok := config["pulseMaxFrequencyHz"]; ok {
		if freqFloat, ok := parseFloat(pulseMaxFreq, "pulse max frequency"); ok {
			w.config.SetHapticsPulseMaxFrequencyHz(freqFloat)
		}
	}

	if pulseMinFreq, ok := config["pulseMinFrequencyHz"]; ok {
		if freqFloat, ok := parseFloat(pulseMinFreq, "pulse min frequency"); ok {
			w.config.SetHapticsPulseMinFrequencyHz(freqFloat)
		}
	}

	if enableReplay, ok := config["enableReplay"]; ok {
		if replayBool, ok := parseBool(enableReplay, "enable replay"); ok {
			w.config.SetHapticsEnableReplay(replayBool)
		}
	}

	if pitRadioOutput, ok := config["pitRadioOutput"]; ok {
		if outputStr, ok := pitRadioOutput.(string); ok {
			w.config.SetPitRadioOutput(outputStr)
		} else {
			errors = append(errors, "invalid pit radio output value")
		}
	}

	return errors
}

// applyCalibrationConfig applies calibration configuration changes.
func (w *WebUI) applyCalibrationConfig(config map[string]any) []string {
	var errors []string

	if enabled, ok := config["enabled"]; ok {
		if enabledBool, ok := enabled.(bool); ok {
			w.calibrator.SetEnabled(enabledBool)

			// Broadcast calibration state change to all WebSocket clients
			calibrationState := map[string]any{
				"enabled":       w.calibrator.IsEnabled(),
				"frequency":     w.calibrator.GetSweepFrequency(),
				"volume":        w.calibrator.GetGain(),
				"channel":       string(w.calibrator.GetChannel()),
				"sweeping":      w.calibrator.IsSweeping(),
				"sweepMin":      w.calibrator.GetSweepMin(),
				"sweepMax":      w.calibrator.GetSweepMax(),
				"sweepDuration": w.calibrator.GetSweepDuration(),
			}

			msg := WSMessage{
				Type:      "calibration",
				Timestamp: time.Now().UnixMilli(),
				Data:      calibrationState,
			}

			encodedData, err := json.Marshal(msg)
			if err == nil {
				w.broadcastToUnifiedClients(encodedData, "calibration")
			}
		} else {
			errors = append(errors, "invalid calibration enabled value")
		}
	}

	if frequency, ok := config["frequency"]; ok {
		if freqFloat, ok := frequency.(float64); ok {
			w.calibrator.SetFrequency(freqFloat)
		} else {
			errors = append(errors, "invalid calibration frequency value")
		}
	}

	if sweepMin, ok := config["sweepMin"]; ok {
		if minFloat, ok := sweepMin.(float64); ok {
			w.calibrator.SetSweepMin(minFloat)
		} else {
			errors = append(errors, "invalid calibration sweepMin value")
		}
	}

	if sweepMax, ok := config["sweepMax"]; ok {
		if maxFloat, ok := sweepMax.(float64); ok {
			w.calibrator.SetSweepMax(maxFloat)
		} else {
			errors = append(errors, "invalid calibration sweepMax value")
		}
	}

	if sweepDuration, ok := config["sweepDuration"]; ok {
		if durationFloat, ok := sweepDuration.(float64); ok {
			w.calibrator.SetSweepDuration(durationFloat)
		} else {
			errors = append(errors, "invalid calibration sweepDuration value")
		}
	}

	return errors
}

// checkRestartRequired checks if any configuration changes require a restart.
func (w *WebUI) checkRestartRequired(configData map[string]any) bool {
	// Check if vehicleDBFile changed
	if appConfig, ok := configData["app"].(map[string]any); ok { //nolint:nestif // compact nesting
		if vehicleDBFile, ok := appConfig["vehicleDBFile"]; ok {
			if vehicleDBFileStr, ok := vehicleDBFile.(string); ok {
				if vehicleDBFileStr != w.config.GetAppVehicleDBFile() {
					return true
				}
			}
		}
	}

	// Check if telemetry source changed
	if telemetryConfig, ok := configData["telemetry"].(map[string]any); ok { //nolint:nestif // compact nesting
		if source, ok := telemetryConfig["source"]; ok {
			if sourceStr, ok := source.(string); ok {
				if sourceStr != w.config.GetTelemetrySource() {
					return true
				}
			}
		}
	}

	return false
}

// applyFuelConfig applies fuel management configuration changes.
func (w *WebUI) applyFuelConfig(config map[string]any) []string {
	var errors []string

	if monitoringEnabled, ok := config["enabled"]; ok {
		if enabledBool, ok := monitoringEnabled.(bool); ok {
			w.config.SetPitRadioFuelMonitoringEnabled(enabledBool)
		} else {
			errors = append(errors, "invalid fuel monitoring enabled value")
		}
	}

	if preWarnLaps, ok := config["preWarnNotifyLaps"]; ok {
		if lapsFloat, ok := preWarnLaps.(float64); ok {
			w.config.SetPitRadioFuelPreWarnNotifyLaps(lapsFloat)
		} else {
			errors = append(errors, "invalid pre-warn notify laps value")
		}
	}

	if strategyLaps, ok := config["strategyNotifyLaps"]; ok {
		if lapsFloat, ok := strategyLaps.(float64); ok {
			w.config.SetPitRadioFuelStrategyNotifyLaps(lapsFloat)
		} else {
			errors = append(errors, "invalid strategy notify laps value")
		}
	}

	if safetyMarginLaps, ok := config["rangeSafetyMarginLaps"]; ok {
		if marginFloat, ok := safetyMarginLaps.(float64); ok {
			w.config.SetPitRadioFuelRangeSafetyMarginLaps(marginFloat)
		} else {
			errors = append(errors, "invalid range safety margin laps value")
		}
	}

	if safetyMarginMetres, ok := config["rangeSafetyMarginMetres"]; ok {
		if marginFloat, ok := safetyMarginMetres.(float64); ok {
			w.config.SetPitRadioFuelRangeSafetyMarginMetres(marginFloat)
		} else {
			errors = append(errors, "invalid range safety margin metres value")
		}
	}

	return errors
}

// applyHardwareConfig applies hardware configuration changes.
func (w *WebUI) applyHardwareConfig(config map[string]any) []string {
	var errors []string

	if model, ok := config["model"]; ok {
		if modelStr, ok := model.(string); ok {
			w.config.SetHardwareModel(modelStr)
		} else {
			errors = append(errors, "invalid hardware model value")
		}
	}

	if orientation, ok := config["displayOrientation"]; ok {
		if orientFloat, ok := orientation.(float64); ok {
			w.config.SetDisplayOrientation(int(orientFloat))
		} else {
			errors = append(errors, "invalid display orientation value")
		}
	}

	return errors
}

// applyTelemetryConfig applies telemetry configuration changes.
func (w *WebUI) applyTelemetryConfig(config map[string]any) []string {
	var errors []string

	source, cfgOK := config["source"]
	if !cfgOK {
		return errors
	}

	sourceStr, strOK := source.(string)
	if !strOK {
		errors = append(errors, "invalid telemetry source value")

		return errors
	}

	// If source is a file:// path, validate that the file exists
	if after, cutOK := strings.CutPrefix(sourceStr, "file://"); cutOK {
		filePath := after

		_, err := os.Stat(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				errors = append(errors, "telemetry replay file not found: "+filePath)
			} else {
				errors = append(errors, "cannot access telemetry replay file: "+err.Error())
			}

			return errors
		}
	}

	w.config.SetTelemetrySource(sourceStr)

	return errors
}

// applyDiscordConfig applies Discord configuration changes.
func (w *WebUI) applyDiscordConfig(config map[string]any) []string {
	var errors []string

	if token, ok := config["token"]; ok {
		if tokenStr, ok := token.(string); ok {
			w.config.SetDiscordToken(tokenStr)
		} else {
			errors = append(errors, "invalid discord token value")
		}
	}

	if guildID, ok := config["guildID"]; ok {
		if guildIDStr, ok := guildID.(string); ok {
			w.config.SetDiscordGuildID(guildIDStr)
		} else {
			errors = append(errors, "invalid discord guild ID value")
		}
	}

	if channelID, ok := config["channelID"]; ok {
		if channelIDStr, ok := channelID.(string); ok {
			w.config.SetDiscordChannelID(channelIDStr)
		} else {
			errors = append(errors, "invalid discord channel ID value")
		}
	}

	if voiceChannelID, ok := config["voiceChannelID"]; ok {
		if voiceChannelIDStr, ok := voiceChannelID.(string); ok {
			w.config.SetDiscordVoiceChannelID(voiceChannelIDStr)
		} else {
			errors = append(errors, "invalid discord voice channel ID value")
		}
	}

	return errors
}

// applyNotificationsConfig applies notifications configuration changes.
func (w *WebUI) applyNotificationsConfig(config map[string]any) []string {
	var errors []string

	if raceProgressEnabled, ok := config["enableRaceProgress"]; ok {
		if enabledBool, ok := raceProgressEnabled.(bool); ok {
			w.config.SetPitRadioNotifyRaceProgressEnabled(enabledBool)
		} else {
			errors = append(errors, "invalid race progress enabled value")
		}
	}

	if raceProgressMinLaps, ok := config["raceProgressMinLaps"]; ok {
		if lapsFloat, ok := raceProgressMinLaps.(float64); ok {
			w.config.SetPitRadioNotifyRaceProgressMinLaps(int(lapsFloat))
		} else {
			errors = append(errors, "invalid race progress min laps value")
		}
	}

	if raceProgressIntervalPc, ok := config["raceProgressIntervalPc"]; ok {
		if intervalFloat, ok := raceProgressIntervalPc.(float64); ok {
			w.config.SetPitRadioNotifyRaceProgressIntervalPc(int(intervalFloat))
		} else {
			errors = append(errors, "invalid race progress interval percentage value")
		}
	}

	if raceLapsEnabled, ok := config["enableRaceLaps"]; ok {
		if enabledBool, ok := raceLapsEnabled.(bool); ok {
			w.config.SetPitRadioNotifyRaceLapsEnabled(enabledBool)
		} else {
			errors = append(errors, "invalid race laps enabled value")
		}
	}

	if raceLapsIntervalLaps, ok := config["raceLapsIntervalLaps"]; ok {
		if intervalFloat, ok := raceLapsIntervalLaps.(float64); ok {
			w.config.SetPitRadioNotifyRaceLapsIntervalLaps(int(intervalFloat))
		} else {
			errors = append(errors, "invalid race laps interval laps value")
		}
	}

	if raceLapsCountdownLaps, ok := config["raceLapsCountdownLaps"]; ok {
		if countdownFloat, ok := raceLapsCountdownLaps.(float64); ok {
			w.config.SetPitRadioNotifyRaceLapsCountdownLaps(int(countdownFloat))
		} else {
			errors = append(errors, "invalid race laps countdown laps value")
		}
	}

	if lapTimesEnabled, ok := config["enableLapTimes"]; ok {
		if enabledBool, ok := lapTimesEnabled.(bool); ok {
			w.config.SetPitRadioNotifyLapTimesEnabled(enabledBool)
		} else {
			errors = append(errors, "invalid lap times enabled value")
		}
	}

	if lapTimesMaxDelta, ok := config["lapTimesMaxDeltaSeconds"]; ok {
		if deltaFloat, ok := lapTimesMaxDelta.(float64); ok {
			w.config.SetPitRadioNotifyLapTimesMaxDeltaSeconds(deltaFloat)
		} else {
			errors = append(errors, "invalid lap times max delta seconds value")
		}
	}

	if circuitMatchingEnabled, ok := config["enableCircuitMatching"]; ok {
		if enabledBool, ok := circuitMatchingEnabled.(bool); ok {
			w.config.SetPitRadioNotifyCircuitMatchingEnabled(enabledBool)
		} else {
			errors = append(errors, "invalid circuit matching enabled value")
		}
	}

	return errors
}

// applyPitRadioConfig applies pit radio configuration changes.
func (w *WebUI) applyPitRadioConfig(config map[string]any) []string {
	var errors []string

	if enabled, ok := config["enabled"]; ok {
		if enabledBool, ok := enabled.(bool); ok {
			w.config.SetPitRadioEnabled(enabledBool)
		} else {
			errors = append(errors, "invalid pit radio enabled value")
		}
	}

	if intervalMs, ok := config["messageSendIntervalMs"]; ok {
		if intervalFloat, ok := intervalMs.(float64); ok {
			w.config.SetPitRadioMessageSendIntervalMs(int(intervalFloat))
		} else {
			errors = append(errors, "invalid message send interval value")
		}
	}

	// Handle nested notifications configuration
	if notificationsConfig, ok := config["notifications"]; ok {
		if notificationsMap, ok := notificationsConfig.(map[string]any); ok {
			notificationsErrors := w.applyNotificationsConfig(notificationsMap)
			errors = append(errors, notificationsErrors...)
		} else {
			errors = append(errors, "invalid notifications configuration structure")
		}
	}

	// Handle nested Discord configuration
	if discordConfig, ok := config["discord"]; ok {
		if discordMap, ok := discordConfig.(map[string]any); ok {
			discordErrors := w.applyDiscordConfig(discordMap)
			errors = append(errors, discordErrors...)
		} else {
			errors = append(errors, "invalid discord configuration structure")
		}
	}

	// Handle nested fuel monitoring configuration
	if fuelMonitoringConfig, ok := config["fuelMonitoring"]; ok {
		if fuelMap, ok := fuelMonitoringConfig.(map[string]any); ok {
			fuelErrors := w.applyFuelConfig(fuelMap)
			errors = append(errors, fuelErrors...)
		} else {
			errors = append(errors, "invalid fuel monitoring configuration structure")
		}
	}

	// Handle nested tyre monitoring configuration
	if tyreMonitoringConfig, ok := config["tyreMonitoring"]; ok {
		if tyreMap, ok := tyreMonitoringConfig.(map[string]any); ok {
			tyreErrors := w.applyTyresConfig(tyreMap)
			errors = append(errors, tyreErrors...)
		} else {
			errors = append(errors, "invalid tyre monitoring configuration structure")
		}
	}

	return errors
}

// applyTyresConfig applies tyre management configuration changes.
func (w *WebUI) applyTyresConfig(config map[string]any) []string {
	var errors []string

	if monitoringEnabled, ok := config["enabled"]; ok {
		if enabledBool, ok := monitoringEnabled.(bool); ok {
			w.config.SetPitRadioTyreMonitoringEnabled(enabledBool)
		} else {
			errors = append(errors, "invalid tyre monitoring enabled value")
		}
	}

	if tempOptimal, ok := config["temperatureOptimalCelsius"]; ok {
		if tempFloat, ok := tempOptimal.(float64); ok {
			w.config.SetPitRadioTyreTemperatureOptimalCelsius(float32(tempFloat))
		} else {
			errors = append(errors, "invalid temperature optimal value")
		}
	}

	if tempWindow, ok := config["temperatureOperatingWindow"]; ok {
		if windowFloat, ok := tempWindow.(float64); ok {
			w.config.SetPitRadioTyreTemperatureOperatingWindow(float32(windowFloat))
		} else {
			errors = append(errors, "invalid temperature operating window value")
		}
	}

	if tempMargin, ok := config["temperatureMarginCelsius"]; ok {
		if marginFloat, ok := tempMargin.(float64); ok {
			w.config.SetPitRadioTyreTemperatureMarginCelsius(float32(marginFloat))
		} else {
			errors = append(errors, "invalid temperature margin value")
		}
	}

	return errors
}

// handleConfigStatus returns the configuration status including last update timestamp and restart required flag.
func (w *WebUI) handleConfigStatus(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Content-Type", "application/json")

	if request.Method != http.MethodGet {
		response.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(response).Encode(map[string]string{"error": "Method not allowed"}) //nolint:errchkjson // simple encoding

		return
	}

	status := w.config.Status()

	statusData := map[string]any{
		"lastUpdate":      status.LastUpdate,
		"restartRequired": status.RestartRequired,
	}

	err := json.NewEncoder(response).Encode(statusData)
	if err != nil {
		w.log.Error().Err(err).Msg("failed to encode config status JSON")
		response.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(response).Encode(map[string]string{"error": "Failed to encode status"}) //nolint:errchkjson // simple encoding
	}
}

// handleConfigReset resets the configuration to default values.
func (w *WebUI) handleConfigReset(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Content-Type", "application/json")

	if request.Method != http.MethodPost {
		response.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(response).Encode(map[string]string{"error": "Method not allowed"}) //nolint:errchkjson // simple encoding

		return
	}

	w.log.Info().Msg("configuration reset requested")

	w.config.SetDefault()

	// Save the default configuration to disk
	err := w.config.SaveConfigToFile()
	if err != nil {
		w.log.Error().Err(err).Msg("failed to save default configuration to file")
		response.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(response).Encode(map[string]string{"error": "Failed to save configuration"}) //nolint:errchkjson // simple encoding

		return
	}

	// Mark that a restart is required
	w.config.MarkRestartRequired()

	w.handleGetConfig(response, request)
}

// handleConfigExport handles GET requests to export the full configuration file from disk.
func (w *WebUI) handleConfigExport(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		response.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(response).Encode(map[string]string{"error": "Method not allowed"}) //nolint:errchkjson // simple encoding

		return
	}

	// Get the config file path
	configFilePath := w.config.GetConfigFilePath()

	// Read the config file from disk
	configData, err := os.ReadFile(configFilePath)
	if err != nil {
		w.log.Error().Err(err).Str("file", configFilePath).Msg("failed to read config file")
		response.WriteHeader(http.StatusInternalServerError)
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]string{"error": "Failed to read configuration file"}) //nolint:errchkjson // simple encoding

		return
	}

	// Set headers for file download
	response.Header().Set("Content-Type", "application/json")
	response.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"simtezilo-config-%s.json\"",
		time.Now().Format("20060102-150405")))

	// Write the config file content
	_, err = response.Write(configData)
	if err != nil {
		w.log.Error().Err(err).Msg("failed to write config data to response")
	}
}

// handleConfigImport handles POST requests to import and validate a configuration file.
func (w *WebUI) handleConfigImport(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Content-Type", "application/json")

	if request.Method != http.MethodPost {
		response.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(response).Encode(map[string]string{"error": "Method not allowed"}) //nolint:errchkjson // simple encoding

		return
	}

	// Parse the multipart form with a size limit (10 MB)
	err := request.ParseMultipartForm(10 << 20)
	if err != nil {
		w.log.Error().Err(err).Msg("failed to parse multipart form")
		response.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(response).Encode(map[string]string{"error": "Failed to parse form data"}) //nolint:errchkjson // simple encoding

		return
	}

	// Get the file from the form
	file, header, err := request.FormFile("config")
	if err != nil {
		w.log.Error().Err(err).Msg("failed to get file from form")
		response.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(response).Encode(map[string]string{"error": "No config file provided"}) //nolint:errchkjson // simple encoding

		return
	}
	defer file.Close()

	w.log.Info().Str("filename", header.Filename).Int64("size", header.Size).Msg("config import requested")

	// Read the file content
	fileContent, err := io.ReadAll(file)
	if err != nil {
		w.log.Error().Err(err).Msg("failed to read uploaded file")
		response.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(response).Encode(map[string]string{"error": "Failed to read uploaded file"}) //nolint:errchkjson // simple encoding

		return
	}

	// Validate the JSON content by attempting to unmarshal it
	var testConfig map[string]any

	err = json.Unmarshal(fileContent, &testConfig)
	if err != nil {
		w.log.Error().Err(err).Msg("invalid JSON format")
		response.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(response).Encode(map[string]string{"error": fmt.Sprintf("Invalid JSON format: %v", err)}) //nolint:errchkjson // simple encoding

		return
	}

	// Run comprehensive validation using JSON schema
	validationResult := w.config.ValidateConfig(fileContent)
	if !validationResult.Valid {
		w.log.Warn().Interface("errors", validationResult.Errors).Msg("config validation failed")
		response.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(response).Encode(map[string]any{ //nolint:errchkjson // simple encoding
			"error":            "Configuration validation failed",
			"validationErrors": validationResult.Errors,
		})

		return
	}

	// Create a backup of the current config file before overwriting
	backupPath, err := w.config.BackupConfigFile()
	if err != nil {
		w.log.Warn().Err(err).Msg("failed to backup current config (continuing anyway)")
	} else {
		w.log.Info().Str("backup", backupPath).Msg("created config backup")
	}

	// Write the validated config to the config file
	configFilePath := w.config.GetConfigFilePath()

	err = os.WriteFile(configFilePath, fileContent, 0o600)
	if err != nil {
		w.log.Error().Err(err).Str("file", configFilePath).Msg("failed to write config file")
		response.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(response).Encode(map[string]string{"error": "Failed to write configuration file"}) //nolint:errchkjson // simple encoding

		return
	}

	w.log.Info().Str("file", configFilePath).Msg("config file imported successfully")

	// Mark that a restart is required
	w.config.MarkRestartRequired()

	// Return success response
	_ = json.NewEncoder(response).Encode(map[string]any{ //nolint:errchkjson // simple encoding
		"status":  "success",
		"message": "Configuration imported successfully. Please restart the application for changes to take effect.",
		"backup":  backupPath,
	})
}

// handleRestart handles POST requests to restart the application.
func (w *WebUI) handleRestart(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		response.WriteHeader(http.StatusMethodNotAllowed)

		return
	}

	response.Header().Set("Content-Type", "application/json")
	w.log.Info().Msg("application restart requested")

	// Return success response
	_ = json.NewEncoder(response).Encode(map[string]string{ //nolint:errchkjson // simple encoding
		"status":  "success",
		"message": "Application restarting...",
	})

	// Trigger application shutdown in a goroutine to allow the response to be sent
	go func() {
		time.Sleep(500 * time.Millisecond) // Give time for response to be sent
		w.log.Info().Msg("initiating restart")

		w.shutdownChan <- exitcode.RestartApp
	}()
}

// handleCalibrationSweep handles POST requests to start/stop a calibration frequency sweep.
func (w *WebUI) handleCalibrationSweep(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		response.WriteHeader(http.StatusMethodNotAllowed)

		return
	}

	response.Header().Set("Content-Type", "application/json")

	// Parse request body to determine action
	var reqData struct {
		Action string `json:"action"`
	}

	err := json.NewDecoder(request.Body).Decode(&reqData)
	if err != nil {
		response.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(response).Encode(map[string]string{ //nolint:errchkjson // simple encoding
			"status":  "error",
			"message": "Invalid request body",
		})

		return
	}

	switch reqData.Action {
	case "start":
		w.calibrator.StartSweep() //nolint:contextcheck // sweep context is unrelated to request context
		w.log.Info().Msg("calibration sweep started")
		_ = json.NewEncoder(response).Encode(map[string]any{ //nolint:errchkjson // simple encoding
			"status":    "success",
			"message":   "Sweep started",
			"sweeping":  true,
			"frequency": w.calibrator.GetSweepFrequency(),
		})
	case "stop":
		w.calibrator.StopSweep()
		w.log.Info().Msg("calibration sweep stopped")
		_ = json.NewEncoder(response).Encode(map[string]any{ //nolint:errchkjson // simple encoding
			"status":    "success",
			"message":   "Sweep stopped",
			"sweeping":  false,
			"frequency": w.calibrator.GetFrequency(),
		})
	default:
		response.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(response).Encode(map[string]string{ //nolint:errchkjson // simple encoding
			"status":  "error",
			"message": "Invalid action (must be 'start' or 'stop')",
		})
	}
}

// handleSetupMode handles POST requests to activate setup mode.
func (w *WebUI) handleSetupMode(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		response.WriteHeader(http.StatusMethodNotAllowed)

		return
	}

	response.Header().Set("Content-Type", "application/json")

	w.log.Info().Msg("setup mode requested")

	// Enable setup mode using SetupMode.RunPlatformCommand
	ctx, cancel := context.WithTimeout(request.Context(), 10*time.Second)
	defer cancel()

	_, err := w.setupMode.PlatformAction(ctx, platform.SetupEnable, nil)
	if err != nil {
		w.log.Error().Err(err).Msg("failed to enable setup mode")
		response.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(response).Encode(map[string]string{ //nolint:errchkjson // simple encoding
			"status":  "error",
			"message": "Failed to enable setup mode: " + err.Error(),
		})

		return
	}

	w.log.Info().Msg("setup mode enabled")

	// Return success response
	_ = json.NewEncoder(response).Encode(map[string]string{ //nolint:errchkjson // simple encoding
		"status":  "success",
		"message": "Setup mode activated. Application will shut down.",
	})

	// Trigger application shutdown in a goroutine to allow the response to be sent
	go func() {
		time.Sleep(500 * time.Millisecond) // Give time for response to be sent
		w.log.Info().Msg("initiating shutdown for setup mode")

		w.shutdownChan <- exitcode.SetupMode
	}()
}

// handleSSHEnable handles POST requests to enable SSH.
func (w *WebUI) handleSSHEnable(response http.ResponseWriter, request *http.Request) {
	w.manageSSHEnablement(platform.SSHEnable, response, request)
}

// handleSSHDisable handles POST requests to disable SSH.
func (w *WebUI) handleSSHDisable(response http.ResponseWriter, request *http.Request) {
	w.manageSSHEnablement(platform.SSHDisable, response, request)
}

func (w *WebUI) manageSSHEnablement(action platform.Command, response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		response.WriteHeader(http.StatusMethodNotAllowed)

		return
	}

	var actionStr string

	switch action { //nolint:exhaustive // only interested in enable/disable cases
	case platform.SSHEnable:
		actionStr = "enable"
	case platform.SSHDisable:
		actionStr = "disable"
	default:
		response.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(response).Encode(map[string]string{ //nolint:errchkjson // simple encoding
			"status": "error",
			"error":  "Invalid SSH action",
		})

		return
	}

	response.Header().Set("Content-Type", "application/json")

	w.log.Info().Msgf("SSH %s requested", actionStr)

	ctx, cancel := context.WithTimeout(request.Context(), 10*time.Second)
	defer cancel()

	_, err := w.setupMode.PlatformAction(ctx, action, nil)
	if err != nil {
		w.log.Error().Err(err).Msgf("failed to %s SSH", actionStr)
		response.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(response).Encode(map[string]string{ //nolint:errchkjson // simple encoding
			"status": "error",
			"error":  "Failed to " + actionStr + " SSH: " + err.Error(),
		})

		return
	}

	w.log.Info().Msgf("SSH %sd successfully", actionStr)

	_ = json.NewEncoder(response).Encode(map[string]string{ //nolint:errchkjson // simple encoding
		"status": "success",
	})
}

// handleSSHProvision handles POST requests to provision an SSH public key.
func (w *WebUI) handleSSHProvision(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		response.WriteHeader(http.StatusMethodNotAllowed)

		return
	}

	response.Header().Set("Content-Type", "application/json")

	// Read the SSH public key from the request body
	publicKey, err := io.ReadAll(request.Body)
	if err != nil {
		w.log.Error().Err(err).Msg("failed to read SSH public key from request")
		response.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(response).Encode(map[string]string{ //nolint:errchkjson // simple encoding
			"status": "error",
			"error":  "Failed to read SSH public key",
		})

		return
	}

	w.log.Info().Msg("SSH key provision requested")

	ctx, cancel := context.WithTimeout(request.Context(), 10*time.Second)
	defer cancel()

	_, err = w.setupMode.PlatformAction(ctx, platform.SSHProvision, publicKey)
	if err != nil {
		w.log.Error().Err(err).Msg("failed to provision SSH key")
		response.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(response).Encode(map[string]string{ //nolint:errchkjson // simple encoding
			"status": "error",
			"error":  "Failed to provision SSH key: " + err.Error(),
		})

		return
	}

	w.log.Info().Msg("SSH key provisioned successfully")

	_ = json.NewEncoder(response).Encode(map[string]string{ //nolint:errchkjson // simple encoding
		"status": "success",
	})
}

// handleFactoryReset handles POST requests to perform a factory reset.
func (w *WebUI) handleFactoryReset(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		response.WriteHeader(http.StatusMethodNotAllowed)

		return
	}

	w.log.Warn().Msg("factory reset requested - all settings and network configurations will be deleted")

	// Execute factory reset using SetupMode.RunPlatformCommand
	// Note: No response is sent as the network will disconnect during the reset
	ctx, cancel := context.WithTimeout(request.Context(), 30*time.Second)
	defer cancel()

	_, err := w.setupMode.PlatformAction(ctx, platform.Reset, nil)
	if err != nil {
		w.log.Error().Err(err).Msg("failed to perform factory reset")

		return
	}

	w.log.Info().Msg("factory reset completed successfully")

	// Trigger application shutdown to restart in setup mode
	w.log.Info().Msg("initiating shutdown for setup mode after factory reset")

	w.shutdownChan <- exitcode.SetupMode
}

// handleI18nAPI handles GET requests for i18n translations.
func (w *WebUI) handleI18nAPI(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		response.WriteHeader(http.StatusMethodNotAllowed)

		return
	}

	// Get language from query parameter, default to config language
	lang := request.URL.Query().Get("lang")
	if lang == "" {
		lang = *w.config.GetAppLanguage()
	}

	// Get all translations with the "runmode." prefix
	i18n := w.config.GetI18n()
	if i18n == nil {
		w.log.Error().Msg("i18n instance not available")
		http.Error(response, "i18n not configured", http.StatusInternalServerError)

		return
	}

	translations := i18n.GetStringsWithPrefixForLanguage(lang, "runmode.")

	data, err := json.Marshal(translations)
	if err != nil {
		w.log.Error().Err(err).Msg("failed to marshal translations")
		http.Error(response, "error encoding translations", http.StatusInternalServerError)

		return
	}

	response.Header().Set("Content-Type", "application/json")
	response.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	length, err := response.Write(data)
	if err != nil {
		w.log.Error().Err(err).Int("bytes_written", length).Msg("error writing i18n response")

		return
	}

	w.log.Debug().Str("language", lang).Msg("served i18n translations")
}

// handleLanguagesAPI handles GET requests for available languages.
func (w *WebUI) handleLanguagesAPI(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		response.WriteHeader(http.StatusMethodNotAllowed)

		return
	}

	i18n := w.config.GetI18n()
	if i18n == nil {
		w.log.Error().Msg("i18n instance not available")
		http.Error(response, "i18n not configured", http.StatusInternalServerError)

		return
	}

	languagesMap := i18n.Languages()

	// Build response as array of language objects
	type languageInfo struct {
		Code           string `json:"code"`
		Name           string `json:"name"`
		DefaultCountry string `json:"defaultCountry"`
	}

	languages := make([]languageInfo, 0, len(languagesMap))
	for code, metadata := range languagesMap {
		languages = append(languages, languageInfo{
			Code:           code,
			Name:           metadata.Name,
			DefaultCountry: metadata.DefaultCountry,
		})
	}

	data, err := json.Marshal(languages)
	if err != nil {
		w.log.Error().Err(err).Msg("failed to marshal languages")
		http.Error(response, "error encoding languages", http.StatusInternalServerError)

		return
	}

	response.Header().Set("Content-Type", "application/json")
	response.Header().Set("Cache-Control", "public, max-age=3600")

	length, err := response.Write(data)
	if err != nil {
		w.log.Error().Err(err).Int("bytes_written", length).Msg("error writing languages response")

		return
	}

	w.log.Debug().Msg("served available languages")
}

// handleSystemInfo handles GET requests for system information including build info and hardware platform.
func (w *WebUI) handleSystemInfo(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		response.WriteHeader(http.StatusMethodNotAllowed)

		return
	}

	platform := hardware.Platform().String()

	setupModeAvailable := false
	sshEnabled := false

	if w.setupMode != nil {
		setupModeAvailable = w.setupMode.IsAvailable()

		// Get SSH status if setup mode is available
		if setupModeAvailable {
			ctx, cancel := context.WithTimeout(request.Context(), 5*time.Second)
			defer cancel()

			status := w.setupMode.Status(ctx)
			sshEnabled = status.SSHEnabled
			w.log.Debug().
				Bool("available", status.Available).
				Bool("sshEnabled", status.SSHEnabled).
				Msg("Retrieved SSH status from platform")
		}
	}

	responseData := map[string]any{
		"version":            w.buildVersion,
		"commitHash":         w.buildCommitHash,
		"buildTime":          w.buildTime,
		"buildPlatform":      w.buildPlatform,
		"hardware":           platform,
		"setupModeAvailable": setupModeAvailable,
		"sshEnabled":         sshEnabled,
	}

	response.Header().Set("Content-Type", "application/json")
	response.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	err := json.NewEncoder(response).Encode(responseData)
	if err != nil {
		w.log.Error().Err(err).Msg("failed to encode system info response")
		http.Error(response, "error encoding system info", http.StatusInternalServerError)

		return
	}

	w.log.Debug().Bool("setupModeAvailable", setupModeAvailable).Msg("served system info")
}

// handleLogsAPI returns log entries from the in-memory store.
func (w *WebUI) handleLogsAPI(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		response.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(response).Encode(map[string]string{"error": "Method not allowed"}) //nolint:errchkjson // simple encoding

		return
	}

	if w.logStore == nil {
		w.log.Warn().Msg("log store not initialized")
		response.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(response).Encode(map[string]string{"error": "Log store not available"}) //nolint:errchkjson // simple encoding

		return
	}

	// Parse pagination parameters
	queryParams := request.URL.Query()
	page := 1
	pageSize := 100

	if pageStr := queryParams.Get("page"); pageStr != "" {
		p, err := fmt.Sscanf(pageStr, "%d", &page)
		if err == nil && p == 1 {
			if page < 1 {
				page = 1
			}
		}
	}

	if pageSizeStr := queryParams.Get("pageSize"); pageSizeStr != "" {
		ps, err := fmt.Sscanf(pageSizeStr, "%d", &pageSize)
		if err == nil && ps == 1 {
			if pageSize < 1 {
				pageSize = 100
			} else if pageSize > 1000 {
				pageSize = 1000 // max limit
			}
		}
	}

	// Parse level filters
	var levelFilters map[string]bool

	if levelsParam := queryParams.Get("levels"); levelsParam != "" {
		levels := strings.Split(levelsParam, ",")
		levelFilters = make(map[string]bool)

		for _, level := range levels {
			trimmedLevel := strings.TrimSpace(level)
			if trimmedLevel != "" {
				levelFilters[trimmedLevel] = true
			}
		}
	}

	// Get all logs and filter by level if needed
	allLogs := w.logStore.GetAll()

	var filteredLogs []logstore.LogEntry

	if len(levelFilters) > 0 {
		for _, log := range allLogs {
			if level, ok := log["level"].(string); ok && levelFilters[level] {
				filteredLogs = append(filteredLogs, log)
			}
		}
	} else {
		filteredLogs = allLogs
	}

	// Calculate pagination based on filtered results
	totalCount := len(filteredLogs)

	totalPages := max((totalCount+pageSize-1)/pageSize, 1)

	// Validate page number
	if page > totalPages {
		page = totalPages
	}

	// Calculate offset and slice the filtered logs
	offset := (page - 1) * pageSize

	endIdx := min(offset+pageSize, totalCount)

	var logs []logstore.LogEntry
	if offset < totalCount {
		logs = filteredLogs[offset:endIdx]
	} else {
		logs = []logstore.LogEntry{}
	}

	stats := w.logStore.GetStats()

	// Build response
	responseData := map[string]any{
		"logs":       logs,
		"stats":      stats,
		"count":      len(logs),
		"totalCount": totalCount,
		"page":       page,
		"pageSize":   pageSize,
		"totalPages": totalPages,
	}

	response.Header().Set("Content-Type", "application/json")
	response.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	err := json.NewEncoder(response).Encode(responseData)
	if err != nil {
		w.log.Error().Err(err).Msg("failed to encode logs response")
		http.Error(response, "error encoding logs", http.StatusInternalServerError)

		return
	}

	w.log.Debug().Int("log_count", len(logs)).Int("page", page).Msg("served logs")
}

// handleCacheSize returns the size of the cache directory.
func (w *WebUI) handleCacheSize(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		response.WriteHeader(http.StatusMethodNotAllowed)

		return
	}

	baseDir := w.config.GetAppBaseDir()
	cacheDir := filepath.Join(baseDir, "data", "cache")

	size, count, err := w.getDirSize(cacheDir)
	if err != nil {
		w.log.Error().Err(err).Str("cache_dir", cacheDir).Msg("failed to get cache size")
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{ //nolint:errchkjson // simple encoding
			"success": false,
			"message": "Failed to get cache size",
			"size":    "Error",
		})

		return
	}

	response.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(response).Encode(map[string]any{ //nolint:errchkjson // simple encoding
		"success": true,
		"size":    formatBytes(size),
		"bytes":   size,
		"count":   count,
	})
}

// handleCacheClear clears all files in the cache directory.
func (w *WebUI) handleCacheClear(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		response.WriteHeader(http.StatusMethodNotAllowed)

		return
	}

	baseDir := w.config.GetAppBaseDir()
	cacheDir := filepath.Join(baseDir, "data", "cache")

	w.log.Info().Str("cache_dir", cacheDir).Msg("clearing cache directory")

	// Read directory contents
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		w.log.Error().Err(err).Str("cache_dir", cacheDir).Msg("failed to read cache directory")
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{ //nolint:errchkjson // simple encoding
			"success": false,
			"message": "Failed to read cache directory",
		})

		return
	}

	// Delete all files and directories
	deletedCount := 0
	errorCount := 0

	for _, entry := range entries {
		entryPath := filepath.Join(cacheDir, entry.Name())

		err := os.RemoveAll(entryPath)
		if err != nil {
			w.log.Error().Err(err).Str("path", entryPath).Msg("failed to delete cache entry")

			errorCount++
		} else {
			deletedCount++
		}
	}

	w.log.Info().Int("deleted", deletedCount).Int("errors", errorCount).Msg("cache clear completed")

	response.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(response).Encode(map[string]any{ //nolint:errchkjson // simple encoding
		"success": errorCount == 0,
		"message": fmt.Sprintf("Deleted %d items", deletedCount),
		"deleted": deletedCount,
		"errors":  errorCount,
	})
}

// getDirSize returns the total size of all files in a directory (recursive).
func (w *WebUI) getDirSize(path string) (int64, int, error) {
	var (
		size  int64
		count int
	)

	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			w.log.Warn().Err(err).Str("path", filePath).Msg("Get directory size")

			return nil // Skip files that can't be accessed
		}

		if !info.IsDir() {
			size += info.Size()
			count++
		}

		return nil
	})

	w.log.Debug().Int64("size_bytes", size).Int("count", count).Str("path", path).Msg("Get directory size")

	return size, count, err
}

// formatBytes formats bytes into a human-readable string.
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// handleUpdatesStatus returns the current update status.
func (w *WebUI) handleUpdatesStatus(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		response.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(response).Encode(map[string]string{"error": "Method not allowed"}) //nolint:errchkjson // simple encoding

		return
	}

	responseData := map[string]any{
		"enabled":              w.config.GetAppUpdateAutoCheck(),
		"autoInstall":          w.config.GetAppUpdateAutoInstall(),
		"checkIntervalMinutes": w.config.GetAppUpdateCheckIntervalMinutes(),
		"currentVersion":       w.buildVersion,
		"channel":              w.config.GetAppUpdateChannel(),
		"status":               "disabled",
		"rollbackAvailable":    false,
		"rollbackVersion":      "",
	}

	if w.updater != nil {
		status := w.updater.Status()
		availableUpdate := w.updater.AvailableUpdate()
		lastCheck := w.updater.Checker().LastCheck()
		lastError := w.updater.Checker().LastError()
		currentChannel := w.config.GetAppUpdateChannel()

		responseData["status"] = status.String()
		responseData["rollbackAvailable"] = w.updater.RollbackAvailable()
		responseData["rollbackVersion"] = w.updater.RollbackVersion()

		// Only show as download ready if the update is for the current channel
		downloadReady := status == updater.UpdateStatusReadyToInstall
		if downloadReady && availableUpdate != nil {
			downloadReady = availableUpdate.Channel == currentChannel
		}

		responseData["downloadReady"] = downloadReady

		// Include download progress when downloading
		if status == updater.UpdateStatusDownloading {
			responseData["download_progress"] = w.updater.Checker().DownloadProgress()
		}

		if !lastCheck.IsZero() {
			responseData["lastChecked"] = lastCheck.Format(time.RFC3339)
		}

		if lastError != nil {
			responseData["error"] = lastError.Error()
		}

		if availableUpdate != nil {
			responseData["availableUpdate"] = map[string]any{
				"version":     availableUpdate.AvailableVersion,
				"releaseDate": availableUpdate.ReleaseDate,
				"changelog":   availableUpdate.Changelog,
				"downloadURL": availableUpdate.DownloadURL,
				"size":        availableUpdate.DownloadSize,
				"channel":     availableUpdate.Channel,
			}
		}
	}

	response.Header().Set("Content-Type", "application/json")
	response.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	err := json.NewEncoder(response).Encode(responseData)
	if err != nil {
		w.log.Error().Err(err).Msg("failed to encode updates status response")
		http.Error(response, "error encoding updates status", http.StatusInternalServerError)

		return
	}

	statusStr, ok := responseData["status"].(string)
	if !ok {
		statusStr = "unknown"
	}

	w.log.Debug().Str("status", statusStr).Msg("served updates status")
}

// handleUpdatesCheck triggers an immediate update check.
func (w *WebUI) handleUpdatesCheck(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		response.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(response).Encode(map[string]string{"error": "Method not allowed"}) //nolint:errchkjson // simple encoding

		return
	}

	responseData := map[string]any{
		"updateAvailable": false,
		"currentVersion":  w.buildVersion,
		"downloadReady":   false,
	}

	if w.updater == nil {
		w.log.Debug().Msg("update check requested but updater not available")
	} else {
		updateInfo, err := w.updater.CheckNow() //nolint:contextcheck // CheckNow manages its own timeout context
		if err != nil {
			w.log.Error().Err(err).Msg("failed to check for updates")
			response.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(response).Encode(map[string]string{"error": err.Error()}) //nolint:errchkjson // simple encoding

			return
		}

		status := w.updater.Status()
		responseData["updateAvailable"] = updateInfo != nil
		responseData["downloadReady"] = status == updater.UpdateStatusReadyToInstall

		if updateInfo != nil {
			responseData["availableUpdate"] = map[string]any{
				"version":     updateInfo.AvailableVersion,
				"releaseDate": updateInfo.ReleaseDate,
				"changelog":   updateInfo.Changelog,
				"downloadURL": updateInfo.DownloadURL,
				"size":        updateInfo.DownloadSize,
			}
		}
	}

	response.Header().Set("Content-Type", "application/json")

	err := json.NewEncoder(response).Encode(responseData)
	if err != nil {
		w.log.Error().Err(err).Msg("failed to encode update check response")
		http.Error(response, "error encoding update check response", http.StatusInternalServerError)

		return
	}

	updateAvailable, ok := responseData["updateAvailable"].(bool)
	if !ok {
		updateAvailable = false
	}

	w.log.Info().
		Bool("updateAvailable", updateAvailable).
		Msg("manual update check completed")
}

// handleUpdatesDownload downloads an available update.
func (w *WebUI) handleUpdatesDownload(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		response.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(response).Encode(map[string]string{"error": "Method not allowed"}) //nolint:errchkjson // simple encoding

		return
	}

	if w.updater == nil {
		response.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(response).Encode(map[string]string{"error": "Updater not available"}) //nolint:errchkjson // simple encoding

		return
	}

	availableUpdate := w.updater.AvailableUpdate()
	if availableUpdate == nil {
		response.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(response).Encode(map[string]string{"error": "No update available"}) //nolint:errchkjson // simple encoding

		return
	}

	w.log.Info().
		Str("version", availableUpdate.AvailableVersion).
		Msg("starting update download")

	// Download and prepare the update with progress tracking
	err := w.updater.DownloadAndPrepare(request.Context(), func(progress updater.DownloadProgress) {
		// Store progress in checker so status endpoint can return it
		w.updater.Checker().SetDownloadProgress(progress.Percent)

		w.log.Debug().
			Int64("downloaded", progress.DownloadedBytes).
			Int64("total", progress.TotalBytes).
			Float64("percent", progress.Percent).
			Msg("download progress")
	})
	if err != nil {
		w.log.Error().Err(err).Msg("failed to download update")
		response.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(response).Encode(map[string]string{"error": err.Error()}) //nolint:errchkjson // simple encoding

		return
	}

	response.Header().Set("Content-Type", "application/json")

	err = json.NewEncoder(response).Encode(map[string]any{
		"success": true,
		"version": availableUpdate.AvailableVersion,
		"message": "Update downloaded and prepared for installation",
	})
	if err != nil {
		w.log.Error().Err(err).Msg("failed to encode download response")
		http.Error(response, "error encoding download response", http.StatusInternalServerError)

		return
	}

	w.log.Info().
		Str("version", availableUpdate.AvailableVersion).
		Msg("update downloaded and staged for installation")
}

// UploadMetadata represents metadata embedded in a custom update archive.
type UploadMetadata struct {
	Version     string    `json:"version"`
	ReleaseDate time.Time `json:"releaseDate"`
	Changelog   []string  `json:"changelog"`
	Platform    string    `json:"platform"`
}

// extractMetadataFromArchive attempts to extract manifest.json from an uploaded archive.
func (w *WebUI) extractMetadataFromArchive(file io.ReadSeeker, filename string) (*UploadMetadata, error) {
	// Reset file pointer to beginning
	_, err := file.Seek(0, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("failed to seek file: %w", err)
	}

	// Determine archive type from extension
	if strings.HasSuffix(filename, ".zip") {
		return w.extractMetadataFromZip(file)
	} else if strings.HasSuffix(filename, ".tar.gz") || strings.HasSuffix(filename, ".tgz") {
		return w.extractMetadataFromTarGz(file)
	}

	return nil, fmt.Errorf("unsupported archive format: %s", filename)
}

// extractMetadataFromZip extracts metadata from a ZIP archive.
func (w *WebUI) extractMetadataFromZip(file io.ReadSeeker) (*UploadMetadata, error) {
	// Get file size for zip.NewReader
	size, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to get file size: %w", err)
	}

	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("failed to seek file: %w", err)
	}

	zipReader, err := zip.NewReader(file.(io.ReaderAt), size) //nolint:forcetypeassert // unnecessary
	if err != nil {
		return nil, fmt.Errorf("failed to open zip: %w", err)
	}

	// Look for manifest.json
	for _, f := range zipReader.File {
		if f.Name == "manifest.json" || strings.HasSuffix(f.Name, "/manifest.json") {
			manifest, openErr := f.Open()
			if openErr != nil {
				return nil, fmt.Errorf("failed to open metadata file: %w", openErr)
			}
			defer manifest.Close()

			var metadata UploadMetadata

			decodeErr := json.NewDecoder(manifest).Decode(&metadata)
			if decodeErr != nil {
				return nil, fmt.Errorf("failed to decode metadata: %w", decodeErr)
			}

			return &metadata, nil
		}
	}

	return nil, errors.New("metadata file not found in archive")
}

// extractMetadataFromTarGz extracts metadata from a tar.gz archive.
func (w *WebUI) extractMetadataFromTarGz(file io.ReadSeeker) (*UploadMetadata, error) {
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to open gzip: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)

	// Iterate through tar entries
	for {
		header, tarErr := tarReader.Next()
		if tarErr == io.EOF {
			break
		}

		if tarErr != nil {
			return nil, fmt.Errorf("failed to read tar: %w", tarErr)
		}

		if header.Name == "manifest.json" || strings.HasSuffix(header.Name, "/manifest.json") {
			var metadata UploadMetadata

			decodeErr := json.NewDecoder(tarReader).Decode(&metadata)
			if decodeErr != nil {
				return nil, fmt.Errorf("failed to decode metadata: %w", decodeErr)
			}

			return &metadata, nil
		}
	}

	return nil, errors.New("metadata file not found in archive")
}

// handleUpdatesUpload handles custom update file uploads.
func (w *WebUI) handleUpdatesUpload(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		response.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(response).Encode(map[string]string{"error": "Method not allowed"}) //nolint:errchkjson // simple encoding

		return
	}

	if w.updater == nil {
		response.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(response).Encode(map[string]string{"error": "Updater not available"}) //nolint:errchkjson // simple encoding

		return
	}

	// Parse multipart form with max 500MB upload size
	err := request.ParseMultipartForm(500 << 20)
	if err != nil {
		w.log.Error().Err(err).Msg("failed to parse multipart form")
		response.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(response).Encode(map[string]string{"error": "Failed to parse upload"}) //nolint:errchkjson // simple encoding

		return
	}

	file, header, err := request.FormFile("file")
	if err != nil {
		w.log.Error().Err(err).Msg("failed to get uploaded file")
		response.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(response).Encode(map[string]string{"error": "No file uploaded"}) //nolint:errchkjson // simple encoding

		return
	}
	defer file.Close()

	w.log.Info().
		Str("filename", header.Filename).
		Int64("size", header.Size).
		Msg("Receiving custom update upload")

	// Try to extract metadata from the archive
	var metadata *UploadMetadata

	if seeker, ok := file.(io.ReadSeeker); ok {
		var metaErr error

		metadata, metaErr = w.extractMetadataFromArchive(seeker, header.Filename)
		if metaErr != nil {
			w.log.Warn().Err(metaErr).Msg("failed to extract metadata from archive, using defaults")
		} else {
			w.log.Info().
				Str("version", metadata.Version).
				Str("platform", metadata.Platform).
				Msg("Extracted metadata from archive")
		}
		// Reset file pointer after reading metadata
		_, _ = seeker.Seek(0, io.SeekStart)
	}

	// Get download directory from updater
	downloadDir := filepath.Join(w.config.GetAppBaseDir(), "data", "update", "downloads")

	err = os.MkdirAll(downloadDir, 0o755)
	if err != nil {
		w.log.Error().Err(err).Msg("failed to create download directory")
		response.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(response).Encode(map[string]string{"error": "Failed to create download directory"}) //nolint:errchkjson // simple encoding

		return
	}

	// Clean up old custom uploads before saving the new one
	w.log.Debug().Msg("Cleaning up old uploads before saving new custom upload")
	_ = w.updater.Downloader().CleanupDownloads()

	// Prefix filename with "custom-" to identify custom uploads
	prefixedFilename := "custom-" + header.Filename
	destPath := filepath.Join(downloadDir, prefixedFilename)

	destFile, err := os.Create(destPath)
	if err != nil {
		w.log.Error().Err(err).Msg("failed to create destination file")
		response.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(response).Encode(map[string]string{"error": "Failed to save upload"}) //nolint:errchkjson // simple encoding

		return
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, file)
	if err != nil {
		w.log.Error().Err(err).Msg("failed to write uploaded file")
		response.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(response).Encode(map[string]string{"error": "Failed to write file"}) //nolint:errchkjson // simple encoding

		return
	}

	// Set executable permissions
	err = os.Chmod(destPath, 0o755)
	if err != nil {
		w.log.Warn().Err(err).Msg("failed to set executable permissions")
	}

	// Set updater status to ready to install
	w.updater.Checker().SetStatus(updater.UpdateStatusReadyToInstall)

	// Create UpdateInfo using metadata if available, otherwise use defaults
	var (
		version     string
		changelog   []string
		releaseDate time.Time
	)

	if metadata != nil {
		version = metadata.Version
		changelog = metadata.Changelog
		releaseDate = metadata.ReleaseDate
	} else {
		// Fallback to synthetic info if no metadata
		version = "custom-" + filepath.Base(header.Filename)
		changelog = []string{"Custom uploaded file: " + header.Filename}
		releaseDate = time.Now()
	}

	w.updater.Checker().SetAvailableUpdate(&updater.UpdateInfo{
		CurrentVersion:   w.buildVersion,
		AvailableVersion: version,
		Channel:          "custom",
		Changelog:        changelog,
		DownloadURL:      "",
		DownloadSize:     header.Size,
		SHA256:           "",
		ReleaseDate:      releaseDate,
	})

	// Prepare the update for installation (creates state file for platform script)
	err = w.updater.PrepareInstall(destPath)
	if err != nil {
		w.log.Error().Err(err).Msg("failed to prepare custom update for installation")
		response.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(response).Encode(map[string]string{"error": "Failed to prepare update for installation"}) //nolint:errchkjson // simple encoding

		return
	}

	w.log.Info().
		Str("filename", prefixedFilename).
		Str("version", version).
		Str("path", destPath).
		Msg("Custom update uploaded and ready to install")

	response.Header().Set("Content-Type", "application/json")

	err = json.NewEncoder(response).Encode(map[string]any{
		"success":  true,
		"filename": prefixedFilename,
		"size":     header.Size,
		"version":  version,
		"message":  "Update uploaded and ready to install",
	})
	if err != nil {
		w.log.Error().Err(err).Msg("failed to encode upload response")
		http.Error(response, "error encoding upload response", http.StatusInternalServerError)

		return
	}
}

// handleUpdatesInstall triggers installation of a downloaded update (restarts the service).
func (w *WebUI) handleUpdatesInstall(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		response.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(response).Encode(map[string]string{"error": "Method not allowed"}) //nolint:errchkjson // simple encoding

		return
	}

	if w.updater == nil {
		response.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(response).Encode(map[string]string{"error": "Updater not available"}) //nolint:errchkjson // simple encoding

		return
	}

	status := w.updater.Status()
	if status != updater.UpdateStatusReadyToInstall {
		response.WriteHeader(http.StatusBadRequest)
		//nolint:errchkjson // simple encoding
		_ = json.NewEncoder(response).Encode(map[string]string{
			"error":  "No update ready to install",
			"status": status.String(),
		})

		return
	}

	w.log.Info().Msg("triggering service restart to apply update")

	// Set status to installing before we signal shutdown
	w.updater.SetStatus(updater.UpdateStatusInstalling)

	// Send success response before triggering restart
	response.Header().Set("Content-Type", "application/json")

	err := json.NewEncoder(response).Encode(map[string]any{
		"success": true,
		"message": "Service will restart to apply update",
	})
	if err != nil {
		w.log.Error().Err(err).Msg("failed to encode install response")
	}

	// Signal graceful shutdown - systemd will restart us due to Restart=always,
	// and the ExecStartPre script will apply the staged update.
	go func() {
		// Small delay to ensure response is sent
		time.Sleep(500 * time.Millisecond)

		w.shutdownChan <- exitcode.RestartApp
	}()
}

// handleUpdatesRollback handles POST /api/updates/rollback.
func (w *WebUI) handleUpdatesRollback(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		response.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(response).Encode(map[string]string{"error": "Method not allowed"}) //nolint:errchkjson // simple encoding

		return
	}

	if w.updater == nil {
		response.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(response).Encode(map[string]string{"error": "Updater not available"}) //nolint:errchkjson // simple encoding

		return
	}

	if !w.updater.RollbackAvailable() {
		response.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(response).Encode(map[string]string{"error": "No rollback version available"}) //nolint:errchkjson // simple encoding

		return
	}

	w.log.Info().Msg("triggering rollback to previous version")

	// Perform the rollback
	err := w.updater.Rollback()
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(response).Encode(map[string]string{"error": err.Error()}) //nolint:errchkjson // simple encoding

		return
	}

	// Send success response before triggering restart
	response.Header().Set("Content-Type", "application/json")

	encErr := json.NewEncoder(response).Encode(map[string]any{
		"success": true,
		"message": "Service will restart with previous version",
	})
	if encErr != nil {
		w.log.Error().Err(encErr).Msg("failed to encode rollback response")
	}

	// Signal graceful shutdown - systemd will restart us due to Restart=always
	go func() {
		// Small delay to ensure response is sent
		time.Sleep(500 * time.Millisecond)

		w.shutdownChan <- exitcode.RestartApp
	}()
}
