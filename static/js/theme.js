(function() {
    const STORAGE_KEY = 'kv-theme';
    const DEFAULT_THEME = 'dark';

    function getTheme() {
        return localStorage.getItem(STORAGE_KEY) || DEFAULT_THEME;
    }

    function setTheme(theme) {
        localStorage.setItem(STORAGE_KEY, theme);
        document.documentElement.setAttribute('data-theme', theme);
        updateToggleIcon(theme);
    }

    function updateToggleIcon(theme) {
        const toggle = document.getElementById('theme-toggle');
        if (!toggle) return;

        if (theme === 'dark') {
            toggle.innerHTML = '<i class="bi bi-sun-fill"></i>';
            toggle.setAttribute('aria-label', 'Switch to light mode');
        } else {
            toggle.innerHTML = '<i class="bi bi-moon-fill"></i>';
            toggle.setAttribute('aria-label', 'Switch to dark mode');
        }
    }

    function toggleTheme() {
        const current = getTheme();
        const next = current === 'dark' ? 'light' : 'dark';
        setTheme(next);
    }

    // Initialize theme on load
    const initialTheme = getTheme();
    document.documentElement.setAttribute('data-theme', initialTheme);

    // Wait for DOM to be ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', function() {
            updateToggleIcon(initialTheme);
            const toggle = document.getElementById('theme-toggle');
            if (toggle) {
                toggle.addEventListener('click', toggleTheme);
            }
        });
    } else {
        updateToggleIcon(initialTheme);
        const toggle = document.getElementById('theme-toggle');
        if (toggle) {
            toggle.addEventListener('click', toggleTheme);
        }
    }
})();
