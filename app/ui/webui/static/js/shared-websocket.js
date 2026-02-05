// shared-websocket.js - Shared WebSocket connection manager for multiple components

(function () {
    'use strict';

    // Singleton WebSocket manager
    const SharedWebSocket = {
        socket: null,
        isConnected: false,
        reconnectAttempts: 0,
        maxReconnectAttempts: 10,
        reconnectDelay: 3000,
        sessionId: null,
        subscribers: new Map(), // Map of message type -> array of handlers
        connectionListeners: [], // Array of connection status listeners

        // Initialize shared WebSocket
        init() {
            if (this.socket) {
                console.log('SharedWebSocket already initialized');
                return;
            }

            this.sessionId = this.getOrCreateSessionId();
            this.connect();
        },

        // Get or create session ID
        getOrCreateSessionId() {
            const key = 'shared_session_id';
            let sessionId = sessionStorage.getItem(key);
            if (!sessionId) {
                sessionId = `shared_${Date.now()}_${Math.random().toString(36).substring(2, 11)}`;
                sessionStorage.setItem(key, sessionId);
            }
            return sessionId;
        },

        // Connect to WebSocket
        connect() {
            const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            const wsUrl = `${protocol}//${window.location.host}/ws?session=${encodeURIComponent(this.sessionId)}`;

            try {
                this.socket = new WebSocket(wsUrl);

                this.socket.onopen = () => {
                    console.log('SharedWebSocket connected');
                    this.isConnected = true;
                    this.reconnectAttempts = 0;

                    // Collect all subscriptions from subscribers
                    const subscriptions = {
                        vehicle: false,
                        gameState: false,
                        circuit: false,
                        race: false,
                        telemetry: false
                    };

                    // Enable subscriptions for registered message types
                    this.subscribers.forEach((handlers, messageType) => {
                        if (handlers.length > 0) {
                            subscriptions[messageType] = true;
                        }
                    });

                    // Send subscription request
                    this.socket.send(JSON.stringify({
                        type: 'subscribe',
                        subscriptions: subscriptions
                    }));

                    console.log('Sent subscription request:', subscriptions);

                    // Notify connection listeners
                    this.notifyConnectionListeners(true);
                };

                this.socket.onmessage = (event) => {
                    try {
                        const message = JSON.parse(event.data);

                        // Route message to appropriate subscribers
                        if (message.type && this.subscribers.has(message.type)) {
                            const handlers = this.subscribers.get(message.type);
                            handlers.forEach(handler => {
                                try {
                                    handler(message.data);
                                } catch (error) {
                                    console.error(`Error in ${message.type} handler:`, error);
                                }
                            });
                        }
                    } catch (error) {
                        console.error('Error parsing WebSocket message:', error);
                    }
                };

                this.socket.onerror = (error) => {
                    console.error('SharedWebSocket error:', error);
                    this.isConnected = false;
                    this.notifyConnectionListeners(false);
                };

                this.socket.onclose = () => {
                    console.log('SharedWebSocket disconnected');
                    this.isConnected = false;
                    this.notifyConnectionListeners(false);

                    // Attempt to reconnect
                    if (this.reconnectAttempts < this.maxReconnectAttempts) {
                        this.reconnectAttempts++;
                        console.log(`Attempting to reconnect SharedWebSocket (${this.reconnectAttempts}/${this.maxReconnectAttempts})...`);
                        setTimeout(() => this.connect(), this.reconnectDelay);
                    } else {
                        console.error('Max reconnection attempts reached for SharedWebSocket');
                    }
                };
            } catch (error) {
                console.error('Failed to create SharedWebSocket:', error);
                this.isConnected = false;
                this.notifyConnectionListeners(false);
            }
        },

        // Subscribe to a specific message type
        subscribe(messageType, handler) {
            if (!this.subscribers.has(messageType)) {
                this.subscribers.set(messageType, []);
            }
            this.subscribers.get(messageType).push(handler);

            // If already connected, update subscriptions
            if (this.isConnected && this.socket) {
                const subscriptions = {};
                subscriptions[messageType] = true;
                this.socket.send(JSON.stringify({
                    type: 'subscribe',
                    subscriptions: subscriptions
                }));
            }

            console.log(`Subscribed to ${messageType} messages`);
        },

        // Unsubscribe from a specific message type
        unsubscribe(messageType, handler) {
            if (this.subscribers.has(messageType)) {
                const handlers = this.subscribers.get(messageType);
                const index = handlers.indexOf(handler);
                if (index > -1) {
                    handlers.splice(index, 1);
                }

                // If no more handlers for this type and connected, unsubscribe
                if (handlers.length === 0 && this.isConnected && this.socket) {
                    const subscriptions = {};
                    subscriptions[messageType] = false;
                    this.socket.send(JSON.stringify({
                        type: 'subscribe',
                        subscriptions: subscriptions
                    }));
                    console.log(`Unsubscribed from ${messageType} messages`);
                }
            }
        },

        // Add connection status listener
        addConnectionListener(listener) {
            this.connectionListeners.push(listener);
            // Immediately notify of current status
            listener(this.isConnected);
        },

        // Remove connection status listener
        removeConnectionListener(listener) {
            const index = this.connectionListeners.indexOf(listener);
            if (index > -1) {
                this.connectionListeners.splice(index, 1);
            }
        },

        // Notify all connection listeners
        notifyConnectionListeners(connected) {
            this.connectionListeners.forEach(listener => {
                try {
                    listener(connected);
                } catch (error) {
                    console.error('Error in connection listener:', error);
                }
            });
        },

        // Close the connection
        close() {
            if (this.socket) {
                this.socket.close();
                this.socket = null;
            }
        }
    };

    // Expose to global scope
    window.SharedWebSocket = SharedWebSocket;

    // Auto-initialize when DOM is ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', () => SharedWebSocket.init());
    } else {
        SharedWebSocket.init();
    }
})();
