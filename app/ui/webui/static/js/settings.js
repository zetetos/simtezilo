// Settings page JavaScript functionality
class ConfigManager {
    constructor() {
        this.config = {};
        this.loadTimeout = null;
        this.saveTimeout = null;
        this.inputSaveTimeouts = new Map(); // Track individual input save timeouts
        this.previousValues = new Map(); // Store previous valid values
        this.configTimestamp = 0; // Backend config timestamp
        this.statusCheckInterval = null; // Interval for checking config status
        this.isInitializing = true; // Flag to prevent saves during initial load
        this.isPopulating = false; // Flag to prevent saves during form population
        this.languageMetadata = {}; // Store language metadata including default countries
        this.init();
    }

    // Format gain value to 2 decimal places with minus prefix (even for 0.00)
    formatGainValue(value) {
        const num = parseFloat(value) || 0;
        const formatted = Math.abs(num).toFixed(2);
        return '-' + formatted;
    }

    // Format decimal value to 2 decimal places (no prefix)
    formatDecimalValue(value) {
        const num = parseFloat(value) || 0;
        return num.toFixed(2);
    }

    async init() {
        await this.loadLanguages();
        await this.loadConfiguration(); // This calls populateForm() internally
        this.setupEventListeners();
        // populateForm() already called in loadConfiguration(), no need to call again
        await this.checkSetupModeAvailability();
        await this.checkHardwarePlatform();
        this.startStatusPolling();
        this.initAdvancedToggle();
        this.initEngineProfiles();
        this.initEqualizer();
        this.isInitializing = false; // Initialization complete, allow saves
    }

    // Initialize advanced settings toggle
    initAdvancedToggle() {
        // Advanced settings toggles are handled in the HTML file
        // (advancedSettingsSynth and advancedSettingsHaptics)
    }

    async checkHardwarePlatform() {
        try {
            const response = await fetch('/api/system/info');
            if (!response.ok) {
                return;
            }
            const data = await response.json();

            // Hide hardware settings card if not on RPI platform
            if (data.hardware !== 'rpi') {
                const hardwareSettingsCard = document.getElementById('hardware-settings-card');
                if (hardwareSettingsCard) {
                    hardwareSettingsCard.style.display = 'none';
                }
            }
        } catch (error) {
            console.error('Failed to check hardware platform:', error);
        }
    }

    // Start polling backend for config status changes
    startStatusPolling() {
        // Check every 2 seconds for config changes
        this.statusCheckInterval = setInterval(() => {
            this.checkConfigStatus();
        }, 2000);
    }

    // Stop status polling
    stopStatusPolling() {
        if (this.statusCheckInterval) {
            clearInterval(this.statusCheckInterval);
            this.statusCheckInterval = null;
        }
    }

    // Check config status from backend
    async checkConfigStatus() {
        try {
            const response = await fetch('/api/config/status');
            if (!response.ok) {
                return;
            }

            const status = await response.json();

            // Update restart indicator based on backend status
            if (status.restartRequired) {
                this.showRestartRequired();
            } else {
                this.hideRestartRequired();
            }

            // Check if backend config is newer than our local copy
            if (status.lastUpdate > this.configTimestamp) {
                await this.loadConfiguration();
            }
        } catch (error) {
            console.error('Failed to check config status:', error);
        }
    }

    // Show status in navbar for a specific input
    showInputStatus(input, type) {
        // Show in navbar if available
        if (typeof window.showNavbarStatus === 'function') {
            window.showNavbarStatus(type);
        }
    }

    // Show restart required indicator
    showRestartRequired() {
        const indicator = document.getElementById('restart-required-indicator');
        if (indicator) {
            indicator.style.display = 'inline-block';
        }

        // Update restart button to green
        const restartBtn = document.getElementById('restart-app');
        if (restartBtn) {
            restartBtn.classList.remove('btn-outline-secondary');
            restartBtn.classList.add('btn-success');
        }
    }

    // Hide restart required indicator
    hideRestartRequired() {
        const indicator = document.getElementById('restart-required-indicator');
        if (indicator) {
            indicator.style.display = 'none';
        }

        // Restore restart button to normal styling
        const restartBtn = document.getElementById('restart-app');
        if (restartBtn) {
            restartBtn.classList.remove('btn-success');
            restartBtn.classList.add('btn-outline-secondary');
        }
    }

    async loadLanguages() {
        try {
            const response = await fetch('/api/languages');
            if (!response.ok) {
                throw new Error('Failed to load languages');
            }
            const languages = await response.json();

            const languageSelect = document.getElementById('app-language');
            if (languageSelect) {
                // Clear existing options
                languageSelect.innerHTML = '';

                // Sort languages alphabetically by name
                languages.sort((a, b) => a.name.localeCompare(b.name));

                // Populate language select with fetched languages and store metadata
                languages.forEach(lang => {
                    const option = document.createElement('option');
                    option.value = lang.code;
                    option.textContent = lang.name;
                    languageSelect.appendChild(option);

                    // Store language metadata including default country
                    this.languageMetadata[lang.code] = {
                        name: lang.name,
                        defaultCountry: lang.defaultCountry
                    };
                });
            }
        } catch (error) {
            console.error('Error loading languages:', error);
            // Languages will stay as hardcoded fallback in HTML
        }
    }

    setupEventListeners() {
        // Export button
        document.getElementById('export-config').addEventListener('click', () => {
            this.exportConfiguration();
        });

        // Import button
        document.getElementById('import-config').addEventListener('click', () => {
            document.getElementById('import-file-input').click();
        });

        // File input change handler
        document.getElementById('import-file-input').addEventListener('change', (event) => {
            const file = event.target.files[0];
            if (file) {
                this.importConfiguration(file);
            }
            // Reset file input so the same file can be selected again
            event.target.value = '';
        });

        // Reset button
        document.getElementById('reset-config').addEventListener('click', () => {
            this.resetConfiguration();
        });

        // Restart app button
        document.getElementById('restart-app').addEventListener('click', () => {
            this.restartApp();
        });

        // Setup mode button
        document.getElementById('setup-mode').addEventListener('click', () => {
            this.enterSetupMode();
        });

        // Factory reset button
        document.getElementById('factory-reset').addEventListener('click', () => {
            this.factoryReset();
        });

        // Developer Tools toggle - update visibility when changed
        const enableDevToolsToggle = document.getElementById('app-enabledevtools');
        if (enableDevToolsToggle) {
            enableDevToolsToggle.addEventListener('change', () => {
                // Update the visibility after the value changes
                setTimeout(() => {
                    this.updateDevToolsVisibility();
                    // Also update navigation to show/hide Developer menu
                    if (typeof initializeNavigation === 'function') {
                        initializeNavigation();
                    }
                }, 100);
            });
        }

        // SSH enable/disable toggle
        const sshEnabledToggle = document.getElementById('ssh-enabled');
        if (sshEnabledToggle) {
            sshEnabledToggle.addEventListener('change', () => {
                this.toggleSSH(sshEnabledToggle.checked);
            });
        }

        // SSH provision button
        const provisionSSHBtn = document.getElementById('provision-ssh-btn');
        if (provisionSSHBtn) {
            provisionSSHBtn.addEventListener('click', () => {
                this.provisionSSH();
            });
        }

        // Calibration frequency buttons
        const calibrationFrequencyUp = document.getElementById('calibration-frequency-up');
        const calibrationFrequencyDown = document.getElementById('calibration-frequency-down');
        const calibrationFrequencyInput = document.getElementById('calibration-frequency');

        if (calibrationFrequencyUp && calibrationFrequencyDown && calibrationFrequencyInput) {
            calibrationFrequencyUp.addEventListener('click', () => {
                const currentValue = parseFloat(calibrationFrequencyInput.value) || 5;
                const step = parseFloat(calibrationFrequencyInput.step) || 1;
                const max = parseFloat(calibrationFrequencyInput.max) || 160;
                calibrationFrequencyInput.value = Math.min(currentValue + step, max);
                calibrationFrequencyInput.dispatchEvent(new Event('change', { bubbles: true }));
            });

            calibrationFrequencyDown.addEventListener('click', () => {
                const currentValue = parseFloat(calibrationFrequencyInput.value) || 5;
                const step = parseFloat(calibrationFrequencyInput.step) || 1;
                const min = parseFloat(calibrationFrequencyInput.min) || 5;
                calibrationFrequencyInput.value = Math.max(currentValue - step, min);
                calibrationFrequencyInput.dispatchEvent(new Event('change', { bubbles: true }));
            });
        }

        // Calibration volume buttons
        const calibrationVolumeUp = document.getElementById('calibration-volume-up');
        const calibrationVolumeDown = document.getElementById('calibration-volume-down');
        const calibrationVolumeInput = document.getElementById('calibration-volume');

        if (calibrationVolumeUp && calibrationVolumeDown && calibrationVolumeInput) {
            calibrationVolumeUp.addEventListener('click', () => {
                const currentValue = parseFloat(calibrationVolumeInput.value) || -30;
                const step = parseFloat(calibrationVolumeInput.step) || 0.25;
                const max = parseFloat(calibrationVolumeInput.max) || 0;
                calibrationVolumeInput.value = Math.min(currentValue + step, max);
                calibrationVolumeInput.dispatchEvent(new Event('change', { bubbles: true }));
            });

            calibrationVolumeDown.addEventListener('click', () => {
                const currentValue = parseFloat(calibrationVolumeInput.value) || -30;
                const step = parseFloat(calibrationVolumeInput.step) || 0.25;
                const min = parseFloat(calibrationVolumeInput.min) || -60;
                calibrationVolumeInput.value = Math.max(currentValue - step, min);
                calibrationVolumeInput.dispatchEvent(new Event('change', { bubbles: true }));
            });
        }

        // Calibration sweep button
        const calibrationSweepBtn = document.getElementById('calibration-sweep-btn');

        if (calibrationSweepBtn && calibrationFrequencyInput) {
            let sweepInterval = null;
            let isSweeping = false;

            // Initialize sweep state from config
            const initSweepState = () => {
                if (this.config.calibration && this.config.calibration.sweeping) {
                    isSweeping = true;
                    calibrationSweepBtn.classList.remove('btn-outline-primary');
                    calibrationSweepBtn.classList.add('btn-primary');
                    calibrationSweepBtn.title = 'Stop Sweep';
                    this.startSweepPolling();
                }
            };

            // Start polling for sweep frequency updates
            this.startSweepPolling = () => {
                if (sweepInterval) return; // Already polling

                sweepInterval = setInterval(async () => {
                    try {
                        const configResponse = await fetch('/api/config');
                        if (configResponse.ok) {
                            const configData = await configResponse.json();
                            if (configData.calibration && configData.calibration.sweeping) {
                                // Update frequency display without triggering save
                                if (calibrationFrequencyInput) {
                                    const currentFreq = Math.round(configData.calibration.frequency);
                                    if (calibrationFrequencyInput.value != currentFreq) {
                                        calibrationFrequencyInput.value = currentFreq;
                                        this.previousValues.set('calibration.frequency', currentFreq);
                                    }
                                }
                            }
                        }
                    } catch (error) {
                        console.error('Failed to poll sweep status:', error);
                    }
                }, 200);
            };

            calibrationSweepBtn.addEventListener('click', async () => {
                if (isSweeping) {
                    // Stop sweep
                    try {
                        const response = await fetch('/api/calibration/sweep', {
                            method: 'POST',
                            headers: {
                                'Content-Type': 'application/json',
                            },
                            body: JSON.stringify({ action: 'stop' })
                        });

                        if (response.ok) {
                            isSweeping = false;
                            calibrationSweepBtn.classList.remove('btn-primary');
                            calibrationSweepBtn.classList.add('btn-outline-primary');
                            calibrationSweepBtn.title = 'Frequency Sweep';

                            // Stop polling frequency
                            if (sweepInterval) {
                                clearInterval(sweepInterval);
                                sweepInterval = null;
                            }
                        }
                    } catch (error) {
                        console.error('Failed to stop sweep:', error);
                    }
                } else {
                    // Check if calibration mode is enabled
                    const calibrationEnabled = document.getElementById('calibration-enabled');
                    if (!calibrationEnabled || !calibrationEnabled.checked) {
                        console.log('Calibration mode must be enabled to start sweep');
                        return;
                    }

                    // Start sweep
                    try {
                        const response = await fetch('/api/calibration/sweep', {
                            method: 'POST',
                            headers: {
                                'Content-Type': 'application/json',
                            },
                            body: JSON.stringify({ action: 'start' })
                        });

                        if (response.ok) {
                            isSweeping = true;
                            calibrationSweepBtn.classList.remove('btn-outline-primary');
                            calibrationSweepBtn.classList.add('btn-primary');
                            calibrationSweepBtn.title = 'Stop Sweep';

                            // Start polling for current frequency
                            this.startSweepPolling();
                        }
                    } catch (error) {
                        console.error('Failed to start sweep:', error);
                    }
                }
            });

            // When calibration mode is disabled, ensure sweep UI and polling stop
            const calibrationEnabled = document.getElementById('calibration-enabled');
            if (calibrationEnabled) {
                calibrationEnabled.addEventListener('change', () => {
                    if (!calibrationEnabled.checked && isSweeping) {
                        isSweeping = false;
                        calibrationSweepBtn.classList.remove('btn-primary');
                        calibrationSweepBtn.classList.add('btn-outline-primary');
                        calibrationSweepBtn.title = 'Frequency Sweep';

                        // Stop polling frequency
                        if (sweepInterval) {
                            clearInterval(sweepInterval);
                            sweepInterval = null;
                        }
                    }
                });
            }

            // Initialize on page load
            setTimeout(initSweepState, 500);
        }

        // Sweep range preset buttons
        const sweepPresetHaptic = document.getElementById('sweep-preset-haptic');
        const sweepPresetFull = document.getElementById('sweep-preset-full');
        const sweepMinInput = document.getElementById('calibration-sweep-min');
        const sweepMaxInput = document.getElementById('calibration-sweep-max');

        if (sweepPresetHaptic && sweepMinInput && sweepMaxInput) {
            sweepPresetHaptic.addEventListener('click', () => {
                // Get haptic pulse min/max frequency from config
                const pulseMinFreq = this.config?.haptics?.pulseMinFrequencyHz || 20;
                const pulseMaxFreq = this.config?.haptics?.pulseMaxFrequencyHz || 80;

                sweepMinInput.value = pulseMinFreq;
                sweepMaxInput.value = pulseMaxFreq;

                // Trigger save for both inputs
                sweepMinInput.dispatchEvent(new Event('change', { bubbles: true }));
                sweepMaxInput.dispatchEvent(new Event('change', { bubbles: true }));

                // Update highlighting
                this.updateSweepPresetHighlights();
            });
        }

        if (sweepPresetFull && sweepMinInput && sweepMaxInput) {
            sweepPresetFull.addEventListener('click', () => {
                sweepMinInput.value = 5;
                sweepMaxInput.value = 160;

                // Trigger save for both inputs
                sweepMinInput.dispatchEvent(new Event('change', { bubbles: true }));
                sweepMaxInput.dispatchEvent(new Event('change', { bubbles: true }));

                // Update highlighting
                this.updateSweepPresetHighlights();
            });
        }

        // Update highlighting when min/max values change
        if (sweepMinInput) {
            sweepMinInput.addEventListener('input', () => this.updateSweepPresetHighlights());
        }
        if (sweepMaxInput) {
            sweepMaxInput.addEventListener('input', () => this.updateSweepPresetHighlights());
        }

        // Mute toggle button click handlers
        const muteToggleButtons = document.querySelectorAll('button[data-mute-checkbox]');
        muteToggleButtons.forEach(button => {
            button.addEventListener('click', () => {
                const checkboxId = button.dataset.muteCheckbox;
                const checkbox = document.getElementById(checkboxId);
                if (checkbox) {
                    // Toggle the checkbox
                    checkbox.checked = !checkbox.checked;
                    // Update icon and button styling
                    this.updateMuteIconForCheckbox(checkbox);
                    // Trigger the change event to save the setting
                    checkbox.dispatchEvent(new Event('change', { bubbles: true }));
                }
            });
        });

        // Note: Mute button icons are initialized by updateAllMuteIcons() in populateForm()
        // which runs before setupEventListeners(), so no need to call initializeMuteButtonIcons() here

        // Pit Radio output change handler to show/hide Discord settings
        const pitRadioOutput = document.getElementById('pitradio-output');
        if (pitRadioOutput) {
            const updateDiscordVisibility = () => {
                const discordSection = document.getElementById('discord-settings-section');
                if (discordSection) {
                    discordSection.style.display = pitRadioOutput.value === 'discord' ? 'block' : 'none';
                }
            };
            // Update on change
            pitRadioOutput.addEventListener('change', updateDiscordVisibility);
            // Update on initial load
            updateDiscordVisibility();
        }

        // Auto-save on blur or Enter key for inputs
        const inputs = document.querySelectorAll('[data-config]');
        inputs.forEach(input => {
            // Store initial value as previous value
            const configPath = input.dataset.config;
            const currentValue = input.type === 'checkbox' ? input.checked : input.value;
            this.previousValues.set(configPath, currentValue);

            // Checkboxes and selects should save immediately on change
            if (input.type === 'checkbox' || input.tagName === 'SELECT' || input.type === 'radio') {
                input.addEventListener('change', () => {
                    // Don't process changes during form population
                    if (this.isPopulating) {
                        return;
                    }

                    // Check if language changed to reload translations
                    if (input.id === 'app-language') {
                        this.handleLanguageChange(input.value);
                    } else {
                        this.saveInputConfiguration(input);
                        // Update transmission disabled state when transmission mode changes
                        if (input.name === 'transmission-mode') {
                            this.updateTransmissionDisabledState();
                        }
                    }
                });
            } else {
                // For text and number inputs, save on blur
                input.addEventListener('blur', () => {
                    if (input.id === 'app-language') {
                        this.handleLanguageChange(input.value);
                    } else {
                        this.saveInputConfiguration(input);
                    }
                    // Format gain inputs after editing
                    if (input.classList.contains('gain-input')) {
                        input.value = this.formatGainValue(input.value);
                    }
                    // Format decimal inputs after editing
                    if (input.classList.contains('decimal-input')) {
                        input.value = this.formatDecimalValue(input.value);
                    }
                });

                // Also save on Enter key press
                input.addEventListener('keydown', (e) => {
                    if (e.key === 'Enter') {
                        e.preventDefault();
                        input.blur(); // Trigger blur to save
                    }
                });

                // For gain inputs, haptics, pitRadio, and calibration inputs, also save immediately on change (spinner buttons)
                if (input.classList.contains('gain-input') ||
                    input.classList.contains('decimal-input') ||
                    configPath.startsWith('haptics.') ||
                    configPath.startsWith('pitRadio.') ||
                    configPath.startsWith('calibration.')) {
                    input.addEventListener('change', () => {
                        this.saveInputConfiguration(input);
                        // Format gain inputs after change
                        if (input.classList.contains('gain-input')) {
                            input.value = this.formatGainValue(input.value);
                        }
                        // Format decimal inputs after change
                        if (input.classList.contains('decimal-input')) {
                            input.value = this.formatDecimalValue(input.value);
                        }
                    });
                }
            }
        });
    }

    // Debounce save for individual inputs
    debounceInputSave(input) {
        const inputId = input.id || input.dataset.config;
        const configPath = input.dataset.config;

        // Clear existing timeout for this input
        clearTimeout(this.inputSaveTimeouts.get(inputId));

        // Set new timeout
        this.inputSaveTimeouts.set(inputId, setTimeout(async () => {
            await this.saveInputConfiguration(input);
        }, 1000));
    }

    // Save configuration for a specific input
    async saveInputConfiguration(input) {
        // Skip saves during initial load or form population
        if (this.isInitializing || this.isPopulating) {
            return;
        }

        const configPath = input.dataset.config;
        let newValue;

        if (input.type === 'radio' && input.dataset.radioValue !== undefined) {
            // Handle radio buttons with data-radio-value
            if (!input.checked) {
                return; // Only save when a radio is checked, not unchecked
            }
            newValue = input.dataset.radioValue === 'true' ? true :
                input.dataset.radioValue === 'false' ? false :
                    input.dataset.radioValue;
        } else if (input.type === 'checkbox') {
            newValue = input.checked;
        } else if (input.type === 'number' || input.dataset.type === 'number') {
            newValue = parseFloat(input.value);
            if (isNaN(newValue)) {
                newValue = 0;
            }

            // Haptics curve values need to be divided by 1000 before sending to backend
            // The UI shows integers (5-955) but backend expects floats (0.005-0.955)
            if (configPath === 'haptics.jerkCurve' ||
                configPath === 'haptics.snapCurve' ||
                configPath === 'haptics.dynamicTransmissionCurve') {
                newValue = newValue / 1000.0;
            }
        } else {
            newValue = input.value;
        }

        // Store the attempted value
        const attemptedValue = newValue;
        const previousValue = this.previousValues.get(configPath);

        // Show saving indicator now that we're actually making the API call
        this.showInputStatus(input, 'saving');

        try {
            // Build form data with just this value
            const formData = {};
            this.setNestedValue(formData, configPath, newValue);

            // Send to server
            try {
                const controller = new AbortController();
                const timeoutId = setTimeout(() => controller.abort(), 5000); // 5 second timeout

                const updateResponse = await fetch('/api/config', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify(formData),
                    signal: controller.signal
                });

                clearTimeout(timeoutId);

                if (!updateResponse.ok) {
                    // Try to parse error details
                    try {
                        const errorData = await updateResponse.json();
                        throw new Error(`HTTP error! status: ${updateResponse.status}, details: ${JSON.stringify(errorData)}`);
                    } catch (parseError) {
                        throw new Error(`HTTP error! status: ${updateResponse.status}`);
                    }
                }

                // Update successful
                this.showInputStatus(input, 'success');
            } catch (fetchError) {
                if (fetchError.name === 'AbortError') {
                    throw new Error('Request timed out');
                } else {
                    throw fetchError;
                }
            }
            this.previousValues.set(configPath, attemptedValue);

            // Update local config
            this.setNestedValue(this.config, configPath, newValue);

            // If gain increment was changed, update step attribute on all gain inputs
            if (configPath === 'synthesizer.gainIncrement') {
                this.updateGainInputSteps();
            }

        } catch (error) {
            console.error(`Failed to save ${configPath}:`, error);
            this.showInputStatus(input, 'error');

            // Keep the attempted value visible while error icon is showing
            // After 3 seconds (when icon disappears), revert to previous valid value
            const inputId = input.id || input.dataset.config;
            this.inputSaveTimeouts.set(inputId + '-revert', setTimeout(() => {
                if (input.type === 'checkbox') {
                    input.checked = previousValue;
                    // Dispatch change event to update any UI elements that depend on this checkbox
                    input.dispatchEvent(new Event('change', { bubbles: true }));
                } else {
                    input.value = previousValue;
                    // Dispatch input event for text/number fields
                    input.dispatchEvent(new Event('input', { bubbles: true }));
                }
            }, 3000));
        }
    }

    async handleLanguageChange(newLanguage) {
        const languageInput = document.getElementById('app-language');
        const accentInput = document.getElementById('app-accent');

        try {
            // Show saving indicator
            this.showInputStatus(languageInput, 'saving');

            // Get the default country for the selected language
            const defaultCountry = this.languageMetadata[newLanguage]?.defaultCountry || '';

            // Build the config update with language and accent changes
            const formData = {};
            this.setNestedValue(formData, 'app.language', newLanguage);
            this.setNestedValue(formData, 'app.accent', defaultCountry);

            // Update in-memory config
            const updateResponse = await fetch('/api/config', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify(formData)
            });

            if (!updateResponse.ok) {
                throw new Error(`HTTP error! status: ${updateResponse.status}`);
            }

            // Verify the config was actually saved by reading it back with retries
            // Give more time for disk I/O to complete
            let verified = false;
            for (let i = 0; i < 8; i++) {
                await new Promise(resolve => setTimeout(resolve, 300));
                const verifyResponse = await fetch('/api/config?t=' + Date.now());
                if (verifyResponse.ok) {
                    const config = await verifyResponse.json();
                    // Verify language changed (accent will be updated but may still show default on first read)
                    if (config.app && config.app.language === newLanguage) {
                        verified = true;
                        break;
                    }
                }
            }

            if (!verified) {
                throw new Error('Could not verify language change in config');
            }

            // Additional wait to ensure config file is fully flushed to disk
            await new Promise(resolve => setTimeout(resolve, 500));

            // Show success briefly before reload
            this.showInputStatus(languageInput, 'success');
            this.previousValues.set('app.language', newLanguage);
            this.previousValues.set('app.accent', defaultCountry);

            // Reload after small delay
            setTimeout(() => {
                window.location.reload(true);
            }, 250);

        } catch (error) {
            console.error('Failed to change language:', error);
            this.showInputStatus(languageInput, 'error');

            // Revert to previous language after icon disappears
            const previousLanguage = this.previousValues.get('app.language');
            setTimeout(() => {
                if (previousLanguage && languageInput) {
                    languageInput.value = previousLanguage;
                }
            }, 3000);
        }
    }

    debounceAutoSave() {
        clearTimeout(this.saveTimeout);

        // Show "saving" indicator immediately
        this.showStatus('Saving...', 'info');

        this.saveTimeout = setTimeout(async () => {
            this.collectFormData();
            await this.saveConfiguration(true); // Silent save
        }, 1000);
    }

    async loadConfiguration() {
        try {
            this.showStatus(t('runmode.settings.status.loading'), 'info');

            const response = await fetch('/api/config');
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }

            this.config = await response.json();

            this.populateForm();
            // Configuration loaded successfully (no need to show indicator)

            // Get and store the backend timestamp
            const statusResponse = await fetch('/api/config/status');
            if (statusResponse.ok) {
                const status = await statusResponse.json();
                this.configTimestamp = status.lastUpdate;

                // Update restart indicator based on backend status
                if (status.restartRequired) {
                    this.showRestartRequired();
                } else {
                    this.hideRestartRequired();
                }
            }

        } catch (error) {
            console.error('Failed to load configuration:', error);
            this.showStatus(t('runmode.settings.error.loadconfigfailed') + error.message, 'error');
        }
    }

    populateForm() {
        this.isPopulating = true; // Set flag to prevent saves during population

        const inputs = document.querySelectorAll('[data-config]');

        inputs.forEach(input => {
            // Skip updating fields that currently have focus (user is editing)
            if (document.activeElement === input) {
                return;
            }

            const configPath = input.dataset.config;
            const value = this.getNestedValue(this.config, configPath);

            if (value !== undefined) {
                if (input.type === 'radio' && input.dataset.radioValue !== undefined) {
                    // Handle radio buttons with data-radio-value
                    const radioValue = input.dataset.radioValue === 'true' ? true :
                        input.dataset.radioValue === 'false' ? false :
                            input.dataset.radioValue;
                    input.checked = (value === radioValue);
                } else if (input.type === 'checkbox') {
                    input.checked = value;
                } else if (input.classList.contains('gain-input')) {
                    // Format gain inputs with 2 decimal places and minus prefix
                    input.value = this.formatGainValue(value);
                } else if (input.classList.contains('decimal-input')) {
                    // Format decimal inputs with 2 decimal places
                    input.value = this.formatDecimalValue(value);
                } else {
                    input.value = value;
                }
                // Update previous values
                this.previousValues.set(configPath, value);
            }
        });

        // After populating all inputs, update all mute icons based on checkbox states
        this.updateAllMuteIcons();

        // Update transmission disabled state based on radio selection
        this.updateTransmissionDisabledState();

        // Update sweep preset button highlighting based on current values
        this.updateSweepPresetHighlights();

        // Update visibility of dev-only elements
        this.updateDevToolsVisibility();

        // Update step attribute on all gain inputs based on gainIncrement config
        this.updateGainInputSteps();

        // Notify UpdateManager to sync its state with the channel dropdown
        if (typeof UpdateManager !== 'undefined' && typeof UpdateManager.syncChannelState === 'function') {
            UpdateManager.syncChannelState();
        }

        this.isPopulating = false; // Clear flag after population complete
    }

    // Update step attribute on all gain inputs to match the configured gainIncrement
    updateGainInputSteps() {
        const gainIncrement = this.getNestedValue(this.config, 'synthesizer.gainIncrement');
        if (gainIncrement && gainIncrement > 0) {
            const gainInputs = document.querySelectorAll('.gain-input');
            gainInputs.forEach(input => {
                input.step = gainIncrement;
                // Also update any cloned touch spinner inputs that mirror this input
                const touchInputs = document.querySelectorAll(`[data-touch-for="${input.id}"]`);
                touchInputs.forEach(touchInput => {
                    touchInput.step = gainIncrement;
                });
            });
        }
    }

    // Update all mute icons based on their corresponding checkbox states
    updateAllMuteIcons() {
        const muteCheckboxes = document.querySelectorAll('input[type="checkbox"][id*="mute"]');
        muteCheckboxes.forEach(checkbox => {
            this.updateMuteIconForCheckbox(checkbox);
        });
    }

    // Update visibility of developer-only UI elements based on enableDevTools setting
    updateDevToolsVisibility() {
        const devToolsEnabled = this.getNestedValue(this.config, 'app.enableDevTools') === true;

        // Hide/show the dev channel option in update channel select
        const updateChannelSelect = document.getElementById('update-channel');
        if (updateChannelSelect) {
            let devChannelOption = updateChannelSelect.querySelector('option[value="dev"]');

            if (devToolsEnabled) {
                // Add the dev option if it doesn't exist
                if (!devChannelOption) {
                    devChannelOption = document.createElement('option');
                    devChannelOption.value = 'dev';
                    devChannelOption.textContent = 'Dev';
                    // Insert before "custom" option
                    const customOption = updateChannelSelect.querySelector('option[value="custom"]');
                    if (customOption) {
                        updateChannelSelect.insertBefore(devChannelOption, customOption);
                    } else {
                        updateChannelSelect.appendChild(devChannelOption);
                    }
                }
            } else {
                // Remove the dev option completely
                if (devChannelOption) {
                    // If dev channel is currently selected, switch to stable first
                    if (updateChannelSelect.value === 'dev') {
                        updateChannelSelect.value = 'stable';
                        // Save the new value
                        this.saveInputConfiguration(updateChannelSelect);
                    }
                    devChannelOption.remove();
                }
            }
        }

        // Hide/show the SSH Access section in System Advanced settings
        const sshAccessSection = document.getElementById('ssh-access-settings');
        if (sshAccessSection) {
            sshAccessSection.style.display = devToolsEnabled ? 'block' : 'none';
        }

        // Hide the entire advanced settings expander for System when dev tools is disabled
        // (since SSH is currently the only content in System advanced settings)
        const advancedToggle = document.querySelector('[data-bs-target="#advancedSettingsSystem"]');
        if (advancedToggle) {
            if (devToolsEnabled) {
                // Show the toggle - restore d-flex class and clear display
                if (!advancedToggle.classList.contains('d-flex')) {
                    advancedToggle.classList.add('d-flex');
                }
                advancedToggle.style.display = '';
            } else {
                // Hide the toggle - remove d-flex and set display none with !important
                advancedToggle.classList.remove('d-flex');
                advancedToggle.style.setProperty('display', 'none', 'important');
            }
        }

        // Store the dev tools state globally so navigation can access it
        window.devToolsEnabled = devToolsEnabled;
    }

    // Update transmission curve and g-force inputs disabled state based on mode
    updateTransmissionDisabledState() {
        const dynamicRadio = document.getElementById('transmission-dynamic-radio');
        const transmissionCurveInput = document.getElementById('haptics-transmissioncurve');
        const gForceMaxInput = document.getElementById('haptics-gforcemax');

        if (dynamicRadio && transmissionCurveInput && gForceMaxInput) {
            const isDynamic = dynamicRadio.checked;
            transmissionCurveInput.disabled = !isDynamic;
            gForceMaxInput.disabled = !isDynamic;
        }
    }

    // Update sweep preset button highlighting based on current min/max values
    updateSweepPresetHighlights() {
        const sweepMinInput = document.getElementById('calibration-sweep-min');
        const sweepMaxInput = document.getElementById('calibration-sweep-max');
        const sweepPresetHaptic = document.getElementById('sweep-preset-haptic');
        const sweepPresetFull = document.getElementById('sweep-preset-full');

        if (!sweepMinInput || !sweepMaxInput) return;

        const currentMin = parseInt(sweepMinInput.value, 10);
        const currentMax = parseInt(sweepMaxInput.value, 10);

        // Haptic preset values
        const hapticMin = this.config?.haptics?.pulseMinFrequencyHz || 20;
        const hapticMax = this.config?.haptics?.pulseMaxFrequencyHz || 80;

        // Full preset values
        const fullMin = 5;
        const fullMax = 160;

        // Update haptic preset button
        if (sweepPresetHaptic) {
            if (currentMin === hapticMin && currentMax === hapticMax) {
                sweepPresetHaptic.classList.remove('btn-outline-secondary');
                sweepPresetHaptic.classList.add('btn-outline-primary');
            } else {
                sweepPresetHaptic.classList.remove('btn-outline-primary');
                sweepPresetHaptic.classList.add('btn-outline-secondary');
            }
        }

        // Update full preset button
        if (sweepPresetFull) {
            if (currentMin === fullMin && currentMax === fullMax) {
                sweepPresetFull.classList.remove('btn-outline-secondary');
                sweepPresetFull.classList.add('btn-outline-primary');
            } else {
                sweepPresetFull.classList.remove('btn-outline-primary');
                sweepPresetFull.classList.add('btn-outline-secondary');
            }
        }
    }

    // Helper to update mute icon and button styling based on checkbox state
    async updateMuteIconForCheckbox(checkbox) {
        // Find the button associated with this checkbox
        const button = document.querySelector(`button[data-mute-checkbox="${checkbox.id}"]`);
        if (button && typeof IconHelper !== 'undefined') {
            if (checkbox.checked) {
                // Muted state - show xmark icon and red outline
                const svg = await IconHelper.loadIcon('fa-volume-xmark');
                if (svg) {
                    button.innerHTML = svg;
                }
                button.classList.remove('btn-outline-primary');
                button.classList.add('btn-outline-danger');
            } else {
                // Unmuted state - show volume icon and secondary outline
                const svg = await IconHelper.loadIcon('fa-volume-high');
                if (svg) {
                    button.innerHTML = svg;
                }
                button.classList.remove('btn-outline-danger');
                button.classList.add('btn-outline-primary');
            }
        }
    }

    // Initialize mute button icons on page load (before config is loaded)
    async initializeMuteButtonIcons() {
        const muteButtons = document.querySelectorAll('button[data-mute-checkbox]');
        for (const button of muteButtons) {
            if (typeof IconHelper !== 'undefined') {
                // Default to volume-high (unmuted) icon and primary outline
                const svg = await IconHelper.loadIcon('fa-volume-high');
                if (svg) {
                    button.innerHTML = svg;
                }
                // Ensure button has the correct unmuted styling
                button.classList.remove('btn-outline-secondary', 'btn-outline-danger');
                button.classList.add('btn-outline-primary');
            }
        }
    }

    collectFormData() {
        const inputs = document.querySelectorAll('[data-config]');
        const formData = {};

        inputs.forEach(input => {
            const configPath = input.dataset.config;
            let value;

            if (input.type === 'radio') {
                // Only process checked radio button
                if (!input.checked) {
                    return;
                }
                // Handle radio buttons with data-radio-value
                if (input.dataset.radioValue !== undefined) {
                    value = input.dataset.radioValue === 'true' ? true :
                        input.dataset.radioValue === 'false' ? false :
                            input.dataset.radioValue;
                } else {
                    value = input.value;
                }
            } else if (input.type === 'checkbox') {
                value = input.checked;
            } else if (input.type === 'number' || input.dataset.type === 'number') {
                value = parseFloat(input.value);
                if (isNaN(value)) {
                    value = 0;
                }
            } else {
                value = input.value;
            }

            this.setNestedValue(formData, configPath, value);
        });

        return formData;
    }

    async saveConfiguration(silent = false) {
        try {
            // Show saving indicator for explicit saves
            if (!silent && typeof window.showNavbarStatus === 'function') {
                window.showNavbarStatus('saving');
            }

            const formData = this.collectFormData();

            // First update the in-memory configuration
            const updateResponse = await fetch('/api/config', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify(formData)
            });

            if (!updateResponse.ok) {
                throw new Error(`HTTP error! status: ${updateResponse.status}`);
            }

            // If not silent, also save to file with backup
            if (!silent) {
                const saveResponse = await fetch('/api/config/save', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    }
                });

                if (!saveResponse.ok) {
                    throw new Error(`Failed to save configuration to file! status: ${saveResponse.status}`);
                }

                const saveResult = await saveResponse.json();
            }

            // Show success indicator
            if (typeof window.showNavbarStatus === 'function') {
                window.showNavbarStatus('success');
            }

            // Update local config with server response
            this.config = { ...this.config, ...formData };

        } catch (error) {
            console.error('Failed to save configuration:', error);
            // Show error indicator
            if (typeof window.showNavbarStatus === 'function') {
                window.showNavbarStatus('error');
            }
        }
    }

    async resetConfiguration() {
        if (!confirm(t('runmode.settings.confirm.reset'))) {
            return;
        }

        const resetBtn = document.getElementById('reset-config');

        try {
            resetBtn.disabled = true;
            if (typeof window.showNavbarStatus === 'function') {
                window.showNavbarStatus('saving');
            }

            const response = await fetch('/api/config/reset', {
                method: 'POST'
            });

            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }

            this.config = await response.json();
            this.populateForm();

            // Show success indicator for reset operation
            if (typeof window.showNavbarStatus === 'function') {
                window.showNavbarStatus('success');
            }

            resetBtn.disabled = false;
        } catch (error) {
            console.error('Failed to reset configuration:', error);
            // Show error indicator for reset operation
            if (typeof window.showNavbarStatus === 'function') {
                window.showNavbarStatus('error');
            }
            resetBtn.disabled = false;
        }
    }

    exportConfiguration() {
        try {
            // Fetch the full config file from the server instead of using the cached JSON payload
            const link = document.createElement('a');
            link.href = '/api/config/export';
            link.download = ''; // Let server decide filename

            // Trigger download
            document.body.appendChild(link);
            link.click();
            document.body.removeChild(link);
        } catch (error) {
            console.error('Failed to export configuration:', error);
            alert(t('runmode.settings.error.exportfailed') + error.message);
        }
    }

    async importConfiguration(file) {
        const importBtn = document.getElementById('import-config');

        try {
            // Validate file extension
            if (!file.name.endsWith('.conf') && !file.name.endsWith('.json')) {
                throw new Error('Invalid file type. Please select a .conf or .json file.');
            }

            // Create FormData to send file
            const formData = new FormData();
            formData.append('config', file);

            // Show loading state
            importBtn.disabled = true;
            if (typeof window.showNavbarStatus === 'function') {
                window.showNavbarStatus('saving');
            }

            // Send to server
            const response = await fetch('/api/config/import', {
                method: 'POST',
                body: formData
            });

            const result = await response.json();

            if (!response.ok) {
                // Handle validation errors specially
                if (result.validationErrors && result.validationErrors.length > 0) {
                    let errorMessage = 'Configuration validation failed:\n\n';
                    result.validationErrors.forEach(err => {
                        errorMessage += `• ${err.field}: ${err.message}\n`;
                    });
                    throw new Error(errorMessage);
                }
                throw new Error(result.error || 'Failed to import configuration');
            }

            // Show success indicator
            if (typeof window.showNavbarStatus === 'function') {
                window.showNavbarStatus('success');
            }

            // Show restart required indicator
            this.showRestartRequired();

            // Show success message
            alert(result.message + (result.backup ? '\n\nBackup created at: ' + result.backup : ''));

            // Reload the configuration
            await this.loadConfiguration();

            // Re-enable button
            importBtn.disabled = false;
        } catch (error) {
            console.error('Failed to import configuration:', error);

            // Show error indicator
            if (typeof window.showNavbarStatus === 'function') {
                window.showNavbarStatus('error');
            }

            alert(t('runmode.settings.error.importfailed') + error.message);

            // Re-enable button
            importBtn.disabled = false;
        }
    }

    getNestedValue(obj, path) {
        return path.split('.').reduce((current, key) => {
            return current && current[key] !== undefined ? current[key] : undefined;
        }, obj);
    }

    setNestedValue(obj, path, value) {
        const keys = path.split('.');
        const lastKey = keys.pop();

        const target = keys.reduce((current, key) => {
            if (!current[key] || typeof current[key] !== 'object') {
                current[key] = {};
            }
            return current[key];
        }, obj);

        target[lastKey] = value;
    }

    showStatus(message, type) {
        // Only show navbar indicator for actual save operations (success/error from saves)
        // Don't show for 'info' type (loading messages) or other informational states
        // This prevents indicators from showing on page load or config reload
    }

    async checkSetupModeAvailability() {
        try {
            const response = await fetch('/api/system/info');
            if (response.ok) {
                const result = await response.json();
                if (result.setupModeAvailable) {
                    // Show setup mode and factory reset buttons
                    const setupBtn = document.getElementById('setup-mode');
                    const resetBtn = document.getElementById('factory-reset');
                    if (setupBtn) {
                        setupBtn.style.display = 'inline-block';
                    }
                    if (resetBtn) {
                        resetBtn.style.display = 'inline-block';
                    }

                    // Set SSH toggle state from the API response
                    const sshToggle = document.getElementById('ssh-enabled');
                    if (sshToggle && typeof result.sshEnabled === 'boolean') {
                        sshToggle.checked = result.sshEnabled;
                    }
                }
            }
        } catch (error) {
            console.error('Failed to check setup mode availability:', error);
        }
    }

    async restartApp() {
        if (!confirm(t('runmode.settings.confirm.restart'))) {
            return;
        }

        const restartBtn = document.getElementById('restart-app');

        try {
            // Don't disable the button - let user restart
            if (typeof window.showNavbarStatus === 'function') {
                window.showNavbarStatus('saving');
            }

            const response = await fetch('/api/system/restart', {
                method: 'POST'
            });

            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }

            // Show success indicator
            if (typeof window.showNavbarStatus === 'function') {
                window.showNavbarStatus('success');
            }

            // Hide restart required indicator since we're restarting
            this.hideRestartRequired();

            // Show restart overlay and start polling for reconnection
            this.showRestartOverlay();
            this.pollForReconnection();

        } catch (error) {
            console.error('Failed to restart application:', error);
            if (typeof window.showNavbarStatus === 'function') {
                window.showNavbarStatus('error');
            }
            alert(t('runmode.settings.error.restartfailed') + error.message);
        }
    }

    showRestartOverlay() {
        // Create overlay if it doesn't exist
        let overlay = document.getElementById('restart-overlay');
        if (!overlay) {
            overlay = document.createElement('div');
            overlay.id = 'restart-overlay';
            overlay.innerHTML = `
                <div class="restart-message">
                    <div style="display: flex; justify-content: center; margin-bottom: 1rem;">
                        <div class="spinner-border text-light" role="status">
                            <span class="visually-hidden">${t('runmode.settings.restart.overlay.restarting')}</span>
                        </div>
                    </div>
                    <h3 style="text-align: center;">${t('runmode.settings.restart.overlay.title')}</h3>
                    <p style="text-align: center;">${t('runmode.settings.restart.overlay.pleasewait')}</p>
                </div>
            `;
            document.body.appendChild(overlay);
        }

        // Set inline styles to ensure they override everything
        overlay.style.cssText = `
            display: flex !important;
            position: fixed !important;
            top: 0 !important;
            left: 0 !important;
            right: 0 !important;
            bottom: 0 !important;
            width: 100vw !important;
            height: 100vh !important;
            background-color: rgba(0, 0, 0, 0.9) !important;
            z-index: 999999 !important;
            justify-content: center !important;
            align-items: center !important;
            margin: 0 !important;
            padding: 0 !important;
        `;

        // Add blur class to body
        document.body.classList.add('restart-blur');
    }

    hideRestartOverlay() {
        const overlay = document.getElementById('restart-overlay');
        if (overlay) {
            overlay.style.display = 'none';
        }

        // Remove blur class from body
        document.body.classList.remove('restart-blur');
    }

    async pollForReconnection() {
        const maxAttempts = 60; // Try for 60 seconds
        let attempts = 0;

        const poll = async () => {
            attempts++;

            try {
                const response = await fetch('/api/config/status', {
                    method: 'GET',
                    cache: 'no-cache'
                });

                if (response.ok) {
                    // Successfully reconnected
                    this.hideRestartOverlay();
                    // Reload the page to ensure fresh state
                    window.location.reload();
                    return;
                }
            } catch (error) {
                // Expected during restart - server is down
            }

            if (attempts < maxAttempts) {
                // Try again in 1 second
                setTimeout(poll, 1000);
            } else {
                // Give up after max attempts
                this.hideRestartOverlay();
                alert(t('runmode.settings.error.reconnectfailed'));
            }
        };

        // Wait 2 seconds before first poll to give app time to start shutting down
        setTimeout(poll, 2000);
    }

    async enterSetupMode() {
        if (!confirm(t('runmode.settings.confirm.setupmode'))) {
            return;
        }

        const setupBtn = document.getElementById('setup-mode');

        try {
            setupBtn.disabled = true;
            if (typeof window.showNavbarStatus === 'function') {
                window.showNavbarStatus('saving');
            }

            const response = await fetch('/api/mode/setup', {
                method: 'POST'
            });

            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }

            // Show success indicator
            if (typeof window.showNavbarStatus === 'function') {
                window.showNavbarStatus('success');
            }

        } catch (error) {
            console.error('Failed to enter setup mode:', error);
            if (typeof window.showNavbarStatus === 'function') {
                window.showNavbarStatus('error');
            }
            alert(t('runmode.settings.error.setupmodefailed') + error.message);
            setupBtn.disabled = false;
        }
    }

    async toggleSSH(enabled) {
        const action = enabled ? 'enable' : 'disable';
        const toggle = document.getElementById('ssh-enabled');

        try {
            toggle.disabled = true;
            if (typeof window.showNavbarStatus === 'function') {
                window.showNavbarStatus('saving');
            }

            const response = await fetch(`/api/system/ssh/${action}`, {
                method: 'POST'
            });

            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }

            // Show success indicator
            if (typeof window.showNavbarStatus === 'function') {
                window.showNavbarStatus('success');
            }

        } catch (error) {
            console.error(`Failed to ${action} SSH:`, error);
            if (typeof window.showNavbarStatus === 'function') {
                window.showNavbarStatus('error');
            }
            alert(`Failed to ${action} SSH: ${error.message}`);
            // Revert toggle state on error
            toggle.checked = !enabled;
        } finally {
            toggle.disabled = false;
        }
    }

    async provisionSSH() {
        const publicKeyInput = document.getElementById('ssh-public-key');
        const provisionBtn = document.getElementById('provision-ssh-btn');
        const publicKey = publicKeyInput.value.trim();

        if (!publicKey) {
            alert(t('runmode.settings.error.sshkeyrequired') || 'Please enter an SSH public key');
            return;
        }

        try {
            provisionBtn.disabled = true;
            if (typeof window.showNavbarStatus === 'function') {
                window.showNavbarStatus('saving');
            }

            const response = await fetch('/api/system/ssh/provision', {
                method: 'POST',
                headers: {
                    'Content-Type': 'text/plain'
                },
                body: publicKey
            });

            if (!response.ok) {
                const errorData = await response.json().catch(() => ({}));
                throw new Error(errorData.error || `HTTP error! status: ${response.status}`);
            }

            // Show success indicator
            if (typeof window.showNavbarStatus === 'function') {
                window.showNavbarStatus('success');
            }

            alert(t('runmode.settings.success.sshprovisioned') || 'SSH key provisioned successfully');

            // Clear the input field
            publicKeyInput.value = '';

        } catch (error) {
            console.error('Failed to provision SSH key:', error);
            if (typeof window.showNavbarStatus === 'function') {
                window.showNavbarStatus('error');
            }
            alert(`Failed to provision SSH key: ${error.message}`);
        } finally {
            provisionBtn.disabled = false;
        }
    }

    async factoryReset() {
        // Show the modal dialog
        const modal = document.getElementById('factory-reset-modal');
        const input = document.getElementById('factory-reset-confirm-input');
        const confirmBtn = document.getElementById('factory-reset-confirm');
        const cancelBtn = document.getElementById('factory-reset-cancel');

        modal.style.display = 'flex';
        input.value = '';
        input.focus();
        confirmBtn.disabled = true;

        // Enable/disable confirm button based on input
        const checkInput = () => {
            confirmBtn.disabled = input.value.toLowerCase() !== 'reset';
        };

        input.addEventListener('input', checkInput);

        // Handle cancel
        const handleCancel = () => {
            modal.style.display = 'none';
            input.removeEventListener('input', checkInput);
            cancelBtn.removeEventListener('click', handleCancel);
            confirmBtn.removeEventListener('click', handleConfirm);
        };

        // Handle confirm
        const handleConfirm = async () => {
            if (input.value.toLowerCase() !== 'reset') {
                return;
            }

            // Close modal
            modal.style.display = 'none';
            input.removeEventListener('input', checkInput);
            cancelBtn.removeEventListener('click', handleCancel);
            confirmBtn.removeEventListener('click', handleConfirm);

            // Disable the factory reset button
            const factoryResetBtn = document.getElementById('factory-reset');
            factoryResetBtn.disabled = true;

            if (typeof window.showNavbarStatus === 'function') {
                window.showNavbarStatus('saving');
            }

            try {
                const response = await fetch('/api/system/factory-reset', {
                    method: 'POST'
                });

                if (!response.ok) {
                    throw new Error(`HTTP error! status: ${response.status}`);
                }

                // Show success indicator
                if (typeof window.showNavbarStatus === 'function') {
                    window.showNavbarStatus('success');
                }

                // Connection will be lost, no need to show further progress
                // The app will restart in setup mode

            } catch (error) {
                console.error('Failed to perform factory reset:', error);
                if (typeof window.showNavbarStatus === 'function') {
                    window.showNavbarStatus('error');
                }
                alert(t('runmode.settings.error.factoryresetfailed') + error.message);
                factoryResetBtn.disabled = false;
            }
        };

        cancelBtn.addEventListener('click', handleCancel);
        confirmBtn.addEventListener('click', handleConfirm);

        // Allow Enter key to confirm if text is correct
        input.addEventListener('keypress', (e) => {
            if (e.key === 'Enter' && input.value.toLowerCase() === 'reset') {
                handleConfirm();
            }
        });
    }

    // Utility method to validate input values
    validateInput(input) {
        const value = input.value;
        const min = parseFloat(input.min);
        const max = parseFloat(input.max);

        if (input.type === 'number') {
            const numValue = parseFloat(value);
            if (isNaN(numValue)) {
                return false;
            }
            if (!isNaN(min) && numValue < min) {
                return false;
            }
            if (!isNaN(max) && numValue > max) {
                return false;
            }
        }

        if (input.required && !value.trim()) {
            return false;
        }

        return true;
    }

    // Format engine profile key into human-readable name
    formatEngineProfileName(key) {
        // Engine layout names
        const layouts = {
            's': 'Two-Stroke',
            'i': 'Inline',
            'v': 'V',
            'w': 'W',
            'h': 'Flat',
            'k': 'Wankel'
        };

        // Wankel rotor count names
        const rotorNames = {
            '1': 'Single Rotor',
            '2': 'Twin Rotor',
            '3': 'Triple Rotor',
            '4': 'Quad Rotor'
        };

        // RPM category names
        const rpmCategories = {
            'rstd': 'std RPM',
            'rhigh': 'high RPM',
            'rmed': 'med RPM'
        };

        // Parse the key
        const match = key.match(/^([a-z]+)(\d+)(?:_b(\d+))?(?:_c(\d+))?(?:_(rstd|rhigh|rmed))?$/i);

        if (!match) {
            return key; // Return original if format doesn't match
        }

        const [, layout, cylinders, bank, crank, rpm] = match;

        // Special case: VR6 (V6 with 15° bank angle)
        if (layout.toLowerCase() === 'v' && cylinders === '6' && bank === '15') {
            let name = 'VR6';
            if (crank) {
                name += `, ${crank}º crank`;
            }
            if (rpm) {
                name += `, ${rpmCategories[rpm] || rpm}`;
            }
            return name;
        }

        // Special case: Single cylinder two-stroke
        if (layout.toLowerCase() === 's' && cylinders === '1') {
            let name = 'Two-Stroke';
            if (rpm) {
                name += `, ${rpmCategories[rpm] || rpm}`;
            }
            return name;
        }

        // Build the formatted name
        let name;

        // Special handling for Wankel rotary engines
        if (layout.toLowerCase() === 'k') {
            name = rotorNames[cylinders] || `${cylinders} Rotor`;
        } else {
            const layoutName = layouts[layout.toLowerCase()] || layout.toUpperCase();
            // V and W engines have no space between layout and cylinder count
            if (layout.toLowerCase() === 'v' || layout.toLowerCase() === 'w') {
                name = `${layoutName}${cylinders}`;
            } else {
                // Other layouts (Inline, Flat, Two-Stroke) have a space
                name = `${layoutName} ${cylinders}`;
            }
        }

        if (bank) {
            name += `, ${bank}º bank`;
        }

        if (crank) {
            name += `, ${crank}º crank`;
        }

        if (rpm) {
            name += `, ${rpmCategories[rpm] || rpm}`;
        }

        return name;
    }

    // Initialize engine profiles dropdown and settings
    initEngineProfiles() {
        const profileSelect = document.getElementById('engine-profile-select');
        const profileSettings = document.getElementById('engine-profile-settings');

        if (!profileSelect || !this.config.synthesizer || !this.config.synthesizer.engineProfiles) {
            return;
        }

        // Populate dropdown with engine profiles
        profileSelect.innerHTML = `<option value="" disabled selected>${t('runmode.settings.status.selectprofile')}</option>`;

        const profiles = this.config.synthesizer.engineProfiles;

        // Define custom layout order: Inline, Flat, V, W, Wankel, Two-Stroke
        const layoutOrder = { 'i': 1, 'h': 2, 'v': 3, 'w': 4, 'k': 5, 's': 6 };

        // Sort by layout type, then cylinder count, then alphabetically
        const sortedKeys = Object.keys(profiles).sort((a, b) => {
            // Extract layout and cylinder count from keys
            const matchA = a.match(/^([a-z]+)(\d+)/i);
            const matchB = b.match(/^([a-z]+)(\d+)/i);

            if (!matchA || !matchB) return a.localeCompare(b);

            const layoutA = matchA[1].toLowerCase();
            const layoutB = matchB[1].toLowerCase();
            const cylindersA = parseInt(matchA[2]);
            const cylindersB = parseInt(matchB[2]);

            // Sort by custom layout order
            const orderA = layoutOrder[layoutA] || 999;
            const orderB = layoutOrder[layoutB] || 999;

            if (orderA !== orderB) {
                return orderA - orderB;
            }

            // Then by cylinder count
            if (cylindersA !== cylindersB) {
                return cylindersA - cylindersB;
            }

            // Finally alphabetically by full key
            return a.localeCompare(b);
        });

        sortedKeys.forEach(key => {
            const option = document.createElement('option');
            option.value = key;
            option.textContent = this.formatEngineProfileName(key);
            profileSelect.appendChild(option);
        });

        // Auto-select first profile if available
        if (sortedKeys.length > 0) {
            const firstProfile = sortedKeys[0];
            profileSelect.value = firstProfile;
            this.loadEngineProfile(firstProfile, profiles[firstProfile]);
            profileSettings.style.display = 'block';
            // Initialize touch spinners for engine profile fields
            if (typeof window.initTouchSpinners === 'function') {
                window.initTouchSpinners();
            }
        }

        // Handle profile selection
        profileSelect.addEventListener('change', () => {
            const selectedProfile = profileSelect.value;
            if (selectedProfile && profiles[selectedProfile]) {
                this.loadEngineProfile(selectedProfile, profiles[selectedProfile]);
                profileSettings.style.display = 'block';
            } else {
                profileSettings.style.display = 'none';
            }
        });

        // Setup input handlers for engine profile settings
        ['engine-primarybalance', 'engine-secondarybalance', 'engine-gain', 'engine-pulsescale'].forEach(id => {
            const input = document.getElementById(id);
            if (input) {
                input.addEventListener('change', () => this.saveEngineProfile());
                input.addEventListener('input', () => this.debounceEngineProfileSave());
            }
        });
    }

    // Load engine profile data into form
    loadEngineProfile(profileName, profile) {
        document.getElementById('engine-primarybalance').value = this.formatDecimalValue(profile.primaryBalance);
        document.getElementById('engine-secondarybalance').value = this.formatDecimalValue(profile.secondaryBalance);
        document.getElementById('engine-gain').value = profile.gain;
        document.getElementById('engine-pulsescale').value = this.formatDecimalValue(profile.pulseScale);
    }

    // Debounce save for engine profile
    debounceEngineProfileSave() {
        clearTimeout(this.engineProfileSaveTimeout);
        this.engineProfileSaveTimeout = setTimeout(() => this.saveEngineProfile(), 1000);
    }

    // Save engine profile changes
    async saveEngineProfile() {
        const profileSelect = document.getElementById('engine-profile-select');
        const selectedProfile = profileSelect.value;

        if (!selectedProfile) return;

        const profile = {
            primaryBalance: parseFloat(document.getElementById('engine-primarybalance').value),
            secondaryBalance: parseFloat(document.getElementById('engine-secondarybalance').value),
            gain: parseFloat(document.getElementById('engine-gain').value),
            pulseScale: parseFloat(document.getElementById('engine-pulsescale').value)
        };

        try {
            // Show saving indicator
            if (typeof window.showNavbarStatus === 'function') {
                window.showNavbarStatus('saving');
            }

            // Update local config
            if (!this.config.synthesizer.engineProfiles) {
                this.config.synthesizer.engineProfiles = {};
            }
            this.config.synthesizer.engineProfiles[selectedProfile] = profile;

            // Save to server
            const formData = {
                synthesizer: {
                    engineProfiles: {
                        [selectedProfile]: profile
                    }
                }
            };

            const response = await fetch('/api/config', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify(formData)
            });

            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }

            // Show success indicator
            if (typeof window.showNavbarStatus === 'function') {
                window.showNavbarStatus('success');
            }
        } catch (error) {
            console.error('Failed to save engine profile:', error);
            // Show error indicator
            if (typeof window.showNavbarStatus === 'function') {
                window.showNavbarStatus('error');
            }
            alert(t('runmode.settings.error.saveprofilefailed') + error.message);
        }
    }

    // Get the minimum EQ frequency from pulse config
    getEqMinFreq() {
        return this.config?.haptics?.pulseMinFrequencyHz || 8;
    }

    // Get the maximum EQ frequency from pulse config
    getEqMaxFreq() {
        return this.config?.haptics?.pulseMaxFrequencyHz || 60;
    }

    // Get the EQ frequency range (max - min)
    getEqFreqRange() {
        return this.getEqMaxFreq() - this.getEqMinFreq();
    }

    // Initialize equalizer controls
    initEqualizer() {
        const channelSelect = document.getElementById('eq-channel-select');
        const bandSelect = document.getElementById('eq-band-select');
        const frequencySlider = document.getElementById('eq-frequency-slider');
        const gainSlider = document.getElementById('eq-gain-slider');
        const qSlider = document.getElementById('eq-q-slider');
        const frequencyValue = document.getElementById('eq-frequency-value');
        const gainValue = document.getElementById('eq-gain-value');
        const qFactor = document.getElementById('eq-q-factor');
        const resetBtn = document.getElementById('eq-reset-btn');
        const eqEnabledCheckbox = document.getElementById('synth-eqenabled');

        if (!bandSelect || !frequencySlider || !gainSlider || !qSlider || !this.config.synthesizer) {
            return;
        }

        // Update frequency slider bounds from pulse config
        frequencySlider.min = this.getEqMinFreq();
        frequencySlider.max = this.getEqMaxFreq();

        // Initialize current channel
        this.currentChannel = 0;

        // Get EQ bands array (per-channel: [[{frequency, gain, q}, ...], [{frequency, gain, q}, ...]])
        // Deep copy to avoid reference issues with config object
        let eqBandsAll = JSON.parse(JSON.stringify(this.config.synthesizer.eq || []));

        // Default bands for both channels
        const defaultBands = [
            { frequency: 12, gain: 0.0, q: 2.0 },
            { frequency: 16, gain: 0.0, q: 2.0 },
            { frequency: 20, gain: 0.0, q: 2.0 },
            { frequency: 25, gain: 0.0, q: 2.0 },
            { frequency: 30, gain: 0.0, q: 2.0 },
            { frequency: 38, gain: 0.0, q: 2.0 },
            { frequency: 48, gain: 0.0, q: 2.0 },
            { frequency: 58, gain: 0.0, q: 2.0 }
        ];

        // Initialize eqBandsAll as array of 2 channels
        if (!Array.isArray(eqBandsAll) || eqBandsAll.length !== 2) {
            eqBandsAll = [
                JSON.parse(JSON.stringify(defaultBands)),
                JSON.parse(JSON.stringify(defaultBands))
            ];
        } else {
            // Validate each channel has 8 bands
            for (let ch = 0; ch < 2; ch++) {
                if (!Array.isArray(eqBandsAll[ch]) || eqBandsAll[ch].length !== 8) {
                    eqBandsAll[ch] = JSON.parse(JSON.stringify(defaultBands));
                }
            }
        }

        // Store all channel bands
        this.eqBandsAll = eqBandsAll;
        // Set current working bands (for current channel)
        this.eqBands = this.eqBandsAll[this.currentChannel];
        this.currentBandIndex = 0;
        this.isDraggingBand = false;
        this.draggedBandIndex = -1;

        // Set EQ enabled checkbox (use current channel's enabled state)
        if (eqEnabledCheckbox) {
            const enableEQ = this.config.synthesizer.enableEQ;
            if (Array.isArray(enableEQ) && enableEQ.length > this.currentChannel) {
                eqEnabledCheckbox.checked = enableEQ[this.currentChannel];
            } else {
                eqEnabledCheckbox.checked = false;
            }
            // Listen for checkbox changes and trigger EQ save
            eqEnabledCheckbox.addEventListener('change', () => {
                this.debounceEqSave();
            });
        }

        // Channel select change event
        if (channelSelect) {
            channelSelect.addEventListener('change', (e) => {
                this.currentChannel = parseInt(e.target.value);
                this.eqBands = this.eqBandsAll[this.currentChannel];
                this.currentBandIndex = 0;

                // Update EQ enabled checkbox for this channel
                if (eqEnabledCheckbox) {
                    const enableEQ = this.config.synthesizer.enableEQ;
                    if (Array.isArray(enableEQ) && enableEQ.length > this.currentChannel) {
                        eqEnabledCheckbox.checked = enableEQ[this.currentChannel];
                    } else {
                        eqEnabledCheckbox.checked = false;
                    }
                }

                this.updateBandSelect();
                this.loadBandValues(0);
                this.updateFrequencyConstraints();
                this.drawEqCurve();
            });
        }

        // Populate band select dropdown
        this.updateBandSelect();

        // Load initial band values
        this.loadBandValues(0);

        // Band select change event
        bandSelect.addEventListener('change', (e) => {
            this.currentBandIndex = parseInt(e.target.value);
            this.loadBandValues(this.currentBandIndex);
            this.updateFrequencyConstraints();
            this.drawEqCurve(); // Redraw to highlight selected band
        });

        // Frequency slider events
        frequencySlider.addEventListener('input', (e) => {
            const value = parseFloat(e.target.value);
            frequencyValue.textContent = `${value} Hz`;
            this.eqBands[this.currentBandIndex].frequency = value;
            this.updateBandSelect(); // Update dropdown to show new frequency
            this.updateFrequencyConstraints(); // Update constraints for neighboring bands
            this.drawEqCurve(); // Redraw to show updated position
            this.debounceEqSave();
        });

        // Gain slider events
        gainSlider.addEventListener('input', (e) => {
            const value = parseFloat(e.target.value);
            gainValue.textContent = `${value > 0 ? '+' : ''}${value.toFixed(1)} dB`;
            this.eqBands[this.currentBandIndex].gain = value;
            this.drawEqCurve(); // Redraw to show updated position
            this.debounceEqSave();
        });

        // Q slider events
        qSlider.addEventListener('input', (e) => {
            const value = parseFloat(e.target.value);
            qFactor.textContent = value.toFixed(1);
            this.eqBands[this.currentBandIndex].q = value;
            this.debounceEqSave();
        });

        // Draw initial curve
        this.drawEqCurve();

        // Add mouse handlers to canvas for selecting and dragging bands
        const canvas = document.getElementById('eq-curve-canvas');
        if (canvas) {
            canvas.style.cursor = 'pointer';

            canvas.addEventListener('mousedown', (e) => {
                this.handleCanvasMouseDown(e);
            });

            canvas.addEventListener('mousemove', (e) => {
                this.handleCanvasMouseMove(e);
            });

            canvas.addEventListener('mouseup', (e) => {
                this.handleCanvasMouseUp(e);
            });

            canvas.addEventListener('mouseleave', (e) => {
                this.handleCanvasMouseUp(e);
            });
        }

        // Setup reset button
        if (resetBtn) {
            resetBtn.addEventListener('click', () => {
                const defaultBands = [
                    { frequency: 12, gain: 0.0, q: 2.0 },
                    { frequency: 16, gain: 0.0, q: 2.0 },
                    { frequency: 20, gain: 0.0, q: 2.0 },
                    { frequency: 25, gain: 0.0, q: 2.0 },
                    { frequency: 30, gain: 0.0, q: 2.0 },
                    { frequency: 38, gain: 0.0, q: 2.0 },
                    { frequency: 48, gain: 0.0, q: 2.0 },
                    { frequency: 58, gain: 0.0, q: 2.0 }
                ];
                // Reset current channel's bands
                this.eqBandsAll[this.currentChannel] = JSON.parse(JSON.stringify(defaultBands));
                this.eqBands = this.eqBandsAll[this.currentChannel];
                this.updateBandSelect();
                this.loadBandValues(this.currentBandIndex);
                this.saveEqualizer();
            });
        }
    }

    // Update band select dropdown options
    updateBandSelect() {
        const bandSelect = document.getElementById('eq-band-select');
        if (!bandSelect || !this.eqBands) return;

        bandSelect.innerHTML = '';
        this.eqBands.forEach((band, index) => {
            const freq = band.frequency !== undefined ? band.frequency : (band.Frequency !== undefined ? band.Frequency : 12);
            const option = document.createElement('option');
            option.value = index;
            option.textContent = `${t('runmode.settings.synth.eqband')} ${index + 1} (${freq} Hz)`;
            if (index === this.currentBandIndex) {
                option.selected = true;
            }
            bandSelect.appendChild(option);
        });
    }

    // Load values for selected band into sliders
    loadBandValues(index) {
        const frequencySlider = document.getElementById('eq-frequency-slider');
        const gainSlider = document.getElementById('eq-gain-slider');
        const qSlider = document.getElementById('eq-q-slider');
        const frequencyValue = document.getElementById('eq-frequency-value');
        const gainValue = document.getElementById('eq-gain-value');
        const qFactor = document.getElementById('eq-q-factor');

        if (!this.eqBands || index < 0 || index >= this.eqBands.length) return;

        const band = this.eqBands[index];
        const freq = band.frequency !== undefined ? band.frequency : (band.Frequency !== undefined ? band.Frequency : 12);
        const gain = band.gain !== undefined ? band.gain : (band.Gain !== undefined ? band.Gain : 0.0);
        const q = band.q !== undefined ? band.q : (band.Q !== undefined ? band.Q : 2.0);

        if (frequencySlider) {
            frequencySlider.value = freq;
            frequencyValue.textContent = `${freq} Hz`;
        }

        if (gainSlider) {
            gainSlider.value = gain;
            gainValue.textContent = `${gain > 0 ? '+' : ''}${gain.toFixed(1)} dB`;
        }

        if (qSlider) {
            qSlider.value = q;
            qFactor.textContent = q.toFixed(1);
        }
    }

    // Update frequency slider constraints based on neighboring bands
    updateFrequencyConstraints() {
        const frequencySlider = document.getElementById('eq-frequency-slider');
        if (!frequencySlider || !this.eqBands) return;

        const index = this.currentBandIndex;
        let minFreq = this.getEqMinFreq();  // Dynamic minimum from pulse config
        let maxFreq = this.getEqMaxFreq();  // Dynamic maximum from pulse config

        // Get frequency of previous band (if exists)
        if (index > 0) {
            const prevBand = this.eqBands[index - 1];
            const prevFreq = prevBand.frequency !== undefined ? prevBand.frequency : (prevBand.Frequency !== undefined ? prevBand.Frequency : minFreq);
            minFreq = prevFreq + 0.5; // Must be at least 0.5 Hz above previous band
        }

        // Get frequency of next band (if exists)
        if (index < this.eqBands.length - 1) {
            const nextBand = this.eqBands[index + 1];
            const nextFreq = nextBand.frequency !== undefined ? nextBand.frequency : (nextBand.Frequency !== undefined ? nextBand.Frequency : maxFreq);
            maxFreq = nextFreq - 0.5; // Must be at least 0.5 Hz below next band
        }

        frequencySlider.min = minFreq;
        frequencySlider.max = maxFreq;

        // Constrain current value if it's out of bounds
        const currentFreq = parseFloat(frequencySlider.value);
        if (currentFreq < minFreq) {
            frequencySlider.value = minFreq;
            this.eqBands[index].frequency = minFreq;
        } else if (currentFreq > maxFreq) {
            frequencySlider.value = maxFreq;
            this.eqBands[index].frequency = maxFreq;
        }
    }

    // Handle canvas mouse down to start dragging
    handleCanvasMouseDown(event) {
        const canvas = document.getElementById('eq-curve-canvas');
        if (!canvas || !this.eqBands) return;

        const rect = canvas.getBoundingClientRect();
        const scaleX = canvas.width / rect.width;
        const scaleY = canvas.height / rect.height;
        const x = (event.clientX - rect.left) * scaleX;
        const y = (event.clientY - rect.top) * scaleY;

        const width = canvas.width;
        const height = canvas.height;

        // Get dynamic frequency range
        const eqMinFreq = this.getEqMinFreq();
        const eqFreqRange = this.getEqFreqRange();

        // Check if mouse down is near any band dot
        const clickRadius = 15;

        for (let index = 0; index < this.eqBands.length; index++) {
            const band = this.eqBands[index];
            const freq = band.frequency !== undefined ? band.frequency : (band.Frequency !== undefined ? band.Frequency : 12);
            const gain = band.gain !== undefined ? band.gain : (band.Gain !== undefined ? band.Gain : 0.0);

            const bandX = ((freq - eqMinFreq) / eqFreqRange) * width;
            const bandY = height / 2 - (gain / 18) * height / 2;

            const distance = Math.sqrt(Math.pow(x - bandX, 2) + Math.pow(y - bandY, 2));

            if (distance < clickRadius) {
                this.isDraggingBand = true;
                this.draggedBandIndex = index;
                this.currentBandIndex = index;

                // Update dropdown
                const bandSelect = document.getElementById('eq-band-select');
                if (bandSelect) {
                    bandSelect.value = index;
                }

                // Update sliders
                this.loadBandValues(index);
                this.updateFrequencyConstraints();

                // Redraw to highlight selected band
                this.drawEqCurve();

                canvas.style.cursor = 'grabbing';
                break;
            }
        }
    }

    // Handle canvas mouse move for dragging
    handleCanvasMouseMove(event) {
        if (!this.isDraggingBand || this.draggedBandIndex === -1) return;

        const canvas = document.getElementById('eq-curve-canvas');
        if (!canvas || !this.eqBands) return;

        const rect = canvas.getBoundingClientRect();
        const scaleX = canvas.width / rect.width;
        const scaleY = canvas.height / rect.height;
        const x = (event.clientX - rect.left) * scaleX;
        const y = (event.clientY - rect.top) * scaleY;

        const width = canvas.width;
        const height = canvas.height;

        // Convert x position to frequency (dynamic range from pulse config)
        const eqMinFreq = this.getEqMinFreq();
        const eqMaxFreq = this.getEqMaxFreq();
        const eqFreqRange = this.getEqFreqRange();
        let frequency = (x / width) * eqFreqRange + eqMinFreq;

        // Apply frequency constraints
        const index = this.draggedBandIndex;
        let minFreq = eqMinFreq;
        let maxFreq = eqMaxFreq;

        if (index > 0) {
            const prevBand = this.eqBands[index - 1];
            const prevFreq = prevBand.frequency !== undefined ? prevBand.frequency : (prevBand.Frequency !== undefined ? prevBand.Frequency : eqMinFreq);
            minFreq = prevFreq + 0.5;
        }

        if (index < this.eqBands.length - 1) {
            const nextBand = this.eqBands[index + 1];
            const nextFreq = nextBand.frequency !== undefined ? nextBand.frequency : (nextBand.Frequency !== undefined ? nextBand.Frequency : eqMaxFreq);
            maxFreq = nextFreq - 0.5;
        }

        frequency = Math.max(minFreq, Math.min(maxFreq, frequency));
        frequency = Math.round(frequency * 2) / 2; // Round to nearest 0.5

        // Convert y position to gain (-12 to +6 dB)
        let gain = ((height / 2 - y) / (height / 2)) * 18;
        gain = Math.max(-12, Math.min(6, gain));
        gain = Math.round(gain * 2) / 2; // Round to nearest 0.5

        // Update band values
        this.eqBands[index].frequency = frequency;
        this.eqBands[index].gain = gain;

        // Update sliders and values
        const frequencySlider = document.getElementById('eq-frequency-slider');
        const gainSlider = document.getElementById('eq-gain-slider');
        const frequencyValue = document.getElementById('eq-frequency-value');
        const gainValue = document.getElementById('eq-gain-value');

        if (frequencySlider) {
            frequencySlider.value = frequency;
            frequencyValue.textContent = `${frequency} Hz`;
        }

        if (gainSlider) {
            gainSlider.value = gain;
            gainValue.textContent = `${gain > 0 ? '+' : ''}${gain.toFixed(1)} dB`;
        }

        // Update dropdown and constraints
        this.updateBandSelect();
        this.updateFrequencyConstraints();

        // Redraw curve
        this.drawEqCurve();
    }

    // Handle canvas mouse up to stop dragging
    handleCanvasMouseUp(event) {
        if (this.isDraggingBand) {
            this.isDraggingBand = false;
            this.draggedBandIndex = -1;

            const canvas = document.getElementById('eq-curve-canvas');
            if (canvas) {
                canvas.style.cursor = 'pointer';
            }

            // Save changes
            this.debounceEqSave();
        }
    }

    // Handle canvas click to select bands (legacy - now handled by mousedown)
    handleCanvasClick(event) {
        // No longer needed - selection is handled in mousedown
    }

    // Draw EQ curve visualization
    drawEqCurve() {
        const canvas = document.getElementById('eq-curve-canvas');
        if (!canvas || !this.config.eqCurve) return;

        const ctx = canvas.getContext('2d');
        const width = canvas.width;
        const height = canvas.height;
        // Get the curve for the current channel
        const curves = this.config.eqCurve.curve || [];
        const curve = Array.isArray(curves[this.currentChannel]) ? curves[this.currentChannel] : [];
        const minFreq = this.config.eqCurve.minFreq || 10;
        const resolution = this.config.eqCurve.resolution || 0.5;

        // Clear canvas
        ctx.clearRect(0, 0, width, height);

        // Draw background grid
        ctx.strokeStyle = 'rgba(255, 255, 255, 0.1)';
        ctx.lineWidth = 1;

        // Horizontal grid lines (dB levels)
        const dbLevels = [-12, -6, 0, 6];
        dbLevels.forEach(db => {
            const y = height / 2 - (db / 18) * height / 2; // Scale to canvas
            ctx.beginPath();
            ctx.moveTo(0, y);
            ctx.lineTo(width, y);
            ctx.stroke();

            // Label
            ctx.fillStyle = 'rgba(255, 255, 255, 0.4)';
            ctx.font = '10px sans-serif';
            ctx.fillText(`${db > 0 ? '+' : ''}${db}dB`, 5, y - 3);
        });

        // Draw 0dB line thicker
        ctx.strokeStyle = 'rgba(255, 255, 255, 0.3)';
        ctx.lineWidth = 2;
        ctx.beginPath();
        ctx.moveTo(0, height / 2);
        ctx.lineTo(width, height / 2);
        ctx.stroke();

        // Get dynamic frequency range from pulse config
        const eqMinFreq = this.getEqMinFreq();
        const eqMaxFreq = this.getEqMaxFreq();
        const eqFreqRange = this.getEqFreqRange();

        // Draw curve
        if (curve.length > 0) {
            ctx.strokeStyle = '#0d6efd';
            ctx.lineWidth = 2;
            ctx.beginPath();

            curve.forEach((value, i) => {
                const freq = minFreq + i * resolution;
                const x = ((freq - eqMinFreq) / eqFreqRange) * width;
                // Convert amplitude ratio to dB for display
                const db = 20 * Math.log10(value || 1);
                const y = height / 2 - (db / 18) * height / 2;

                if (i === 0) {
                    ctx.moveTo(x, y);
                } else {
                    ctx.lineTo(x, y);
                }
            });

            ctx.stroke();
        }

        // Draw band markers as dots
        if (this.eqBands && this.eqBands.length > 0) {
            this.eqBands.forEach((band, index) => {
                const freq = band.frequency !== undefined ? band.frequency : (band.Frequency !== undefined ? band.Frequency : 12);
                const gain = band.gain !== undefined ? band.gain : (band.Gain !== undefined ? band.Gain : 0.0);

                const x = ((freq - eqMinFreq) / eqFreqRange) * width;
                const y = height / 2 - (gain / 18) * height / 2;

                // Draw dot
                ctx.beginPath();
                ctx.arc(x, y, 5, 0, 2 * Math.PI);

                // Highlight active band
                if (index === this.currentBandIndex) {
                    ctx.fillStyle = '#ff9800'; // Orange for active band
                } else {
                    ctx.fillStyle = '#0d6efd'; // Blue for inactive bands
                }
                ctx.fill();
            });
        }

        // Draw frequency markers dynamically based on range
        ctx.fillStyle = 'rgba(255, 255, 255, 0.4)';
        ctx.font = '10px sans-serif';
        // Generate markers at reasonable intervals
        const markerStep = eqFreqRange <= 60 ? 10 : (eqFreqRange <= 120 ? 20 : 40);
        const startMarker = Math.ceil(eqMinFreq / markerStep) * markerStep;
        for (let freq = startMarker; freq <= eqMaxFreq; freq += markerStep) {
            const x = ((freq - eqMinFreq) / eqFreqRange) * width;
            ctx.fillText(`${freq}Hz`, x - 10, height - 5);
        }
    }

    // Debounce save for equalizer
    debounceEqSave() {
        clearTimeout(this.eqSaveTimeout);
        this.eqSaveTimeout = setTimeout(() => this.saveEqualizer(), 1000);
    }

    // Save equalizer settings
    async saveEqualizer() {
        const eqEnabledCheckbox = document.getElementById('synth-eqenabled');

        if (!this.eqBandsAll || this.eqBandsAll.length !== 2) {
            console.error('EQ must have exactly 2 channels');
            return;
        }

        // Validate each channel has 8 bands
        for (let ch = 0; ch < 2; ch++) {
            if (!this.eqBandsAll[ch] || this.eqBandsAll[ch].length !== 8) {
                console.error(`Channel ${ch} must have exactly 8 bands`);
                return;
            }
        }

        try {
            // Show saving indicator
            if (typeof window.showNavbarStatus === 'function') {
                window.showNavbarStatus('saving');
            }

            // Normalize bands for all channels to ensure lowercase field names
            const normalizedBandsAll = this.eqBandsAll.map(channelBands =>
                channelBands.map(band => ({
                    frequency: band.frequency !== undefined ? band.frequency : (band.Frequency !== undefined ? band.Frequency : 0),
                    gain: band.gain !== undefined ? band.gain : (band.Gain !== undefined ? band.Gain : 0),
                    q: band.q !== undefined ? band.q : (band.Q !== undefined ? band.Q : 2.0)
                }))
            );

            // Update enableEQ for the current channel
            let enableEQArray = this.config.synthesizer.enableEQ;
            if (!Array.isArray(enableEQArray) || enableEQArray.length !== 2) {
                enableEQArray = [false, false];
            }
            if (eqEnabledCheckbox) {
                enableEQArray[this.currentChannel] = eqEnabledCheckbox.checked;
            }

            // Update local config
            this.config.synthesizer.eq = normalizedBandsAll;
            this.config.synthesizer.enableEQ = enableEQArray;

            // Save to server
            const formData = {
                synthesizer: {
                    enableEQ: enableEQArray,
                    eq: normalizedBandsAll
                }
            };

            const response = await fetch('/api/config', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify(formData)
            });

            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }

            const result = await response.json();

            // Update curve data and redraw
            if (result.config && result.config.eqCurve) {
                this.config.eqCurve = result.config.eqCurve;
                this.drawEqCurve();
            }

            // Show success indicator
            if (typeof window.showNavbarStatus === 'function') {
                window.showNavbarStatus('success');
            }
        } catch (error) {
            console.error('Failed to save equalizer:', error);
            // Show error indicator
            if (typeof window.showNavbarStatus === 'function') {
                window.showNavbarStatus('error');
            }
        }
    }
}

// Initialize the configuration manager when the page loads
document.addEventListener('DOMContentLoaded', () => {
    window.configManager = new ConfigManager();
});

// Add some utility functions for enhanced UX
document.addEventListener('DOMContentLoaded', () => {
    // Add input validation feedback
    const inputs = document.querySelectorAll('[data-config]');
    inputs.forEach(input => {
        input.addEventListener('blur', () => {
            if (window.configManager && !window.configManager.validateInput(input)) {
                input.style.borderColor = '#e74c3c';
            } else {
                input.style.borderColor = '';
            }
        });
    });

    // Add tooltips for complex settings (if needed)
    const complexInputs = {
        'hardware-displayorientation': 'Display rotation for screens that are mounted in different orientations',
        'haptics-jerkcurve': 'Controls the responsiveness curve for jerk feedback (higher = more responsive)',
        'haptics-snapcurve': 'Controls the responsiveness curve for snap feedback (higher = more responsive)',
        'synth-mastergain': 'Overall volume level for all haptic feedback in decibels',
        'telemetry-source': 'UDP endpoint where telemetry data is received from the racing game'
    };

    Object.entries(complexInputs).forEach(([id, tooltip]) => {
        const element = document.getElementById(id);
        if (element) {
            element.title = tooltip;
        }
    });
});