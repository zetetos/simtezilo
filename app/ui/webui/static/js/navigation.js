// Navigation component for Simtezilo Web UI
function createNavigation(currentPage) {
    // Build telemetry dropdown - conditionally include Developer page based on devToolsEnabled
    const telemetryDropdown = [
        { path: '/telemetry', nameKey: 'runmode.nav.race', fallback: 'Race', id: 'telemetry' }
    ];

    // Only add Developer page if dev tools is enabled
    if (window.devToolsEnabled) {
        telemetryDropdown.push({ path: '/dev', nameKey: 'runmode.nav.developer', fallback: 'Developer', id: 'dev' });
    }

    const pages = [
        { path: '/settings', nameKey: 'runmode.nav.settings', fallback: 'Settings', id: 'settings' },
        { path: '/logs', nameKey: 'runmode.nav.logs', fallback: 'Logs', id: 'logs' }
    ];

    const telemetryActive = telemetryDropdown.some(item => item.id === currentPage);

    let navHTML = `
        <nav class="navbar navbar-expand-lg" style="background-color: var(--bs-content-bg); border-bottom: var(--bs-border-width) solid var(--bs-content-border-color);">
            <div class="container-fluid">
                <a class="navbar-brand" href="/">
                    <img src="/images/simtezilo-logo-dark.svg" alt="Simtezilo" height="32" class="d-inline-block align-text-top">
                </a>
                <button class="navbar-toggler" type="button" data-bs-toggle="collapse" data-bs-target="#navbar-collapse-1" aria-controls="navbar-collapse-1" aria-expanded="false" aria-label="Toggle navigation">
                    <span class="navbar-toggler-icon"></span>
                </button>
                <div class="collapse navbar-collapse" id="navbar-collapse-1">
                    <ul class="navbar-nav me-auto mb-2 mb-lg-0">
                        <li class="nav-item dropdown">
                            <a class="nav-link dropdown-toggle${telemetryActive ? ' active' : ''}"${telemetryActive ? ' aria-current="page"' : ''} href="#" role="button" data-bs-toggle="dropdown" aria-expanded="false" data-i18n="runmode.nav.telemetry">
                                Telemetry
                            </a>
                            <ul class="dropdown-menu mt-lg-2 rounded-top-0">`;

    telemetryDropdown.forEach((item, index) => {
        const isActive = item.id === currentPage;
        navHTML += `
                                <li><a class="dropdown-item${isActive ? ' active' : ''}" href="${item.path}" data-i18n="${item.nameKey}">${item.fallback}</a></li>`;
    });

    navHTML += `
                            </ul>
                        </li>`;

    pages.forEach(page => {
        const isActive = page.id === currentPage;
        navHTML += `
                        <li class="nav-item">
                            <a class="nav-link${isActive ? ' active' : ''}"${isActive ? ' aria-current="page"' : ''} href="${page.path}" data-i18n="${page.nameKey}">${page.fallback}</a>
                        </li>`;
    });

    // Add Info nav item with popover
    navHTML += `
                        <li class="nav-item">
                            <a class="nav-link" href="#" id="info-nav-link" data-i18n="runmode.nav.info">Info</a>
                        </li>`;

    navHTML += `
                    </ul>`;

    // Add status indicators at the far right
    navHTML += `
                    <div class="d-flex align-items-center me-3">`;

    // Add restart required indicator
    navHTML += `
                        <div id="restart-required-indicator" style="color: #ff4d4d; font-weight: 600; font-size: 0.875rem; white-space: nowrap; display: none; margin-right: 1rem;">
                            <span data-i18n="runmode.settings.restart.required">Restart Required</span>
                        </div>`;

    // Add unified status indicator (used for both telemetry connection and settings save status)
    navHTML += `
                        <div id="nav-status-indicator" class="d-flex align-items-center justify-content-center" style="width: 24px; height: 24px; visibility: hidden;">
                            <div id="nav-spinner" class="spinner-border spinner-border-sm text-warning" role="status" style="display: none;">
                                <span class="visually-hidden">Loading...</span>
                            </div>
                            <span id="nav-success-icon" class="icon text-success" style="display: none; font-size: 1.25rem;"></span>
                            <span id="nav-error-icon" class="icon text-danger" style="display: none; font-size: 1.25rem;"></span>
                        </div>
                    </div>`;

    navHTML += `
                </div>
            </div>
        </nav>`;

    return navHTML;
}

// Helper functions to control the navbar status indicator
window.showNavbarStatus = function (type) {
    const indicator = document.getElementById('nav-status-indicator');
    const spinner = document.getElementById('nav-spinner');
    const successIcon = document.getElementById('nav-success-icon');
    const errorIcon = document.getElementById('nav-error-icon');

    if (!indicator) return;

    // Hide all indicators first
    spinner.style.display = 'none';
    successIcon.style.display = 'none';
    errorIcon.style.display = 'none';

    // Show the container (make visible)
    indicator.style.visibility = 'visible';

    // Show appropriate indicator
    switch (type) {
        case 'saving':
            spinner.style.display = 'block';
            break;
        case 'success':
            successIcon.style.display = 'block';
            // Auto-hide after 3 seconds
            setTimeout(() => window.hideNavbarStatus(), 3000);
            break;
        case 'error':
            errorIcon.style.display = 'block';
            // Auto-hide after 3 seconds
            setTimeout(() => window.hideNavbarStatus(), 3000);
            break;
    }
};

window.hideNavbarStatus = function () {
    const indicator = document.getElementById('nav-status-indicator');
    const spinner = document.getElementById('nav-spinner');
    const successIcon = document.getElementById('nav-success-icon');
    const errorIcon = document.getElementById('nav-error-icon');

    if (indicator) {
        indicator.style.visibility = 'hidden';
    }

    // Also hide individual icons
    if (spinner) spinner.style.display = 'none';
    if (successIcon) successIcon.style.display = 'none';
    if (errorIcon) errorIcon.style.display = 'none';
};

// Initialize navigation when DOM is loaded
document.addEventListener('DOMContentLoaded', function () {
    // Fetch devToolsEnabled setting before initializing navigation
    fetch('/api/config')
        .then(response => response.ok ? response.json() : null)
        .then(config => {
            if (config && config.app) {
                window.devToolsEnabled = config.app.enableDevTools === true;
            }
            initializeNavigation();
        })
        .catch(error => {
            console.error('Failed to fetch config for navigation:', error);
            initializeNavigation();
        });
});

// Function to initialize or reinitialize navigation
function initializeNavigation() {
    const navContainer = document.getElementById('navigation');
    if (navContainer && typeof currentPageId !== 'undefined') {
        // Create navigation immediately with fallback text
        navContainer.innerHTML = createNavigation(currentPageId);

        // Add popover element to body if not exists
        if (!document.getElementById('info-popover')) {
            const popover = document.createElement('div');
            popover.id = 'info-popover';
            popover.className = 'card shadow-lg';
            popover.style.cssText = 'position: absolute; display: none; z-index: 9999; min-width: 280px;';
            popover.innerHTML = `
                <div class="card-body">
                    <div class="mb-2">
                        <small class="text-muted" data-i18n="runmode.info.version">Version:</small>
                        <div id="info-version" class="fw-semibold">-</div>
                    </div>
                    <div class="mb-2">
                        <small class="text-muted" data-i18n="runmode.info.commitHash">Commit Hash:</small>
                        <div id="info-commit-hash" class="fw-semibold">-</div>
                    </div>
                    <div class="mb-2">
                        <small class="text-muted" data-i18n="runmode.info.buildDate">Build Date:</small>
                        <div id="info-build-date" class="fw-semibold">-</div>
                    </div>
                    <div class="mb-2">
                        <small class="text-muted" data-i18n="runmode.info.targetPlatform">Target Platform:</small>
                        <div id="info-build-platform" class="fw-semibold">-</div>
                    </div>
                    <div class="mb-0">
                        <small class="text-muted" data-i18n="runmode.info.hardware">Hardware:</small>
                        <div id="info-hardware" class="fw-semibold">-</div>
                    </div>
                </div>
            `;
            document.body.appendChild(popover);

            // Apply translations to the popover content
            if (typeof applyTranslations === 'function') {
                applyTranslations();
            }
        }

        // Setup info nav link popover (need to do this every time navigation is recreated)
        setupInfoPopover();

        // Check config status from backend if on settings page
        if (currentPageId === 'settings' && typeof window.configManager !== 'undefined') {
            // Config manager will handle showing restart indicator via status polling
        } else {
            // For other pages, check backend status once
            fetch('/api/config/status')
                .then(response => response.ok ? response.json() : null)
                .then(status => {
                    if (status && status.restartRequired) {
                        const indicator = document.getElementById('restart-required-indicator');
                        if (indicator) {
                            indicator.style.display = 'inline-block';
                        }
                    }
                })
                .catch(error => console.error('Failed to check config status:', error));
        }

        // Apply translations when i18n loads
        if (typeof i18nLoaded !== 'undefined' && i18nLoaded && typeof applyTranslations === 'function') {
            applyTranslations();
        } else {
            window.addEventListener('i18nLoaded', function () {
                if (typeof applyTranslations === 'function') {
                    applyTranslations();
                }
            }, { once: true });
        }

        // Also listen for language changes
        window.addEventListener('i18nLanguageChanged', function () {
            if (typeof applyTranslations === 'function') {
                applyTranslations();
            }
        });

        // Initialize navigation icons
        if (typeof IconHelper !== 'undefined') {
            IconHelper.loadIcon('fa-circle-check').then(svg => {
                const successIcon = document.getElementById('nav-success-icon');
                if (successIcon && svg) {
                    successIcon.innerHTML = svg;
                }
            });
            IconHelper.loadIcon('fa-circle-xmark').then(svg => {
                const errorIcon = document.getElementById('nav-error-icon');
                if (errorIcon && svg) {
                    errorIcon.innerHTML = svg;
                }
            });
        }
    }
}

// Track popover visibility state
let popoverVisible = false;

// Function to set up info popover event handlers
function setupInfoPopover() {
    const infoLink = document.getElementById('info-nav-link');
    const popover = document.getElementById('info-popover');

    if (infoLink && popover) {
        // Remove any existing click listeners by cloning the element
        const newInfoLink = infoLink.cloneNode(true);
        infoLink.parentNode.replaceChild(newInfoLink, infoLink);

        newInfoLink.addEventListener('click', function (e) {
            e.preventDefault();

            if (popoverVisible) {
                popover.style.display = 'none';
                popoverVisible = false;
            } else {
                // Position popover below the nav link
                const rect = newInfoLink.getBoundingClientRect();
                popover.style.left = rect.left + 'px';
                popover.style.top = (rect.bottom + 5) + 'px';
                popover.style.display = 'block';
                popoverVisible = true;

                // Fetch and populate system info
                fetch('/api/system/info')
                    .then(response => response.json())
                    .then(data => {
                        document.getElementById('info-version').textContent = data.version || '-';
                        document.getElementById('info-commit-hash').textContent = data.commitHash || '-';
                        document.getElementById('info-build-date').textContent = data.buildTime || '-';
                        document.getElementById('info-build-platform').textContent = data.buildPlatform || '-';
                        document.getElementById('info-hardware').textContent = data.hardware || '-';
                    })
                    .catch(error => console.error('Failed to fetch system info:', error));
            }
        });
    }
}

// Close popover when clicking outside (only set up once)
document.addEventListener('click', function (e) {
    const popover = document.getElementById('info-popover');
    const infoLink = document.getElementById('info-nav-link');

    if (popoverVisible && popover && infoLink && !popover.contains(e.target) && e.target !== infoLink) {
        popover.style.display = 'none';
        popoverVisible = false;
    }
});