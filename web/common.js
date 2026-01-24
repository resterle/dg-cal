// Common JavaScript functions shared across pages

// Language switcher
function changeLanguage(lang) {
    const url = new URL(window.location.href);
    url.searchParams.set('lang', lang);
    window.location.href = url.toString();
}

// Mobile menu toggle
function toggleMobileMenu() {
    document.querySelector('.nav-links').classList.toggle('mobile-open');
    document.querySelector('.nav-actions').classList.toggle('mobile-open');
}

// Filter panel toggle (requires data-show and data-hide attributes on button)
function toggleFilters() {
    const btn = document.querySelector('.filters-toggle');
    const row = document.querySelector('.filters-row');
    if (!btn || !row) return;

    btn.classList.toggle('active');
    row.classList.toggle('mobile-open');

    const textSpan = btn.querySelector('span:first-child');
    if (textSpan) {
        const showText = btn.dataset.showText || 'Show Filters';
        const hideText = btn.dataset.hideText || 'Hide Filters';
        textSpan.textContent = row.classList.contains('mobile-open') ? hideText : showText;
    }
}
