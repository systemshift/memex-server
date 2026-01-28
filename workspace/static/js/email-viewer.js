/**
 * Memex Email Viewer
 * Renders email with inline anchor highlights
 */

(function() {
    'use strict';

    let currentEmail = null;
    let anchors = [];
    let selectedAnchorId = null;

    // Initialize email viewer
    window.initEmailViewer = async function(emailId) {
        if (!emailId) {
            showError('No email ID provided');
            return;
        }

        try {
            await loadEmail(emailId);
            setupEventListeners();
        } catch (error) {
            console.error('Error initializing email viewer:', error);
            showError('Failed to load email');
        }
    };

    // Load email from API
    async function loadEmail(emailId) {
        try {
            const response = await fetch(`/api/emails/${emailId}`);
            const data = await response.json();

            if (data.error) {
                showError(data.error);
                return;
            }

            currentEmail = data.email;
            anchors = data.anchors || [];

            renderEmail();
            renderAnchors();

            // Dispatch event for graph panel
            window.dispatchEvent(new CustomEvent('email-loaded', {
                detail: { email: currentEmail, anchors: anchors }
            }));

        } catch (error) {
            console.error('Error loading email:', error);
            showError('Failed to load email');
        }
    }

    // Render email header and body
    function renderEmail() {
        if (!currentEmail) return;

        // Update header
        document.getElementById('email-subject').textContent = currentEmail.subject || '(No subject)';
        document.getElementById('email-from').textContent = formatAddress(currentEmail.from);
        document.getElementById('email-to').textContent = formatAddresses(currentEmail.to);
        document.getElementById('email-date').textContent = formatDate(currentEmail.date);

        // Render body with anchor highlights
        const body = currentEmail.body || currentEmail.body_preview || '';
        document.getElementById('email-body').innerHTML = renderBodyWithAnchors(body);

        // Attach anchor click handlers
        document.querySelectorAll('.anchor-highlight').forEach(el => {
            el.addEventListener('click', () => selectAnchor(el.dataset.anchorId));
            el.addEventListener('mouseenter', (e) => showAnchorPreview(el, e));
            el.addEventListener('mouseleave', hideAnchorPreview);
        });
    }

    // Render body with anchor highlights
    function renderBodyWithAnchors(body) {
        if (!body) return '<p>No content</p>';

        // Sort anchors by position (descending) to insert from end
        const bodyAnchors = anchors
            .filter(a => a.properties?.zone !== 'subject')
            .sort((a, b) => b.start - a.start);

        let html = escapeHtml(body);

        // Insert anchor spans (from end to preserve offsets)
        bodyAnchors.forEach(anchor => {
            const start = anchor.start;
            const end = anchor.end;

            if (start >= 0 && end > start && end <= html.length) {
                const before = html.substring(0, start);
                const text = html.substring(start, end);
                const after = html.substring(end);

                html = before +
                    `<span class="anchor-highlight ${anchor.type}" ` +
                    `data-anchor-id="${anchor.id}" ` +
                    `data-anchor-type="${anchor.type}">` +
                    text +
                    '</span>' +
                    after;
            }
        });

        // Convert line breaks to paragraphs
        html = html.split(/\n\n+/).map(p => `<p>${p.trim()}</p>`).join('');
        html = html.replace(/\n/g, '<br>');

        return html;
    }

    // Render anchor list in sidebar
    function renderAnchors() {
        const container = document.getElementById('anchor-list');
        const countEl = document.getElementById('anchor-count');

        countEl.textContent = `${anchors.length} anchor${anchors.length !== 1 ? 's' : ''} found`;

        if (anchors.length === 0) {
            container.innerHTML = `
                <div class="no-selection">
                    <div class="no-selection-icon">&#128269;</div>
                    <div>No anchors extracted yet</div>
                </div>
            `;
            return;
        }

        container.innerHTML = anchors.map(anchor => `
            <div class="anchor-item ${selectedAnchorId === anchor.id ? 'selected' : ''}"
                 data-anchor-id="${anchor.id}">
                <span class="anchor-type ${anchor.type}">${formatAnchorType(anchor.type)}</span>
                <div class="anchor-text">${escapeHtml(anchor.text)}</div>
                ${anchor.matched_patterns?.length ? `
                    <div class="anchor-patterns">
                        Patterns: ${anchor.matched_patterns.join(', ')}
                    </div>
                ` : ''}
            </div>
        `).join('');

        // Attach click handlers
        container.querySelectorAll('.anchor-item').forEach(el => {
            el.addEventListener('click', () => selectAnchor(el.dataset.anchorId));
        });
    }

    // Select an anchor
    function selectAnchor(anchorId) {
        selectedAnchorId = anchorId;

        // Update highlights
        document.querySelectorAll('.anchor-highlight').forEach(el => {
            el.classList.toggle('selected', el.dataset.anchorId === anchorId);
        });

        // Update anchor list
        document.querySelectorAll('.anchor-item').forEach(el => {
            el.classList.toggle('selected', el.dataset.anchorId === anchorId);
        });

        // Scroll to anchor in body
        const highlight = document.querySelector(`.anchor-highlight[data-anchor-id="${anchorId}"]`);
        if (highlight) {
            highlight.scrollIntoView({ behavior: 'smooth', block: 'center' });
        }

        // Dispatch event for graph panel
        const anchor = anchors.find(a => a.id === anchorId);
        if (anchor) {
            window.dispatchEvent(new CustomEvent('anchor-selected', {
                detail: { anchor: anchor }
            }));
        }
    }

    // Show anchor preview on hover
    function showAnchorPreview(element, event) {
        const anchorId = element.dataset.anchorId;
        const anchor = anchors.find(a => a.id === anchorId);
        if (!anchor) return;

        // Remove existing preview
        hideAnchorPreview();

        // Create preview element
        const preview = document.createElement('div');
        preview.className = 'anchor-preview';
        preview.innerHTML = `
            <div class="anchor-preview-header">
                <span class="anchor-preview-type ${anchor.type}">${formatAnchorType(anchor.type)}</span>
            </div>
            <div class="anchor-preview-text">${escapeHtml(anchor.text)}</div>
            ${anchor.matched_patterns?.length ? `
                <div class="anchor-preview-patterns">
                    Matched: ${anchor.matched_patterns.join(', ')}
                </div>
            ` : ''}
            <div class="anchor-preview-link">Click to see connections</div>
        `;

        // Position preview
        const rect = element.getBoundingClientRect();
        preview.style.left = `${rect.left}px`;
        preview.style.top = `${rect.bottom + 8}px`;

        document.body.appendChild(preview);

        // Animate in
        requestAnimationFrame(() => preview.classList.add('visible'));
    }

    // Hide anchor preview
    function hideAnchorPreview() {
        const existing = document.querySelector('.anchor-preview');
        if (existing) {
            existing.remove();
        }
    }

    // Setup event listeners
    function setupEventListeners() {
        // Re-extract button
        const reprocessBtn = document.getElementById('reprocess-btn');
        if (reprocessBtn) {
            reprocessBtn.addEventListener('click', reprocessEmail);
        }
    }

    // Re-process email for anchor extraction
    async function reprocessEmail() {
        if (!currentEmail) return;

        const btn = document.getElementById('reprocess-btn');
        btn.disabled = true;
        btn.textContent = 'Extracting...';

        try {
            const response = await fetch(`/api/emails/${currentEmail.id}/reprocess`, {
                method: 'POST'
            });
            const data = await response.json();

            if (data.error) {
                alert('Error: ' + data.error);
            } else {
                // Reload email to get new anchors
                await loadEmail(currentEmail.id);
                alert(`Extracted ${data.anchors_extracted} anchors`);
            }
        } catch (error) {
            console.error('Reprocess error:', error);
            alert('Failed to reprocess email');
        } finally {
            btn.disabled = false;
            btn.textContent = 'Re-extract';
        }
    }

    // Show error message
    function showError(message) {
        document.getElementById('email-body').innerHTML = `
            <div class="no-selection">
                <div class="no-selection-icon">&#128683;</div>
                <div>${escapeHtml(message)}</div>
            </div>
        `;
    }

    // Format email address
    function formatAddress(addr) {
        if (!addr) return '-';
        if (typeof addr === 'string') return addr;
        if (addr.name) return `${addr.name} <${addr.address}>`;
        return addr.address || '-';
    }

    // Format address list
    function formatAddresses(addrs) {
        if (!addrs || !Array.isArray(addrs)) return '-';
        return addrs.map(formatAddress).join(', ') || '-';
    }

    // Format date
    function formatDate(dateStr) {
        if (!dateStr) return '-';
        const date = new Date(dateStr);
        return date.toLocaleDateString(undefined, {
            weekday: 'short',
            year: 'numeric',
            month: 'short',
            day: 'numeric',
            hour: '2-digit',
            minute: '2-digit'
        });
    }

    // Format anchor type
    function formatAnchorType(type) {
        const labels = {
            person: 'Person',
            organization: 'Organization',
            action_item: 'Action Item',
            decision: 'Decision',
            date: 'Date',
            topic: 'Topic'
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

})();
