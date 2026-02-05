// Icon helper functions for SVG icons
const IconHelper = {
    // Cache for loaded SVG content
    cache: new Map(),

    // Load an SVG icon and return its content
    async loadIcon(name) {
        if (this.cache.has(name)) {
            return this.cache.get(name);
        }

        try {
            // Strip 'fa-' prefix from the filename if present
            const filename = name.startsWith('fa-') ? name.substring(3) : name;
            const response = await fetch(`/images/icons/${filename}.svg?v=${Date.now()}`);
            if (!response.ok) {
                console.error(`Failed to load icon: ${name}`);
                return null;
            }
            let svgText = await response.text();

            // Remove fill and stroke attributes to allow CSS to control color
            svgText = svgText.replace(/\s(fill|stroke)="[^"]*"/g, '');

            this.cache.set(name, svgText);
            return svgText;
        } catch (error) {
            console.error(`Error loading icon ${name}:`, error);
            return null;
        }
    },

    // Create an icon element with the given name and optional classes
    async createIcon(name, classes = '') {
        const svgContent = await this.loadIcon(name);
        if (!svgContent) {
            return document.createElement('span');
        }

        const wrapper = document.createElement('span');
        wrapper.className = `icon ${classes}`;
        wrapper.innerHTML = svgContent;
        return wrapper;
    },

    // Replace a Font Awesome icon with an SVG icon
    async replaceIcon(element, iconName) {
        const svgContent = await this.loadIcon(iconName);
        if (!svgContent) {
            return;
        }

        // Remove only Font Awesome classes, keep other classes
        const classesToRemove = ['fa-solid', 'fa-regular', 'fas', 'far', 'fab'];
        classesToRemove.forEach(cls => element.classList.remove(cls));
        element.classList.remove(...Array.from(element.classList).filter(c => c.startsWith('fa-')));

        // Add icon class if not present
        if (!element.classList.contains('icon')) {
            element.classList.add('icon');
        }

        // Set the SVG content
        element.innerHTML = svgContent;

        // If element has inline styles for positioning, ensure the SVG inherits them
        if (element.style.position === 'absolute') {
            const svg = element.querySelector('svg');
            if (svg) {
                svg.style.display = 'block';
            }
        }
    },

    // Preload common icons (optional - can be called to eagerly load specific icons)
    async preloadIcons(iconNames) {
        const promises = iconNames.map(name => this.loadIcon(name));
        await Promise.all(promises);
    }
};
