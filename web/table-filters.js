// Reusable table filtering and sorting functionality
class TableFilter {
    constructor(tableSelector, options = {}) {
        this.table = document.querySelector(tableSelector);
        this.tbody = this.table.querySelector('tbody');
        // Exclude divider rows from filtering and sorting
        this.rows = Array.from(this.tbody.querySelectorAll('tr:not(.registration-divider):not(.tournament-year-divider)'));
        this.filters = {};
        this.sortColumn = options.defaultSort || null;
        this.sortDirection = 'asc';

        this.init();
    }

    init() {
        // Store original order
        this.rows.forEach((row, index) => {
            row.dataset.originalIndex = index;
        });
    }

    addFilter(name, filterFunction) {
        this.filters[name] = filterFunction;
    }

    setFilterValue(name, value) {
        if (this.filters[name]) {
            this.filters[name].value = value;
            this.applyFilters();
        }
    }

    applyFilters() {
        this.rows.forEach(row => {
            let visible = true;

            for (let name in this.filters) {
                const filter = this.filters[name];
                if (filter.value && filter.value !== 'all') {
                    if (!filter.fn(row, filter.value)) {
                        visible = false;
                        break;
                    }
                }
            }

            row.style.display = visible ? '' : 'none';
        });

        // Call post-filter callback if provided
        if (this.onFilterChange) {
            this.onFilterChange();
        }
    }

    sort(columnIndex, direction) {
        this.sortColumn = columnIndex;
        this.sortDirection = direction || this.sortDirection;

        const sortedRows = [...this.rows].sort((a, b) => {
            const aCell = a.cells[columnIndex];
            const bCell = b.cells[columnIndex];

            // Try to get date from date-badge if it exists
            const aDateBadge = aCell.querySelector('.date-badge');
            const bDateBadge = bCell.querySelector('.date-badge');

            if (aDateBadge && bDateBadge) {
                // Get the date from data attribute or reconstruct from badge
                const aDate = new Date(aCell.dataset.sortValue || this.getDateFromBadge(aCell));
                const bDate = new Date(bCell.dataset.sortValue || this.getDateFromBadge(bCell));
                return this.sortDirection === 'asc' ? aDate - bDate : bDate - aDate;
            }

            // Otherwise compare text content
            const aText = aCell.textContent.trim();
            const bText = bCell.textContent.trim();

            if (this.sortDirection === 'asc') {
                return aText.localeCompare(bText);
            } else {
                return bText.localeCompare(aText);
            }
        });

        // Remove all existing dividers
        const existingDividers = this.tbody.querySelectorAll('.tournament-year-divider, .registration-divider');
        existingDividers.forEach(div => div.remove());

        // Group sorted rows by year and rebuild with dividers
        const yearGroups = new Map();
        sortedRows.forEach(row => {
            const dateStr = row.dataset.date || row.dataset.sortValue;
            if (dateStr) {
                const year = new Date(dateStr).getFullYear();
                if (!yearGroups.has(year)) {
                    yearGroups.set(year, []);
                }
                yearGroups.get(year).push(row);
            }
        });

        // Get years in sorted order
        const years = Array.from(yearGroups.keys()).sort((a, b) => {
            return this.sortDirection === 'asc' ? a - b : b - a;
        });

        // Rebuild table with dividers
        years.forEach((year, index) => {
            // When sorting descending, add divider before every year (including first)
            // When sorting ascending, add divider before each year except the first
            if (this.sortDirection === 'desc' || index > 0) {
                const divider = document.createElement('tr');
                divider.className = 'tournament-year-divider';
                divider.innerHTML = `
                    <td colspan="3">
                        <div class="divider-line"></div>
                        <div class="divider-text">${year}</div>
                    </td>
                `;
                this.tbody.appendChild(divider);
            }

            // Add all tournaments for this year
            yearGroups.get(year).forEach(row => {
                this.tbody.appendChild(row);
            });
        });

        // Call post-filter callback if provided
        if (this.onFilterChange) {
            this.onFilterChange();
        }
    }

    getDateFromBadge(cell) {
        // This is a fallback - ideally we'd have data attributes
        return cell.dataset.sortValue || '';
    }

    toggleSort(columnIndex) {
        if (this.sortColumn === columnIndex) {
            this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
        } else {
            this.sortDirection = 'asc';
        }
        this.sort(columnIndex, this.sortDirection);
    }

    reset() {
        // Clear all filters
        for (let name in this.filters) {
            this.filters[name].value = null;
        }

        // Reset sort to original order
        const sortedRows = [...this.rows].sort((a, b) => {
            return parseInt(a.dataset.originalIndex) - parseInt(b.dataset.originalIndex);
        });

        sortedRows.forEach(row => {
            row.style.display = '';
            this.tbody.appendChild(row);
        });

        // Call post-filter callback if provided
        if (this.onFilterChange) {
            this.onFilterChange();
        }
    }
}

// Helper function to extract unique values from table
function extractUniqueValues(tableFilter, columnIndex, selector) {
    const values = new Set();
    tableFilter.rows.forEach(row => {
        const cell = row.cells[columnIndex];
        if (selector) {
            const elements = cell.querySelectorAll(selector);
            elements.forEach(el => values.add(el.textContent.trim()));
        } else {
            const text = cell.textContent.trim();
            if (text) values.add(text);
        }
    });
    return Array.from(values).sort();
}

// Helper to populate select dropdown
function populateSelect(selectElement, values, includeAll = true) {
    selectElement.innerHTML = '';

    if (includeAll) {
        const allOption = document.createElement('option');
        allOption.value = 'all';
        allOption.textContent = 'All';
        selectElement.appendChild(allOption);
    }

    values.forEach(value => {
        const option = document.createElement('option');
        option.value = value;
        option.textContent = value;
        selectElement.appendChild(option);
    });
}
