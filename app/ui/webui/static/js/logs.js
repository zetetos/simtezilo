// logs.js - WebSocket subscription for live log statistics updates

(function () {
    'use strict';

    // Update log statistics in the UI
    function updateLogStats(data) {
        if (!data || !data.stats) {
            return;
        }

        const stats = data.stats;
        const totalCount = data.totalCount || 0;
        const totalPagesFromStats = data.totalPages || 1;

        // Update global variables if they exist in window scope
        if (typeof window.totalLogs !== 'undefined') {
            window.totalLogs = totalCount;
        }
        if (typeof window.totalPages !== 'undefined') {
            window.totalPages = totalPagesFromStats;
        }

        // Update total entries
        const totalElement = document.getElementById('total-entries');
        if (totalElement) {
            totalElement.textContent = totalCount;
        }

        // Update individual level counts
        const errorElement = document.getElementById('error-count');
        if (errorElement) {
            errorElement.textContent = stats.error || 0;
        }

        const warningElement = document.getElementById('warning-count');
        if (warningElement) {
            warningElement.textContent = stats.warn || 0;
        }

        const infoElement = document.getElementById('info-count');
        if (infoElement) {
            infoElement.textContent = stats.info || 0;
        }

        const debugElement = document.getElementById('debug-count');
        if (debugElement) {
            debugElement.textContent = stats.debug || 0;
        }

        const traceElement = document.getElementById('trace-count');
        if (traceElement) {
            traceElement.textContent = stats.trace || 0;
        }

        // Update pagination info
        updatePaginationDisplay(totalCount, totalPagesFromStats);
    }

    // Update pagination display elements
    function updatePaginationDisplay(totalCount, totalPagesValue) {
        // Helper function to get translation or fallback
        const t = window.t || function (key) { return null; };

        // Update "Showing x-y of z" text
        const paginationInfo = document.getElementById('pagination-info');
        if (paginationInfo) {
            const currentPage = window.currentPage || 1;
            const pageSize = window.pageSize || 100;
            const start = (currentPage - 1) * pageSize + 1;
            const end = Math.min(currentPage * pageSize, totalCount);

            const template = t('runmode.logs.showing');
            if (template) {
                paginationInfo.textContent = template.replace('{start}', start).replace('{end}', end).replace('{total}', totalCount);
            } else {
                paginationInfo.textContent = `Showing ${start}-${end} of ${totalCount} logs`;
            }
        }

        // Update "Page N of M" text
        const pageInfo = document.getElementById('page-info');
        if (pageInfo) {
            const currentPage = window.currentPage || 1;
            const template = t('runmode.logs.pageinfo');
            if (template) {
                pageInfo.textContent = template.replace('{current}', currentPage).replace('{total}', totalPagesValue);
            } else {
                pageInfo.textContent = `Page ${currentPage} of ${totalPagesValue}`;
            }
        }

        // Update pagination button states
        const prevBtn = document.getElementById('prev-page');
        const nextBtn = document.getElementById('next-page');
        const currentPage = window.currentPage || 1;

        if (prevBtn) {
            prevBtn.disabled = currentPage <= 1;
        }
        if (nextBtn) {
            nextBtn.disabled = currentPage >= totalPagesValue;
        }
    }

    // Initialize when SharedWebSocket is available
    function initLogStats() {
        if (window.SharedWebSocket) {
            // Subscribe to logStats messages
            window.SharedWebSocket.subscribe('logStats', updateLogStats);

            console.log('Log stats subscribed to SharedWebSocket');
        } else {
            console.error('SharedWebSocket not available');
        }
    }

    // Initialize when page loads
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', initLogStats);
    } else {
        initLogStats();
    }
})();
