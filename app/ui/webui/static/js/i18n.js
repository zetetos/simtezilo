// i18n.js - Internationalization support for Simtezilo Web UI

// Global i18n object to store translations
let i18n = null;
let i18nLoaded = false;

// Load i18n translations from the API
async function loadI18n(lang = null) {
    try {
        const url = lang ? `/api/i18n?lang=${lang}` : '/api/i18n';
        const response = await fetch(url);
        if (!response.ok) {
            throw new Error('Failed to load translations');
        }
        i18n = await response.json();
        const wasLoaded = i18nLoaded;
        i18nLoaded = true;
        applyTranslations();

        // Dispatch custom event to notify that translations are loaded
        if (!wasLoaded) {
            window.dispatchEvent(new CustomEvent('i18nLoaded'));
        } else {
            // Language changed, dispatch language changed event
            window.dispatchEvent(new CustomEvent('i18nLanguageChanged'));
        }
        return true;
    } catch (error) {
        console.error('Error loading i18n:', error);
        i18nLoaded = true; // Set to true even on error to not block UI
        // Continue with English text as fallback
        return false;
    }
}

// Helper function to get translation by key
function t(key) {
    if (!i18n) return '';
    return i18n[key] || '';
}

// Apply translations to the page
function applyTranslations() {
    if (!i18n) return;

    // Find all elements with data-i18n attribute
    document.querySelectorAll('[data-i18n]').forEach(element => {
        const key = element.getAttribute('data-i18n');
        const translation = t(key);

        if (translation) {
            // Check if element has data-i18n-attr to translate an attribute instead of text content
            const attr = element.getAttribute('data-i18n-attr');
            if (attr) {
                element.setAttribute(attr, translation);
            } else {
                element.textContent = translation;
            }
        }
    });

    // Find all elements with data-i18n-html attribute (for HTML content)
    document.querySelectorAll('[data-i18n-html]').forEach(element => {
        const key = element.getAttribute('data-i18n-html');
        const translation = t(key);

        if (translation) {
            element.innerHTML = translation;
        }
    });

    // Find all elements with data-i18n-placeholder attribute
    document.querySelectorAll('[data-i18n-placeholder]').forEach(element => {
        const key = element.getAttribute('data-i18n-placeholder');
        const translation = t(key);

        if (translation) {
            element.placeholder = translation;
        }
    });

    // Find all elements with data-i18n-tooltip attribute
    document.querySelectorAll('[data-i18n-tooltip]').forEach(element => {
        const key = element.getAttribute('data-i18n-tooltip');
        const translation = t(key);

        if (translation) {
            element.setAttribute('data-tooltip', translation);
        }
    });
}

// Initialize i18n when DOM is loaded
document.addEventListener('DOMContentLoaded', function () {
    loadI18n();
});
