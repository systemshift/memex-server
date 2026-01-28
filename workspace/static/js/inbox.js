/**
 * Memex Inbox - Email list management
 */

(function() {
    'use strict';

    // State
    let emails = [];
    let currentPage = 1;
    let pageSize = 20;
    let totalEmails = 0;
    let searchQuery = '';
    let selectedEmails = new Set();
    let loading = false;

    // DOM elements
    const emailList = document.getElementById('email-list');
    const loadingEl = document.getElementById('loading');
    const paginationEl = document.getElementById('pagination');
    const paginationInfo = document.getElementById('pagination-info');
    const prevBtn = document.getElementById('prev-btn');
    const nextBtn = document.getElementById('next-btn');
    const searchInput = document.getElementById('search-input');
    const refreshBtn = document.getElementById('refresh-btn');
    const inboxCount = document.getElementById('inbox-count');
    const statusMessage = document.getElementById('status-message');

    // Initialize
    document.addEventListener('DOMContentLoaded', init);

    function init() {
        // Load emails
        loadEmails();

        // Event listeners
        searchInput.addEventListener('input', debounce(handleSearch, 300));
        refreshBtn.addEventListener('click', () => loadEmails(true));
        prevBtn.addEventListener('click', () => changePage(-1));
        nextBtn.addEventListener('click', () => changePage(1));
    }

    // Fetch emails from API
    async function loadEmails(forceRefresh = false) {
        if (loading) return;
        loading = true;

        showLoading(true);

        try {
            const params = new URLSearchParams({
                page: currentPage,
                limit: pageSize,
                q: searchQuery
            });

            const response = await fetch(`/api/emails?${params}`);
            const data = await response.json();

            if (data.error) {
                showStatus(data.error, 'error');
                return;
            }

            emails = data.emails || [];
            totalEmails = data.total || emails.length;

            renderEmails();
            updatePagination();
            updateInboxCount();

            if (forceRefresh) {
                showStatus('Refreshed');
            }

        } catch (error) {
            console.error('Error loading emails:', error);
            showStatus('Failed to load emails', 'error');
            renderEmpty('Could not load emails');
        } finally {
            loading = false;
            showLoading(false);
        }
    }

    // Render email list
    function renderEmails() {
        if (emails.length === 0) {
            renderEmpty('No emails found');
            return;
        }

        emailList.innerHTML = emails.map(email => renderEmailItem(email)).join('');

        // Attach click handlers
        emailList.querySelectorAll('.email-item').forEach(item => {
            item.addEventListener('click', (e) => {
                if (e.target.type === 'checkbox') return;
                window.location.href = `/inbox/${item.dataset.id}`;
            });
        });
    }

    // Render single email item
    function renderEmailItem(email) {
        const unreadClass = email.unread ? 'unread' : '';
        const selectedClass = selectedEmails.has(email.id) ? 'selected' : '';
        const date = formatDate(email.date);
        const anchors = email.anchors || [];

        // Count anchors by type
        const anchorCounts = {};
        anchors.forEach(a => {
            anchorCounts[a.type] = (anchorCounts[a.type] || 0) + 1;
        });

        const anchorBadges = Object.entries(anchorCounts)
            .slice(0, 3)
            .map(([type, count]) => `
                <span class="anchor-badge ${type}">
                    ${count} ${formatAnchorType(type)}
                </span>
            `).join('');

        return `
            <div class="email-item ${unreadClass} ${selectedClass}" data-id="${email.id}">
                <input type="checkbox" class="email-checkbox"
                    ${selectedEmails.has(email.id) ? 'checked' : ''}
                    onclick="toggleSelection('${email.id}', event)">
                <div class="email-sender">
                    <div class="sender-name">${escapeHtml(email.from_name || email.from_email)}</div>
                </div>
                <div class="email-content">
                    <div class="email-subject-row">
                        <span class="email-subject">${escapeHtml(email.subject)}</span>
                        ${email.thread_count > 1 ? `
                            <span class="thread-indicator">
                                <span class="thread-count">${email.thread_count}</span>
                            </span>
                        ` : ''}
                    </div>
                    <div class="email-preview">${escapeHtml(email.body_preview || '')}</div>
                </div>
                <div class="email-meta">
                    <div class="anchor-badges">
                        ${anchorBadges}
                        ${anchors.length > 3 ? `<span class="anchor-badge">+${anchors.length - 3}</span>` : ''}
                    </div>
                    <div class="email-date">${date}</div>
                </div>
            </div>
        `;
    }

    // Render empty state
    function renderEmpty(message) {
        emailList.innerHTML = `
            <div class="empty-state">
                <div class="empty-icon">&#128232;</div>
                <div class="empty-title">${message}</div>
                <div class="empty-text">
                    ${searchQuery
                        ? 'Try a different search term'
                        : 'Configure email polling to start ingesting emails'}
                </div>
            </div>
        `;
    }

    // Update pagination controls
    function updatePagination() {
        const totalPages = Math.ceil(totalEmails / pageSize);
        const start = (currentPage - 1) * pageSize + 1;
        const end = Math.min(currentPage * pageSize, totalEmails);

        if (totalEmails === 0) {
            paginationEl.style.display = 'none';
            return;
        }

        paginationEl.style.display = 'flex';
        paginationInfo.textContent = `${start}-${end} of ${totalEmails}`;
        prevBtn.disabled = currentPage <= 1;
        nextBtn.disabled = currentPage >= totalPages;
    }

    // Update inbox count badge
    function updateInboxCount() {
        inboxCount.textContent = totalEmails;
    }

    // Change page
    function changePage(delta) {
        const totalPages = Math.ceil(totalEmails / pageSize);
        const newPage = currentPage + delta;

        if (newPage >= 1 && newPage <= totalPages) {
            currentPage = newPage;
            loadEmails();
        }
    }

    // Handle search
    function handleSearch() {
        searchQuery = searchInput.value.trim();
        currentPage = 1;
        loadEmails();
    }

    // Toggle email selection
    window.toggleSelection = function(emailId, event) {
        event.stopPropagation();

        if (selectedEmails.has(emailId)) {
            selectedEmails.delete(emailId);
        } else {
            selectedEmails.add(emailId);
        }

        renderEmails();
        updateBulkActions();
    };

    // Update bulk action visibility
    function updateBulkActions() {
        const bulkActions = document.querySelector('.bulk-actions');
        if (bulkActions) {
            if (selectedEmails.size > 0) {
                bulkActions.classList.add('visible');
                bulkActions.querySelector('.bulk-count').textContent =
                    `${selectedEmails.size} selected`;
            } else {
                bulkActions.classList.remove('visible');
            }
        }
    }

    // Show/hide loading
    function showLoading(show) {
        loadingEl.style.display = show ? 'flex' : 'none';
    }

    // Show status message
    function showStatus(message, type = 'success') {
        statusMessage.textContent = message;
        statusMessage.className = `status-message visible ${type === 'error' ? 'error' : ''}`;

        setTimeout(() => {
            statusMessage.classList.remove('visible');
        }, 3000);
    }

    // Format date
    function formatDate(dateStr) {
        if (!dateStr) return '';

        const date = new Date(dateStr);
        const now = new Date();
        const diff = now - date;

        // Today: show time
        if (diff < 86400000 && date.getDate() === now.getDate()) {
            return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
        }

        // This week: show day name
        if (diff < 604800000) {
            return date.toLocaleDateString([], { weekday: 'short' });
        }

        // This year: show month/day
        if (date.getFullYear() === now.getFullYear()) {
            return date.toLocaleDateString([], { month: 'short', day: 'numeric' });
        }

        // Older: show full date
        return date.toLocaleDateString([], {
            year: 'numeric',
            month: 'short',
            day: 'numeric'
        });
    }

    // Format anchor type for display
    function formatAnchorType(type) {
        const labels = {
            person: 'person',
            organization: 'org',
            action_item: 'task',
            decision: 'decision',
            date: 'date',
            topic: 'topic'
        };
        return labels[type] || type;
    }

    // Escape HTML
    function escapeHtml(str) {
        if (!str) return '';
        const div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    }

    // Debounce helper
    function debounce(fn, delay) {
        let timeout;
        return function(...args) {
            clearTimeout(timeout);
            timeout = setTimeout(() => fn.apply(this, args), delay);
        };
    }

})();
