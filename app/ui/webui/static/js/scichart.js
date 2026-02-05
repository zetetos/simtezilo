// Configuration constants
const CONFIG = {
    FIFO_CAPACITY: 100000,  // Increased to retain more historical data for panning
    RECONNECT_DELAY: 1000,
    MAX_RECONNECT_DELAY: 30000,
    WEBSOCKET_URL: `ws://${location.host}/ws`
};

// Function to update window size display with appropriate units
function updateWindowSizeDisplay(windowSize) {
    const displayElement = document.getElementById('window-size-display');
    if (displayElement && !displayElement.dataset.userEditing) {
        // Window size is in sequence IDs, data arrives at 60Hz (60 packets per second)
        // So windowSize / 60 = seconds
        const windowSizeInSeconds = windowSize / 60;

        let displayText;
        if (windowSizeInSeconds < 1) {
            // Less than 1 second - show milliseconds
            displayText = `${(windowSizeInSeconds * 1000).toFixed(0)}ms`;
        } else if (windowSizeInSeconds < 60) {
            // Less than 60 seconds - show seconds
            displayText = `${windowSizeInSeconds.toFixed(1)}s`;
        } else if (windowSizeInSeconds < 3600) {
            // Less than 60 minutes - show minutes and seconds
            const minutes = Math.floor(windowSizeInSeconds / 60);
            const seconds = Math.floor(windowSizeInSeconds % 60);
            displayText = `${minutes}m ${seconds}s`;
        } else {
            // 60 minutes or more - show hours and minutes
            const hours = Math.floor(windowSizeInSeconds / 3600);
            const minutes = Math.floor((windowSizeInSeconds % 3600) / 60);
            displayText = `${hours}h ${minutes}m`;
        }
        displayElement.value = displayText;
    }
}

// Function to parse user input with units and return window size in sequence IDs
function parseWindowSizeInput(input) {
    const trimmed = input.trim().toLowerCase();

    // Parse patterns like "10s", "2m", "500ms", "1h", "2m 30s"
    const patterns = [
        { regex: /^(\d+(?:\.\d+)?)\s*ms$/, multiplier: 0.001 },
        { regex: /^(\d+(?:\.\d+)?)\s*s$/, multiplier: 1 },
        { regex: /^(\d+(?:\.\d+)?)\s*m$/, multiplier: 60 },
        { regex: /^(\d+(?:\.\d+)?)\s*h$/, multiplier: 3600 },
        { regex: /^(\d+)\s*m\s+(\d+)\s*s$/, isComplex: true },
        { regex: /^(\d+)\s*h\s+(\d+)\s*m$/, isComplex: true }
    ];

    for (const pattern of patterns) {
        const match = trimmed.match(pattern.regex);
        if (match) {
            let seconds;
            if (pattern.isComplex) {
                if (trimmed.includes('h')) {
                    seconds = parseInt(match[1]) * 3600 + parseInt(match[2]) * 60;
                } else {
                    seconds = parseInt(match[1]) * 60 + parseInt(match[2]);
                }
            } else {
                seconds = parseFloat(match[1]) * pattern.multiplier;
            }
            // Convert seconds to sequence IDs (60 per second)
            return seconds * 60;
        }
    }

    return null;
}

// Setup window size input handler
function setupWindowSizeInput(charts, chartFollowModes) {
    const displayElement = document.getElementById('window-size-display');
    if (!displayElement) return;

    displayElement.addEventListener('focus', () => {
        displayElement.dataset.userEditing = 'true';
    });

    displayElement.addEventListener('blur', () => {
        delete displayElement.dataset.userEditing;
    });

    displayElement.addEventListener('keydown', (event) => {
        if (event.key === 'Enter') {
            const windowSize = parseWindowSizeInput(displayElement.value);
            if (windowSize !== null && windowSize > 0) {
                // Apply the new window size to all charts
                charts.forEach((chart, index) => {
                    const xAxis = chart.xAxis;
                    const currentRange = xAxis.visibleRange;
                    const currentCenter = (currentRange.min + currentRange.max) / 2;

                    let newMin = currentCenter - windowSize / 2;
                    let newMax = currentCenter + windowSize / 2;

                    if (newMin < 0) {
                        newMin = 0;
                        newMax = windowSize;
                    }

                    xAxis.visibleRange = new SciChart.NumberRange(newMin, newMax);

                    // Enable follow mode for this chart (like double-click behavior)
                    const chartId = Object.keys(chartFollowModes)[index];
                    if (chartId) {
                        chartFollowModes[chartId] = true;
                    }
                });

                displayElement.blur();
                updateWindowSizeDisplay(windowSize);
            } else {
                // Invalid input - revert to current window size
                const xAxis = charts[0].xAxis;
                const currentRange = xAxis.visibleRange;
                const rangeSize = currentRange.max - currentRange.min;
                updateWindowSizeDisplay(rangeSize);
                displayElement.blur();
            }
        }
    });
}

// Connection state management
let connectionState = {
    isConnected: false,
    reconnectAttempts: 0,
    reconnectDelay: CONFIG.RECONNECT_DELAY
};

// UI status update functions
function updateConnectionStatus(status, message) {
    // Use the unified nav-status-indicator
    if (typeof window.showNavbarStatus === 'function') {
        switch (status) {
            case 'connected':
                window.showNavbarStatus('success');
                break;
            case 'connecting':
                window.showNavbarStatus('saving');
                break;
            case 'disconnected':
            case 'error':
                window.showNavbarStatus('error');
                break;
        }
    }
}

// Manual reconnect function
function forceReconnect() {
    console.log('Manual reconnect requested');
    connectionState.reconnectAttempts = 0;
    connectionState.reconnectDelay = CONFIG.RECONNECT_DELAY;

    if (globalWebSocket) {
        globalWebSocket.close();
    }

    setTimeout(() => {
        globalWebSocket = createWebSocketConnection();
        if (globalHandleWebSocketMessage) {
            globalWebSocket.addEventListener('message', globalHandleWebSocketMessage);
        }
    }, 100);
}

// Chart configurations for different pages
const CHART_CONFIGURATIONS = {
    // Telemetry page - driver-focused charts
    'telemetry': [
        { id: 'rpm-speed', containerId: 'scichart-root-1', enabled: true },
        { id: 'throttle-brake', containerId: 'scichart-root-2', enabled: true },
        { id: 'tyre-temperature', containerId: 'scichart-root-3', enabled: true },
        { id: 'fuel-range', containerId: 'scichart-root-4', enabled: true }
    ],

    // Dev page - developer-focused charts
    'dev': [
        { id: 'translational-acceleration', containerId: 'scichart-root-1', enabled: true },
        { id: 'rotational-acceleration', containerId: 'scichart-root-2', enabled: true },
        { id: 'jerk', containerId: 'scichart-root-3', enabled: true },
        { id: 'snap', containerId: 'scichart-root-4', enabled: true },
        { id: 'synthesizer-output', containerId: 'scichart-root-5', enabled: true },
        { id: 'compute-time', containerId: 'scichart-root-6', enabled: true }
    ],

    // Default/fallback - all charts enabled
    'default': [
        { id: 'rpm-speed', containerId: 'scichart-root-1', enabled: true },
        { id: 'throttle-brake', containerId: 'scichart-root-2', enabled: true },
        { id: 'tyre-temperature', containerId: 'scichart-root-3', enabled: true },
        { id: 'fuel-range', containerId: 'scichart-root-4', enabled: true },
        { id: 'translational-acceleration', containerId: 'scichart-root-5', enabled: true },
        { id: 'rotational-acceleration', containerId: 'scichart-root-6', enabled: true },
        { id: 'jerk', containerId: 'scichart-root-7', enabled: true },
        { id: 'snap', containerId: 'scichart-root-8', enabled: true },
        { id: 'synthesizer-output', containerId: 'scichart-root-9', enabled: true },
        { id: 'compute-time', containerId: 'scichart-root-10', enabled: true }
    ]
};

// Function to get chart configuration from script tag data attribute
function getChartRegistry() {
    // Find the current script tag
    const currentScript = document.currentScript ||
        document.querySelector('script[src*="scichart.js"]') ||
        document.querySelector('script[data-chart-config]');

    if (currentScript) {
        const configType = currentScript.getAttribute('data-chart-config');
        if (configType && CHART_CONFIGURATIONS[configType]) {
            console.log(`Loading chart configuration: ${configType}`);
            return CHART_CONFIGURATIONS[configType];
        }
    }

    console.log('No chart configuration specified, using default');
    return CHART_CONFIGURATIONS.default;
}

// Get the appropriate chart registry based on script tag parameter
const CHART_REGISTRY = getChartRegistry();

// Global variables for WebSocket management
let globalWebSocket = null;
let globalCharts = {};
let globalAllDataSeries = {};
let globalHandleWebSocketMessage = null;

// Generate or retrieve client session ID
function getClientSessionId() {
    let sessionId = sessionStorage.getItem('ws-session-id');
    if (!sessionId) {
        sessionId = 'session-' + Math.random().toString(36).substr(2, 9) + '-' + Date.now();
        sessionStorage.setItem('ws-session-id', sessionId);
    }
    return sessionId;
}

// WebSocket connection management
function createWebSocketConnection() {
    // Close existing connection if any
    if (globalWebSocket) {
        globalWebSocket.onclose = null; // Prevent reconnection logic
        globalWebSocket.onerror = null;
        globalWebSocket.close(1000, 'Creating new connection');
        globalWebSocket = null;
    }

    updateConnectionStatus('connecting');

    // Add session ID to WebSocket URL
    const sessionId = getClientSessionId();
    const wsUrl = `${CONFIG.WEBSOCKET_URL}?session=${encodeURIComponent(sessionId)}`;
    const ws = new WebSocket(wsUrl);

    // Handle connection timeouts
    const connectionTimeout = setTimeout(() => {
        if (ws.readyState === WebSocket.CONNECTING) {
            console.warn('WebSocket connection timeout');
            ws.close();
        }
    }, 5000);

    ws.onopen = (event) => {
        clearTimeout(connectionTimeout);
        console.log('WebSocket connected');
        connectionState.isConnected = true;
        connectionState.reconnectAttempts = 0;
        connectionState.reconnectDelay = CONFIG.RECONNECT_DELAY;
        updateConnectionStatus('connected');

        // Subscribe to telemetry data when connection opens
        ws.send(JSON.stringify({
            type: 'subscribe',
            subscriptions: {
                telemetry: true,
                vehicle: true,
                gameState: true,
                circuit: true,
                race: true
            }
        }));
        console.log('Sent telemetry subscription request');

        // Attach message handler if available
        if (globalHandleWebSocketMessage) {
            ws.addEventListener('message', globalHandleWebSocketMessage);
        }
    };

    ws.onclose = (event) => {
        connectionState.isConnected = false;
        const reason = event.reason || 'Unknown reason';
        console.log(`WebSocket connection closed: ${reason}. Attempting to reconnect...`);

        connectionState.reconnectAttempts++;

        // Exponential backoff with jitter
        const backoffDelay = Math.min(
            connectionState.reconnectDelay * Math.pow(1.5, connectionState.reconnectAttempts - 1),
            CONFIG.MAX_RECONNECT_DELAY
        );
        const jitteredDelay = backoffDelay + (Math.random() * 1000);

        updateConnectionStatus('connecting', 'Reconnecting...');

        setTimeout(() => {
            if (!connectionState.isConnected) {
                globalWebSocket = createWebSocketConnection();
            }
        }, jitteredDelay);
    };

    ws.onerror = (error) => {
        console.error('WebSocket error:', error);
        connectionState.isConnected = false;
        updateConnectionStatus('error', 'Connection failed');
    };

    return ws;
}

async function initSciChart() {
    // We'll initialize WebSocket connection after creating charts
    let ws;

    // Configure SciChart v4 - no need for data file configuration in v4
    // For community license, no action needed. For commercial, use:
    // SciChart.SciChartSurface.setRuntimeLicenseKey("YOUR_LICENSE_KEY");

    // Configure WASM URL if needed (usually auto-detected)
    SciChart.SciChartSurface.configure({
        wasmUrl: `https://cdn.jsdelivr.net/npm/scichart@${SciChart.libraryVersion}/_wasm/scichart2d.wasm`
    });


    // Chart creation helper functions
    const createAxisWithOptions = (wasmContext, options = {}) => {
        return new SciChart.NumericAxis(wasmContext, options);
    };

    const createDataSeries = (wasmContext, dataSeriesName = '') => {
        return new SciChart.XyDataSeries(wasmContext, {
            dataSeriesName: dataSeriesName,
            fifoCapacity: CONFIG.FIFO_CAPACITY,
            isSorted: true,
            containsNaN: false
        });
    };

    const addZoomModifiers = (surface) => {
        // Mouse wheel zoom modifier - zoom Y-axis only (vertical scale)
        surface.chartModifiers.add(new SciChart.MouseWheelZoomModifier({
            xyDirection: SciChart.EXyDirection.YDirection
        }));

        // Add custom horizontal scroll handler for adjusting time window
        surface.domCanvas2D.addEventListener('wheel', (event) => {
            // Check for horizontal scroll (deltaX != 0)
            if (Math.abs(event.deltaX) > Math.abs(event.deltaY)) {
                event.preventDefault();

                const xAxis = surface.xAxes.get(0);
                if (xAxis) {
                    const currentRange = xAxis.visibleRange;
                    const rangeSize = currentRange.max - currentRange.min;

                    // Adjust the time window size based on horizontal scroll
                    // Positive deltaX = scroll right = zoom out (show more time)
                    // Negative deltaX = scroll left = zoom in (show less time)
                    // Scale sensitivity based on current window size for better control
                    // Smaller windows (< 10 seconds) use 5% steps, larger windows use smaller steps
                    const windowSizeInSeconds = rangeSize / 60;
                    let zoomFactor;
                    if (windowSizeInSeconds < 10) {
                        // Small windows: 5% steps
                        zoomFactor = event.deltaX > 0 ? 1.05 : 0.95;
                    } else if (windowSizeInSeconds < 60) {
                        // Medium windows (10s-1min): 3% steps
                        zoomFactor = event.deltaX > 0 ? 1.03 : 0.97;
                    } else {
                        // Large windows (>1 min): 2% steps
                        zoomFactor = event.deltaX > 0 ? 1.02 : 0.98;
                    }
                    const newRangeSize = rangeSize * zoomFactor;

                    // Keep the center point the same
                    const center = (currentRange.min + currentRange.max) / 2;
                    let newMin = center - newRangeSize / 2;
                    let newMax = center + newRangeSize / 2;

                    // Don't allow zooming below 0 on the minimum
                    if (newMin < 0) {
                        newMin = 0;
                        newMax = newRangeSize;
                    }

                    xAxis.visibleRange = new SciChart.NumberRange(newMin, newMax);

                    // Update the window size display
                    updateWindowSizeDisplay(newRangeSize);
                }
            }
        }, { passive: false });

        // Zoom pan modifier - pan by dragging with left mouse button
        surface.chartModifiers.add(new SciChart.ZoomPanModifier({
            xyDirection: SciChart.EXyDirection.XyDirection
        }));

        // Zoom extents modifier - double click to fit the visible data
        surface.chartModifiers.add(new SciChart.ZoomExtentsModifier({ isAnimated: false }));

        // Pinch zoom for touch devices
        surface.chartModifiers.add(new SciChart.PinchZoomModifier());

        // Add rollover modifier to show values at cursor position (Y values only)
        surface.chartModifiers.add(new SciChart.RolloverModifier({
            showTooltip: true,
            showRolloverLine: true,
            rolloverLineStroke: "#51cf66",
            tooltipContainerBackground: "rgba(30, 30, 30, 0.95)",
            tooltipTextStroke: "#ffffff",
            showAxisLabels: false,
            tooltipDataTemplate: (seriesInfo) => {
                // Access the series name from the data series
                const seriesName = seriesInfo.seriesName ||
                    seriesInfo.renderableSeries?.dataSeries?.dataSeriesName ||
                    'Value';
                const yValue = seriesInfo.yValue !== undefined ? seriesInfo.yValue.toFixed(2) : 'N/A';
                return [`${seriesName}: ${yValue}`];
            }
        }));
    };

    const addHorizontalZoomModifiers = (surface) => {
        // No vertical zoom - only horizontal scroll for time window adjustment

        // Add custom horizontal scroll handler for adjusting time window
        surface.domCanvas2D.addEventListener('wheel', (event) => {
            // Check for horizontal scroll (deltaX != 0)
            if (Math.abs(event.deltaX) > Math.abs(event.deltaY)) {
                event.preventDefault();

                const xAxis = surface.xAxes.get(0);
                if (xAxis) {
                    const currentRange = xAxis.visibleRange;
                    const rangeSize = currentRange.max - currentRange.min;

                    // Adjust the time window size based on horizontal scroll
                    const windowSizeInSeconds = rangeSize / 60;
                    let zoomFactor;
                    if (windowSizeInSeconds < 10) {
                        zoomFactor = event.deltaX > 0 ? 1.05 : 0.95;
                    } else if (windowSizeInSeconds < 60) {
                        zoomFactor = event.deltaX > 0 ? 1.03 : 0.97;
                    } else {
                        zoomFactor = event.deltaX > 0 ? 1.02 : 0.98;
                    }
                    const newRangeSize = rangeSize * zoomFactor;

                    const center = (currentRange.min + currentRange.max) / 2;
                    let newMin = center - newRangeSize / 2;
                    let newMax = center + newRangeSize / 2;

                    if (newMin < 0) {
                        newMin = 0;
                        newMax = newRangeSize;
                    }

                    xAxis.visibleRange = new SciChart.NumberRange(newMin, newMax);
                    updateWindowSizeDisplay(newRangeSize);
                }
            }
        }, { passive: false });

        // Zoom pan modifier - horizontal pan only
        surface.chartModifiers.add(new SciChart.ZoomPanModifier({
            xyDirection: SciChart.EXyDirection.XDirection
        }));

        // Zoom extents modifier - double click to fit the visible data
        surface.chartModifiers.add(new SciChart.ZoomExtentsModifier({ isAnimated: false }));

        // Pinch zoom for touch devices - horizontal only
        surface.chartModifiers.add(new SciChart.PinchZoomModifier({
            xyDirection: SciChart.EXyDirection.XDirection
        }));

        // Add rollover modifier to show values at cursor position
        surface.chartModifiers.add(new SciChart.RolloverModifier({
            showTooltip: true,
            showRolloverLine: true,
            rolloverLineStroke: "#51cf66",
            tooltipContainerBackground: "rgba(30, 30, 30, 0.95)",
            tooltipTextStroke: "#ffffff",
            showAxisLabels: false,
            tooltipDataTemplate: (seriesInfo) => {
                const seriesName = seriesInfo.seriesName ||
                    seriesInfo.renderableSeries?.dataSeries?.dataSeriesName ||
                    'Value';
                const yValue = seriesInfo.yValue !== undefined ? seriesInfo.yValue.toFixed(2) : 'N/A';
                return [`${seriesName}: ${yValue}`];
            }
        }));
    };

    const createStandardChart = async (containerId) => {
        const { sciChartSurface, wasmContext } = await SciChart.SciChartSurface.create(containerId);

        const xAxis = createAxisWithOptions(wasmContext, { autoRange: SciChart.EAutoRange.Never });
        const yAxis = createAxisWithOptions(wasmContext, { autoRange: SciChart.EAutoRange.Always });

        sciChartSurface.xAxes.add(xAxis);
        sciChartSurface.yAxes.add(yAxis);

        return { sciChartSurface, wasmContext };
    };

    // Modular chart definitions
    const CHART_DEFINITIONS = {
        'rpm-speed': {
            title: 'RPM / Speed',
            titleKey: 'runmode.telemetry.chart.rpmspeed',
            create: async (containerId) => {
                const { sciChartSurface, wasmContext } = await SciChart.SciChartSurface.create(containerId);

                addZoomModifiers(sciChartSurface);

                const xAxis = createAxisWithOptions(wasmContext, { autoRange: SciChart.EAutoRange.Never });
                const yAxisRPM = createAxisWithOptions(wasmContext, {
                    id: "ID_Y_AXIS_RPM",
                    axisAlignment: SciChart.EAxisAlignment.Left,
                    visibleRange: new SciChart.NumberRange(0, 10000),
                    labelPrecision: 0,
                    labelStyle: { color: "#50C7E0" }
                });
                const yAxisSpeed = createAxisWithOptions(wasmContext, {
                    id: "ID_Y_AXIS_SPEED",
                    axisAlignment: SciChart.EAxisAlignment.Right,
                    visibleRange: new SciChart.NumberRange(0, 350),
                    labelPrecision: 0,
                    labelStyle: { color: "#C750E0" }
                });

                sciChartSurface.xAxes.add(xAxis);
                sciChartSurface.yAxes.add(yAxisRPM, yAxisSpeed);

                const rpmSeries = createDataSeries(wasmContext, "RPM");
                const speedSeries = createDataSeries(wasmContext, "Speed (km/h)");

                sciChartSurface.renderableSeries.add(
                    new SciChart.FastLineRenderableSeries(wasmContext, {
                        dataSeries: rpmSeries,
                        dataSeriesName: "RPM",
                        yAxisId: "ID_Y_AXIS_RPM",
                        strokeThickness: 3,
                        stroke: "#50C7E0"
                    }),
                    new SciChart.FastLineRenderableSeries(wasmContext, {
                        dataSeries: speedSeries,
                        dataSeriesName: "Speed (km/h)",
                        yAxisId: "ID_Y_AXIS_SPEED",
                        strokeThickness: 3,
                        stroke: "#C750E0"
                    })
                );

                return {
                    surface: sciChartSurface,
                    xAxis,
                    dataSeries: {
                        rpm: rpmSeries,
                        speed: speedSeries
                    },
                    dataFields: ['rpm', 'speed']
                };
            }
        },

        'throttle-brake': {
            title: 'Throttle / Brake',
            titleKey: 'runmode.telemetry.chart.throttlebrake',
            create: async (containerId) => {
                const { sciChartSurface, wasmContext } = await SciChart.SciChartSurface.create(containerId);

                addZoomModifiers(sciChartSurface);

                const xAxis = createAxisWithOptions(wasmContext, { autoRange: SciChart.EAutoRange.Never });
                const yAxis = createAxisWithOptions(wasmContext, { visibleRange: new SciChart.NumberRange(0, 110) });

                sciChartSurface.xAxes.add(xAxis);
                sciChartSurface.yAxes.add(yAxis);

                const throttleInputSeries = createDataSeries(wasmContext, "Throttle In");
                const throttleOutputSeries = createDataSeries(wasmContext, "Throttle Out");
                const brakeInputSeries = createDataSeries(wasmContext, "Brake In");
                const brakeOutputSeries = createDataSeries(wasmContext, "Brake Out");

                const seriesConfigs = [
                    { dataSeries: throttleInputSeries, dataSeriesName: "Throttle In", strokeThickness: 3, stroke: "#00F000" },
                    { dataSeries: throttleOutputSeries, dataSeriesName: "Throttle Out", strokeThickness: 2, stroke: "#6EADFF" },
                    { dataSeries: brakeInputSeries, dataSeriesName: "Brake In", strokeThickness: 3, stroke: "#F00000" },
                    { dataSeries: brakeOutputSeries, dataSeriesName: "Brake Out", strokeThickness: 2, stroke: "#FF8A7D" }
                ];

                seriesConfigs.forEach(config => {
                    sciChartSurface.renderableSeries.add(
                        new SciChart.FastLineRenderableSeries(wasmContext, config)
                    );
                });

                return {
                    surface: sciChartSurface,
                    xAxis,
                    dataSeries: {
                        throttleInput: throttleInputSeries,
                        throttleOutput: throttleOutputSeries,
                        brakeInput: brakeInputSeries,
                        brakeOutput: brakeOutputSeries
                    },
                    dataFields: ['throttleInput', 'throttleOutput', 'brakeInput', 'brakeOutput']
                };
            }
        },

        'tyre-temperature': {
            title: 'Tyre Temperature',
            titleKey: 'runmode.telemetry.chart.tyretemperature',
            create: async (containerId) => {
                const { sciChartSurface, wasmContext } = await SciChart.SciChartSurface.create(containerId);

                addZoomModifiers(sciChartSurface);

                const xAxis = createAxisWithOptions(wasmContext, { autoRange: SciChart.EAutoRange.Never });
                const yAxis = createAxisWithOptions(wasmContext, {
                    autoRange: SciChart.EAutoRange.Never,
                    visibleRange: new SciChart.NumberRange(60, 90)
                }); sciChartSurface.xAxes.add(xAxis);
                sciChartSurface.yAxes.add(yAxis);

                const tyreColors = {
                    FL: "#7072fdff", // Front Left - Light Blue
                    FR: "#fa6a6aff", // Front Right - Light Red
                    RL: "#0043fcff", // Rear Left - Dark Blue
                    RR: "#ff0000ff"  // Rear Right - Red
                };

                const tyreSeries = {};
                Object.entries(tyreColors).forEach(([position, color]) => {
                    const series = createDataSeries(wasmContext, `${position}`);
                    tyreSeries[position] = series;

                    sciChartSurface.renderableSeries.add(
                        new SciChart.FastLineRenderableSeries(wasmContext, {
                            dataSeries: series,
                            dataSeriesName: `${position}`,
                            strokeThickness: 3,
                            stroke: color
                        })
                    );
                });

                return {
                    surface: sciChartSurface,
                    xAxis: sciChartSurface.xAxes.get(0),
                    dataSeries: {
                        tyreTempFL: tyreSeries.FL,
                        tyreTempFR: tyreSeries.FR,
                        tyreTempRL: tyreSeries.RL,
                        tyreTempRR: tyreSeries.RR
                    },
                    dataFields: ['tyreTempFL', 'tyreTempFR', 'tyreTempRL', 'tyreTempRR']
                };
            }
        },

        'fuel-range': {
            title: 'Fuel Range/Rate',
            titleKey: 'runmode.telemetry.chart.fuelrange',
            create: async (containerId) => {
                const { sciChartSurface, wasmContext } = await SciChart.SciChartSurface.create(containerId);

                addZoomModifiers(sciChartSurface);

                const xAxis = createAxisWithOptions(wasmContext, { autoRange: SciChart.EAutoRange.Never });
                const yAxisRate = createAxisWithOptions(wasmContext, {
                    id: "ID_Y_AXIS_RATE",
                    axisAlignment: SciChart.EAxisAlignment.Left,
                    autoRange: SciChart.EAutoRange.Always,
                    labelPrecision: 2,
                    labelStyle: { color: "#f9b73dff" }
                });
                const yAxisRange = createAxisWithOptions(wasmContext, {
                    id: "ID_Y_AXIS_RANGE",
                    axisAlignment: SciChart.EAxisAlignment.Right,
                    autoRange: SciChart.EAutoRange.Always,
                    labelPrecision: 1,
                    labelStyle: { color: "#5072e0ff" }
                });

                sciChartSurface.xAxes.add(xAxis);
                sciChartSurface.yAxes.add(yAxisRate, yAxisRange);

                const fuelRateSeries = createDataSeries(wasmContext, "Fuel Usage (%/km)");
                const fuelRangeSeries = createDataSeries(wasmContext, "Range (km)");

                sciChartSurface.renderableSeries.add(
                    new SciChart.FastLineRenderableSeries(wasmContext, {
                        dataSeries: fuelRateSeries,
                        dataSeriesName: "Fuel Usage (%/km)",
                        yAxisId: "ID_Y_AXIS_RATE",
                        strokeThickness: 3,
                        stroke: "#f9b73dff"
                    }),
                    new SciChart.FastLineRenderableSeries(wasmContext, {
                        dataSeries: fuelRangeSeries,
                        dataSeriesName: "Range (km)",
                        yAxisId: "ID_Y_AXIS_RANGE",
                        strokeThickness: 3,
                        stroke: "#5072e0ff"
                    })
                );

                return {
                    surface: sciChartSurface,
                    xAxis,
                    dataSeries: {
                        fuelRate: fuelRateSeries,
                        fuelRange: fuelRangeSeries
                    },
                    dataFields: ['fuelUsagePerKm', 'fuelRangeKm']
                };
            }
        },

        'jerk': {
            title: '6DOF Jerk',
            titleKey: 'runmode.telemetry.chart.jerk',
            create: async (containerId) => {
                const { sciChartSurface, wasmContext } = await createStandardChart(containerId);

                addZoomModifiers(sciChartSurface);

                const translationalJerk = createDataSeries(wasmContext, "Trans. Jerk");
                const translationalJerkCalc = createDataSeries(wasmContext, "Trans. Jerk (Calc)");
                const rotationalJerk = createDataSeries(wasmContext, "Rot. Jerk");

                sciChartSurface.renderableSeries.add(
                    new SciChart.FastLineRenderableSeries(wasmContext, {
                        dataSeries: translationalJerk,
                        dataSeriesName: "Trans. Jerk",
                        strokeThickness: 1,
                        stroke: "#949494ff"
                    }),
                    new SciChart.FastLineRenderableSeries(wasmContext, {
                        dataSeries: translationalJerkCalc,
                        dataSeriesName: "Trans. Jerk (Calc)",
                        strokeThickness: 1,
                        stroke: "#50C7E0"
                    }),
                    new SciChart.FastLineRenderableSeries(wasmContext, {
                        dataSeries: rotationalJerk,
                        dataSeriesName: "Rot. Jerk",
                        strokeThickness: 2,
                        stroke: "#C750E0"
                    })
                );

                return {
                    surface: sciChartSurface,
                    xAxis: sciChartSurface.xAxes.get(0),
                    dataSeries: {
                        translationalJerk: translationalJerk,
                        translationalJerkCalc: translationalJerkCalc,
                        rotationalJerk: rotationalJerk
                    },
                    dataFields: ['SixDOFTranslationalJerk', 'SixDOFTranslationalJerkCalc', 'SixDOFRotationalJerk']
                };
            }
        },

        'snap': {
            title: '6DOF Snap',
            titleKey: 'runmode.telemetry.chart.snap',
            create: async (containerId) => {
                const { sciChartSurface, wasmContext } = await createStandardChart(containerId);

                addZoomModifiers(sciChartSurface);

                const translationalSnapCalc = createDataSeries(wasmContext, "Trans. Snap (Calc)");
                const rotationalSnap = createDataSeries(wasmContext, "Rot. Snap");

                sciChartSurface.renderableSeries.add(
                    new SciChart.FastLineRenderableSeries(wasmContext, {
                        dataSeries: translationalSnapCalc,
                        dataSeriesName: "Trans. Snap (Calc)",
                        strokeThickness: 1,
                        stroke: "#50C7E0ff"
                    }),
                    new SciChart.FastLineRenderableSeries(wasmContext, {
                        dataSeries: rotationalSnap,
                        dataSeriesName: "Rot. Snap",
                        strokeThickness: 2,
                        stroke: "#c850e0ff"
                    })
                );

                return {
                    surface: sciChartSurface,
                    xAxis: sciChartSurface.xAxes.get(0),
                    dataSeries: {
                        translationalSnapCalc: translationalSnapCalc,
                        rotationalSnap: rotationalSnap
                    },
                    dataFields: ['SixDOFTranslationalSnapCalc', 'SixDOFRotationalSnap']
                };
            }
        },

        'translational-acceleration': {
            title: '6DOF Translational Acceleration',
            titleKey: 'runmode.telemetry.chart.translationalaccel',
            create: async (containerId) => {
                const { sciChartSurface, wasmContext } = await createStandardChart(containerId);

                addZoomModifiers(sciChartSurface);

                const translationalAccelerationX = createDataSeries(wasmContext, "Surge");
                const translationalAccelerationY = createDataSeries(wasmContext, "Sway");
                const translationalAccelerationZ = createDataSeries(wasmContext, "Heave");

                sciChartSurface.renderableSeries.add(
                    new SciChart.FastLineRenderableSeries(wasmContext, {
                        dataSeries: translationalAccelerationX,
                        dataSeriesName: "Surge",
                        strokeThickness: 1,
                        stroke: "#e05050ff"
                    }),
                    new SciChart.FastLineRenderableSeries(wasmContext, {
                        dataSeries: translationalAccelerationY,
                        dataSeriesName: "Sway",
                        strokeThickness: 2,
                        stroke: "#50e06aff"
                    }),
                    new SciChart.FastLineRenderableSeries(wasmContext, {
                        dataSeries: translationalAccelerationZ,
                        dataSeriesName: "Heave",
                        strokeThickness: 3,
                        stroke: "#5052e0ff"
                    })
                );

                return {
                    surface: sciChartSurface,
                    xAxis: sciChartSurface.xAxes.get(0),
                    dataSeries: {
                        translationalAccelerationX: translationalAccelerationX,
                        translationalAccelerationY: translationalAccelerationY,
                        translationalAccelerationZ: translationalAccelerationZ
                    },
                    dataFields: ['SixDOFTranslationalAccelX', 'SixDOFTranslationalAccelY', 'SixDOFTranslationalAccelZ']
                };
            }
        },

        'rotational-acceleration': {
            title: '6DOF Rotational Acceleration',
            titleKey: 'runmode.telemetry.chart.rotationalaccel',
            create: async (containerId) => {
                const { sciChartSurface, wasmContext } = await createStandardChart(containerId);

                addZoomModifiers(sciChartSurface);

                const rotationalAccelerationX = createDataSeries(wasmContext, "Pitch");
                const rotationalAccelerationY = createDataSeries(wasmContext, "Yaw");
                const rotationalAccelerationZ = createDataSeries(wasmContext, "Roll");

                sciChartSurface.renderableSeries.add(
                    new SciChart.FastLineRenderableSeries(wasmContext, {
                        dataSeries: rotationalAccelerationX,
                        dataSeriesName: "Pitch",
                        strokeThickness: 1,
                        stroke: "#e05050ff"
                    }),
                    new SciChart.FastLineRenderableSeries(wasmContext, {
                        dataSeries: rotationalAccelerationY,
                        dataSeriesName: "Yaw",
                        strokeThickness: 2,
                        stroke: "#50e06aff"
                    }),
                    new SciChart.FastLineRenderableSeries(wasmContext, {
                        dataSeries: rotationalAccelerationZ,
                        dataSeriesName: "Roll ",
                        strokeThickness: 3,
                        stroke: "#5052e0ff"
                    })
                );

                return {
                    surface: sciChartSurface,
                    xAxis: sciChartSurface.xAxes.get(0),
                    dataSeries: {
                        rotationalAccelerationX: rotationalAccelerationX,
                        rotationalAccelerationY: rotationalAccelerationY,
                        rotationalAccelerationZ: rotationalAccelerationZ
                    },
                    dataFields: ['SixDOFRotationalAccelX', 'SixDOFRotationalAccelY', 'SixDOFRotationalAccelZ']
                };
            }
        },

        'synthesizer-output': {
            title: 'Synthesizer Outputs',
            titleKey: 'runmode.telemetry.chart.synthoutput',
            create: async (containerId) => {
                const { sciChartSurface, wasmContext } = await SciChart.SciChartSurface.create(containerId);

                addHorizontalZoomModifiers(sciChartSurface);

                const xAxis = createAxisWithOptions(wasmContext, { autoRange: SciChart.EAutoRange.Never });
                const yAxisAmplitude = createAxisWithOptions(wasmContext, {
                    id: "ID_Y_AXIS_AMPLITUDE",
                    axisAlignment: SciChart.EAxisAlignment.Left,
                    visibleRange: new SciChart.NumberRange(0, 1),
                    labelPrecision: 3,
                    labelStyle: { color: "#fcdd5fff" }
                });
                const yAxisFrequency = createAxisWithOptions(wasmContext, {
                    id: "ID_Y_AXIS_FREQUENCY",
                    axisAlignment: SciChart.EAxisAlignment.Right,
                    visibleRange: new SciChart.NumberRange(0, 60),
                    labelPrecision: 0,
                    labelStyle: { color: "#38b0faff" }
                });

                sciChartSurface.xAxes.add(xAxis);
                sciChartSurface.yAxes.add(yAxisAmplitude, yAxisFrequency);

                const amplitudeSeries = createDataSeries(wasmContext, "Amplitude");
                const frequencySeries = createDataSeries(wasmContext, "Frequency (Hz)");

                sciChartSurface.renderableSeries.add(
                    new SciChart.FastMountainRenderableSeries(wasmContext, {
                        dataSeries: amplitudeSeries,
                        dataSeriesName: "Amplitude",
                        yAxisId: "ID_Y_AXIS_AMPLITUDE",
                        strokeThickness: 1,
                        stroke: "#fcca5fdf",
                        fillLinearGradient: new SciChart.GradientParams(new SciChart.Point(0, 0), new SciChart.Point(0, 1), [
                            { color: "#fcca5f99", offset: 0 },
                            { color: "#fcca5f1b", offset: 1 },
                        ]),

                    }),
                    new SciChart.FastLineRenderableSeries(wasmContext, {
                        dataSeries: frequencySeries,
                        dataSeriesName: "Frequency (Hz)",
                        yAxisId: "ID_Y_AXIS_FREQUENCY",
                        strokeThickness: 2,
                        stroke: "#38b0faff"
                    })
                );

                return {
                    surface: sciChartSurface,
                    xAxis,
                    dataSeries: {
                        synthAmplitude: amplitudeSeries,
                        synthFrequency: frequencySeries
                    },
                    dataFields: ['synthOutputAmplitude', 'synthOutputFrequency']
                };
            }
        },

        'compute-time': {
            title: 'Compute Time (µs)',
            titleKey: 'runmode.telemetry.chart.computetime',
            create: async (containerId) => {
                const { sciChartSurface, wasmContext } = await createStandardChart(containerId);

                addZoomModifiers(sciChartSurface);

                const series = createDataSeries(wasmContext, "Compute Time (µs)");

                sciChartSurface.renderableSeries.add(
                    new SciChart.FastMountainRenderableSeries(wasmContext, {
                        dataSeries: series,
                        dataSeriesName: "Compute Time (µs)",
                        strokeThickness: 1,
                        stroke: "#42cb52ff",
                        fillLinearGradient: new SciChart.GradientParams(new SciChart.Point(0, 0), new SciChart.Point(0, 1), [
                            { color: "#42cb5299", offset: 0 },
                            { color: "#42cb521e", offset: 1 },
                        ]),
                    })
                );

                return {
                    surface: sciChartSurface,
                    xAxis: sciChartSurface.xAxes.get(0),
                    dataSeries: {
                        computeTime: series
                    },
                    dataFields: ['computeTime']
                };
            }
        }
    };

    // Chart factory system
    const createCharts = async () => {
        const charts = {};
        const allDataSeries = {};

        // Create charts based on registry
        for (const chartConfig of CHART_REGISTRY) {
            if (!chartConfig.enabled) {
                console.log(`Chart ${chartConfig.id} is disabled, skipping...`);
                continue;
            }

            const chartDefinition = CHART_DEFINITIONS[chartConfig.id];
            if (!chartDefinition) {
                console.warn(`Chart definition not found for: ${chartConfig.id}`);
                continue;
            }

            try {
                console.log(`Creating chart: ${chartConfig.id}`);
                const chartInstance = await chartDefinition.create(chartConfig.containerId);

                charts[chartConfig.id] = chartInstance;

                // Merge data series from this chart into the global collection
                Object.assign(allDataSeries, chartInstance.dataSeries);

                console.log(`Successfully created chart: ${chartConfig.id}`);
            } catch (error) {
                console.error(`Failed to create chart ${chartConfig.id}:`, error);
            }
        }

        return { charts, allDataSeries };
    };

    // Create all charts and get references
    const { charts, allDataSeries } = await createCharts();

    // Initialize follow mode for all charts and attach pan detection
    const chartFollowModes = {};
    let isUpdatingRange = false; // Flag to track when we're programmatically updating ranges
    let isDoubleClickTriggered = false;

    // Setup window size input handler (after chartFollowModes is initialized)
    setupWindowSizeInput(Object.values(charts), chartFollowModes);

    Object.entries(charts).forEach(([chartId, chart]) => {
        chartFollowModes[chartId] = true;

        // Disable the ZoomExtents modifier's double-click behavior
        const zoomExtentsModifier = chart.surface.chartModifiers.asArray().find(
            m => m instanceof SciChart.ZoomExtentsModifier
        );
        if (zoomExtentsModifier) {
            zoomExtentsModifier.isEnabled = false;
        }

        // Listen for single click to disable follow mode
        chart.surface.domCanvas2D.addEventListener('click', (event) => {
            // Disable follow mode for all charts on single click
            Object.keys(charts).forEach(otherChartId => {
                chartFollowModes[otherChartId] = false;
            });
        });

        // Listen for double-click to enable follow mode without changing window size
        chart.surface.domCanvas2D.addEventListener('dblclick', (event) => {
            event.preventDefault();
            event.stopPropagation();

            // Set flag that double-click was triggered
            isDoubleClickTriggered = true;

            // Re-enable follow mode for all charts on double-click (preserves window size)
            Object.keys(charts).forEach(otherChartId => {
                chartFollowModes[otherChartId] = true;
            });

            // Reset the flag after a short delay
            setTimeout(() => {
                isDoubleClickTriggered = false;
            }, 100);
        });

        // Listen for when user manually changes the visible range
        chart.xAxis.visibleRangeChanged.subscribe((args) => {
            // Only disable follow mode if this is a user action (not our programmatic update)
            if (!isUpdatingRange && !isDoubleClickTriggered) {
                chartFollowModes[chartId] = false;

                // Synchronize all other charts to the same X-axis range
                isUpdatingRange = true; // Prevent recursion
                Object.entries(charts).forEach(([otherChartId, otherChart]) => {
                    if (otherChartId !== chartId && otherChart && otherChart.xAxis) {
                        chartFollowModes[otherChartId] = false;
                        otherChart.xAxis.visibleRange = new SciChart.NumberRange(
                            args.visibleRange.min,
                            args.visibleRange.max
                        );
                    }
                });
                isUpdatingRange = false;
            }
        });
    });

    // WebSocket message handling
    let lastTimeOfDay = 0;
    let lastSeq = 0;

    const handleWebSocketMessage = (event) => {
        const receivedData = JSON.parse(event.data);

        // Check if this is a unified WebSocket message with type envelope
        let dataFrames;
        if (receivedData.type === 'telemetry') {
            // New unified format: extract data from envelope
            dataFrames = Array.isArray(receivedData.data) ? receivedData.data : [receivedData.data];
        } else if (receivedData.type) {
            // Other message types (vehicle, circuit, race, gameState) - ignore for charts
            return;
        } else {
            // Legacy format: direct data (array or single frame)
            dataFrames = Array.isArray(receivedData) ? receivedData : [receivedData];
        }

        // Process each frame in the batch
        dataFrames.forEach(data => {
            // Clear all data series if sequence number has reset (indicates new session)
            if (data.seq < lastSeq) {
                Object.values(allDataSeries).forEach(series => series.clear());
                // Reset follow modes on new session
                Object.keys(charts).forEach(chartId => {
                    chartFollowModes[chartId] = true;
                });
            }

            const currentTime = data.seq;

            // Create mapping between incoming data and data series
            const dataFieldMappings = {};

            // Build the mapping dynamically based on enabled charts
            Object.values(charts).forEach(chart => {
                chart.dataFields.forEach(fieldName => {
                    const seriesKey = Object.keys(chart.dataSeries).find(key => {
                        // Map data field names to series keys
                        const fieldMappings = {
                            'fuelUsagePerKm': 'fuelRate',
                            'fuelRangeKm': 'fuelRange',
                            'SixDOFTranslationalJerkCalc': 'translationalJerkCalc',
                            'SixDOFTranslationalJerk': 'translationalJerk',
                            'SixDOFTranslationalSnapCalc': 'translationalSnapCalc',
                            'SixDOFTranslationalSnap': 'translationalSnap',
                            'SixDOFTranslationalAccelX': 'translationalAccelerationX',
                            'SixDOFTranslationalAccelY': 'translationalAccelerationY',
                            'SixDOFTranslationalAccelZ': 'translationalAccelerationZ',
                            'SixDOFRotationalJerk': 'rotationalJerk',
                            'SixDOFRotationalSnap': 'rotationalSnap',
                            "SixDOFRotationalAccelX": 'rotationalAccelerationX',
                            "SixDOFRotationalAccelY": 'rotationalAccelerationY',
                            "SixDOFRotationalAccelZ": 'rotationalAccelerationZ',
                            'synthOutputAmplitude': 'synthAmplitude',
                            'synthOutputFrequency': 'synthFrequency'
                        };
                        return fieldMappings[fieldName] === key || fieldName === key;
                    });

                    if (seriesKey && data[fieldName] !== undefined) {
                        dataFieldMappings[seriesKey] = data[fieldName];
                    }
                });
            });

            // Append data to all active series
            Object.entries(dataFieldMappings).forEach(([seriesKey, value]) => {
                if (allDataSeries[seriesKey] && value !== undefined) {
                    allDataSeries[seriesKey].appendRange([currentTime], [value]);
                }
            });

            lastTimeOfDay = data.timeOfDay;
            lastSeq = data.seq;
        });

        // Update visible range for smooth scrolling on all charts (if following live data)
        // Do this once after processing all frames in the batch
        const lastFrame = dataFrames[dataFrames.length - 1];
        const currentTime = lastFrame.seq;

        isUpdatingRange = true; // Set flag before updating ranges
        Object.entries(charts).forEach(([chartId, chart]) => {
            if (chart && chart.xAxis) {
                // Initialize visible range on first data if not set
                if (chart.xAxis.visibleRange.max === 0 || chart.xAxis.visibleRange.min === 0) {
                    const initialWindowSize = 200;
                    chart.xAxis.visibleRange = new SciChart.NumberRange(
                        Math.max(0, currentTime - initialWindowSize),
                        currentTime
                    );
                    // Update window size display on initialization
                    updateWindowSizeDisplay(initialWindowSize);
                    return;
                }

                // Only auto-scroll if in follow mode
                if (chartFollowModes[chartId]) {
                    // Preserve the current window size
                    const currentRange = chart.xAxis.visibleRange;
                    const windowSize = currentRange.max - currentRange.min;

                    // Update to show the latest data while keeping the same window size
                    chart.xAxis.visibleRange = new SciChart.NumberRange(
                        currentTime - windowSize,
                        currentTime
                    );

                    // Update window size display periodically (throttle to first chart only)
                    if (chartId === Object.keys(charts)[0]) {
                        updateWindowSizeDisplay(windowSize);
                    }
                }
            }
        });
        isUpdatingRange = false; // Clear flag after updating ranges
    };

    // Store references globally for reconnection handling
    globalCharts = charts;
    globalAllDataSeries = allDataSeries;
    globalHandleWebSocketMessage = handleWebSocketMessage;

    // Initialize WebSocket connection
    globalWebSocket = createWebSocketConnection();
    globalWebSocket.addEventListener('message', handleWebSocketMessage);

    // Cleanup on page unload to prevent orphaned connections
    const cleanup = () => {
        console.log('Page unloading, cleaning up resources...');

        // Close WebSocket connection immediately and synchronously
        if (globalWebSocket) {
            try {
                // Unsubscribe from telemetry before closing
                if (globalWebSocket.readyState === WebSocket.OPEN) {
                    globalWebSocket.send(JSON.stringify({
                        type: 'subscribe',
                        subscriptions: {
                            telemetry: false
                        }
                    }));
                }

                globalWebSocket.onclose = null; // Prevent reconnection attempts
                globalWebSocket.onerror = null;
                globalWebSocket.onmessage = null;
                // Send close frame synchronously
                globalWebSocket.close(1000, 'Page unload');
                globalWebSocket = null;
            } catch (e) {
                console.error('Error closing WebSocket:', e);
            }
        }

        // Delete all charts to free resources
        Object.values(charts).forEach(chart => {
            if (chart && chart.surface) {
                try {
                    chart.surface.delete();
                } catch (e) {
                    console.error('Error deleting chart:', e);
                }
            }
        });

        // Clear global references
        globalCharts = {};
        globalAllDataSeries = {};
        globalHandleWebSocketMessage = null;
    };

    // Track if page is being unloaded
    let isPageUnloading = false;

    // Use visibility change to detect tab closing/navigation - more reliable than beforeunload
    document.addEventListener('visibilitychange', () => {
        // Only cleanup if page is becoming hidden AND we have an active WebSocket
        if (document.visibilityState === 'hidden' && globalWebSocket) {
            isPageUnloading = true;
            cleanup();
        }
    });

    // Also use pagehide as backup
    window.addEventListener('pagehide', () => {
        if (!isPageUnloading) {
            isPageUnloading = true;
            cleanup();
        }
    });

    // Use unload as last resort (least reliable but covers some edge cases)
    window.addEventListener('unload', () => {
        if (!isPageUnloading) {
            cleanup();
        }
    });
}

// Initialize the application - wait for i18n to be loaded first
let isInitialized = false;

async function initializeWithi18n() {
    // Prevent multiple initializations
    if (isInitialized) {
        console.log('SciChart already initialized, skipping...');
        return;
    }

    // Check if i18n is already loaded
    if (window.i18nLoaded) {
        console.log('i18n already loaded, initializing SciChart...');
        isInitialized = true;
        await initSciChart();
    } else {
        console.log('Waiting for i18n to load before initializing SciChart...');
        window.addEventListener('i18nLoaded', async () => {
            if (!isInitialized) {
                console.log('i18n loaded, now initializing SciChart...');
                isInitialized = true;
                await initSciChart();
            }
        }, { once: true });
    }
}

initializeWithi18n().catch(error => {
    console.error('Failed to initialize application:', error);
});