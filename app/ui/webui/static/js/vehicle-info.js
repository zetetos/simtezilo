// vehicle-info.js - Vehicle information display using SharedWebSocket

(function () {
    'use strict';

    let reconnectAttempts = 0;
    const maxReconnectAttempts = 10;
    const reconnectDelay = 3000; // 3 seconds

    const statusIndicator = document.getElementById('vehicleStatus');
    const vehicleTextElement = document.getElementById('vehicleText');

    let currentManufacturer = '';
    let currentModel = '';
    let currentCarID = 0;

    function updateConnectionStatus(connected) {
        if (statusIndicator) {
            statusIndicator.className = 'status-indicator ' +
                (connected ? 'status-connected' : 'status-disconnected');
        }
    }

    function updateVehicleInfo(data) {
        // Update stored values
        if (data.manufacturer !== undefined) {
            currentManufacturer = data.manufacturer;
        }
        if (data.model !== undefined) {
            currentModel = data.model;
        }
        if (data.carID !== undefined) {
            currentCarID = data.carID;
        }

        // Update display
        if (vehicleTextElement) {
            if (currentManufacturer === '' && currentModel === '') {
                vehicleTextElement.innerHTML = '<span class="vehicle-placeholder" data-i18n="runmode.home.vehicle.waiting">Waiting for data...</span>';
            } else {
                const vehicleFullText = `${currentManufacturer} ${currentModel}`.trim();
                vehicleTextElement.className = 'vehicle-value';

                if (currentCarID && currentCarID > 0) {
                    vehicleTextElement.innerHTML = `<a href="https://www.gran-turismo.com/au/gt7/carlist/id/car${currentCarID}" target="_blank" style="color: #fff; text-decoration: underline;">${vehicleFullText}</a>`;
                } else {
                    vehicleTextElement.textContent = vehicleFullText;
                }
            }
        }

        // Dispatch event for overlay manager
        window.dispatchEvent(new CustomEvent('vehicleDataUpdate', {
            detail: {
                manufacturer: currentManufacturer,
                model: currentModel,
                carID: currentCarID
            }
        }));
    }

    function handleGameState(data) {
        if (data.gamestate !== undefined) {
            window.dispatchEvent(new CustomEvent('gameStateUpdate', {
                detail: {
                    state: data.gamestate
                }
            }));
        }
    }

    // Initialize when SharedWebSocket is available
    function initVehicleInfo() {
        if (window.SharedWebSocket) {
            // Subscribe to vehicle and gameState messages
            window.SharedWebSocket.subscribe('vehicle', updateVehicleInfo);
            window.SharedWebSocket.subscribe('gameState', handleGameState);

            // Listen for connection status changes
            window.SharedWebSocket.addConnectionListener(updateConnectionStatus);

            // Dispatch initial connection status
            window.dispatchEvent(new CustomEvent('vehicleConnectionChange', {
                detail: { connected: window.SharedWebSocket.isConnected }
            }));

            console.log('Vehicle info subscribed to SharedWebSocket');
        } else {
            console.error('SharedWebSocket not available');
        }
    }

    // Initialize when DOM is ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', initVehicleInfo);
    } else {
        initVehicleInfo();
    }
})();
