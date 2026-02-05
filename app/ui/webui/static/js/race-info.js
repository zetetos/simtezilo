// Track connection status
function updateConnectionStatus(connected) {
    const statusElement = document.getElementById('raceStatus');
    if (statusElement) {
        statusElement.className = connected
            ? 'status-indicator status-connected'
            : 'status-indicator status-disconnected';
    }

    if (!connected) {
        clearRaceInfo();
    }

    window.dispatchEvent(new CustomEvent('raceConnectionChange', {
        detail: { connected: connected }
    }));
}

function updateRaceInfo(data) {
    // Update time of day
    if (data.timeofday) {
        document.getElementById('raceTimeOfDay').textContent = data.timeofday;
    }

    // Update lap as "current / total"
    if (data.currentlap && data.racelaps) {
        document.getElementById('raceLap').textContent = `${data.currentlap} / ${data.racelaps}`;
    }

    // Update position as "position / gridsize"
    if (data.position && data.gridsize) {
        document.getElementById('racePosition').textContent = `${data.position} / ${data.gridsize}`;
    }

    // Update lap events table
    if (data.lapevents && Array.isArray(data.lapevents)) {
        updateLapEvents(data.lapevents);
    }

    // Dispatch event for overlay manager
    window.dispatchEvent(new CustomEvent('raceDataUpdate', {
        detail: data
    }));
}

function updateLapEvents(lapEvents) {
    const tbody = document.getElementById('lapEventsBody');

    if (!lapEvents || lapEvents.length === 0) {
        const waitingText = document.querySelector('[data-i18n="runmode.home.race.waiting"]')?.textContent || 'In progress...';
        tbody.innerHTML = `<tr><td colspan="4" class="lap-events-empty">${waitingText}</td></tr>`;
        return;
    }

    // Sort by lap number ascending (oldest first)
    const sortedEvents = [...lapEvents].sort((a, b) => a.lap - b.lap);

    // Find fastest and slowest lap times (raw string comparison won't work, need to parse)
    let fastestTime = Infinity;
    let slowestTime = 0;
    let validLapCount = 0;

    sortedEvents.forEach(event => {
        if (event.laptime && event.laptime !== '-') {
            // Parse lap time MM:SS.mmm to milliseconds
            const parts = event.laptime.split(':');
            if (parts.length === 2) {
                const minutes = parseInt(parts[0]);
                const seconds = parseFloat(parts[1]);
                const timeMs = minutes * 60000 + seconds * 1000;

                if (timeMs < fastestTime) fastestTime = timeMs;
                if (timeMs > slowestTime) slowestTime = timeMs;
                validLapCount++;
            }
        }
    });

    tbody.innerHTML = sortedEvents.map(event => {
        // Delta styling: red for positive (slower), green for negative (faster), white/gray for no difference
        let deltaClass = '';
        if (event.delta) {
            if (event.delta === '0.000' || event.delta === '-') {
                deltaClass = ''; // No difference - standard color
            } else if (event.delta.startsWith('+')) {
                deltaClass = 'lap-delta-positive'; // Slower - red
            } else if (event.delta.startsWith('-')) {
                deltaClass = 'lap-delta-negative'; // Faster - green
            }
        }

        // Determine if this is the fastest or slowest lap
        let lapTimeClass = '';
        if (event.laptime && event.laptime !== '-' && validLapCount > 1) {
            const parts = event.laptime.split(':');
            if (parts.length === 2) {
                const minutes = parseInt(parts[0]);
                const seconds = parseFloat(parts[1]);
                const timeMs = minutes * 60000 + seconds * 1000;

                if (Math.abs(timeMs - fastestTime) < 1) {
                    lapTimeClass = 'fastest-lap';
                } else if (Math.abs(timeMs - slowestTime) < 1) {
                    lapTimeClass = 'slowest-lap';
                }
            }
        }

        return `
            <tr>
                <td>${event.lap || '-'}</td>
                <td class="${lapTimeClass}">${event.laptime || '-'}</td>
                <td class="${deltaClass}">${event.delta || '-'}</td>
                <td>${event.position || '-'}</td>
            </tr>
        `;
    }).join('');
}

function clearRaceInfo() {
    document.getElementById('raceTimeOfDay').textContent = '';
    document.getElementById('raceLap').textContent = '';
    document.getElementById('racePosition').textContent = '';

    // Clear lap events table
    const tbody = document.getElementById('lapEventsBody');
    const waitingText = document.querySelector('[data-i18n="runmode.home.race.waiting"]')?.textContent || 'In progress...';
    tbody.innerHTML = `<tr><td colspan="4" class="lap-events-empty">${waitingText}</td></tr>`;
}

// Initialize when SharedWebSocket is available
function initRaceInfo() {
    if (window.SharedWebSocket) {
        // Subscribe to race messages
        window.SharedWebSocket.subscribe('race', updateRaceInfo);

        // Listen for connection status changes
        window.SharedWebSocket.addConnectionListener(updateConnectionStatus);

        // Set initial connection status
        updateConnectionStatus(window.SharedWebSocket.isConnected);

        console.log('Race info subscribed to SharedWebSocket');
    } else {
        console.error('SharedWebSocket not available');
    }
}

// Initialize when page loads
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initRaceInfo);
} else {
    initRaceInfo();
}
