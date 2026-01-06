/**
 * Memex Workspace - Multi-User & Notification System
 */

let currentUserId = 'alex';
let currentUser = null;
let notificationEventSource = null;
let currentWorkItemId = null;
let selectedHandoffTarget = null;

// User data (matches backend)
const USERS = {
    alex: { id: 'alex', name: 'Alex', role: 'sales', title: 'Sales Rep' },
    jordan: { id: 'jordan', name: 'Jordan', role: 'cs', title: 'Customer Success Manager' },
    sam: { id: 'sam', name: 'Sam', role: 'engineering', title: 'Solutions Engineer' },
    morgan: { id: 'morgan', name: 'Morgan', role: 'manager', title: 'VP Operations' }
};

/**
 * Initialize multi-user functionality
 */
function initMultiUser() {
    currentUser = USERS[currentUserId];

    // Set up user selector
    setupUserSelector();

    // Set up notification panel
    setupNotificationPanel();

    // Set up handoff functionality
    setupHandoffUI();

    // Start notification stream
    startNotificationStream();

    // Update UI for current user
    updateUserUI();

    // Check for pending notifications
    fetchNotifications();
}

/**
 * Set up user selector dropdown
 */
function setupUserSelector() {
    const selector = document.getElementById('user-selector');
    const dropdown = document.getElementById('user-dropdown');

    // Toggle dropdown
    selector.addEventListener('click', (e) => {
        e.stopPropagation();
        dropdown.classList.toggle('open');
        // Close notification panel
        document.getElementById('notification-panel').classList.remove('open');
    });

    // Handle user selection
    dropdown.querySelectorAll('.user-option').forEach(option => {
        option.addEventListener('click', async (e) => {
            e.stopPropagation();
            const userId = option.dataset.user;
            await switchUser(userId);
            dropdown.classList.remove('open');
        });
    });

    // Close on outside click
    document.addEventListener('click', () => {
        dropdown.classList.remove('open');
    });
}

/**
 * Switch to a different user
 */
async function switchUser(userId) {
    // Stop current notification stream
    if (notificationEventSource) {
        notificationEventSource.close();
    }

    currentUserId = userId;
    currentUser = USERS[userId];

    // Create new session for user
    try {
        const resp = await fetch('/api/session', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ user_id: userId })
        });
        const data = await resp.json();
        sessionId = data.session_id;
        document.getElementById('session-info').textContent = `Session: ${sessionId}`;
    } catch (e) {
        console.error('Failed to create session:', e);
    }

    // Update UI
    updateUserUI();

    // Restart notification stream
    startNotificationStream();

    // Fetch notifications for new user
    fetchNotifications();

    // Reset content area
    showWelcome();
}

/**
 * Update UI elements for current user
 */
function updateUserUI() {
    // Update header display
    document.getElementById('current-user-name').textContent = currentUser.name;
    document.getElementById('current-user-role').textContent = currentUser.title;

    const avatar = document.getElementById('current-avatar');
    avatar.textContent = currentUser.name[0];
    avatar.className = `user-avatar ${currentUser.role}`;

    // Update role indicator
    document.getElementById('role-indicator').textContent = `Role: ${currentUser.title}`;

    // Update active state in dropdown
    document.querySelectorAll('.user-option').forEach(option => {
        option.classList.toggle('active', option.dataset.user === currentUserId);
    });

    // Show/hide dashboard link based on role
    const dashboardLink = document.getElementById('dashboard-link');
    if (currentUser.role === 'manager') {
        dashboardLink.style.display = 'block';
    } else {
        dashboardLink.style.display = 'none';
    }
}

/**
 * Set up notification panel
 */
function setupNotificationPanel() {
    const bell = document.getElementById('notification-bell');
    const panel = document.getElementById('notification-panel');

    // Toggle panel
    bell.addEventListener('click', (e) => {
        e.stopPropagation();
        panel.classList.toggle('open');
        // Close user dropdown
        document.getElementById('user-dropdown').classList.remove('open');
    });

    // Mark all read
    document.getElementById('mark-all-read').addEventListener('click', async () => {
        await markAllNotificationsRead();
    });

    // Close on outside click
    document.addEventListener('click', () => {
        panel.classList.remove('open');
    });

    panel.addEventListener('click', (e) => {
        e.stopPropagation();
    });
}

/**
 * Start SSE notification stream
 */
function startNotificationStream() {
    if (notificationEventSource) {
        notificationEventSource.close();
    }

    notificationEventSource = new EventSource(`/api/notifications/stream/${currentUserId}`);

    notificationEventSource.onmessage = (event) => {
        try {
            const data = JSON.parse(event.data);
            handleNotificationEvent(data);
        } catch (e) {
            console.warn('Failed to parse notification event:', e);
        }
    };

    notificationEventSource.onerror = (e) => {
        console.error('Notification stream error:', e);
        // Attempt to reconnect after 5 seconds
        setTimeout(() => {
            if (notificationEventSource.readyState === EventSource.CLOSED) {
                startNotificationStream();
            }
        }, 5000);
    };
}

/**
 * Handle notification event from SSE
 */
function handleNotificationEvent(data) {
    if (data.type === 'notifications') {
        // Update badge
        updateNotificationBadge(data.count);

        // Add new notifications to list
        if (data.new && data.new.length > 0) {
            data.new.forEach(notif => {
                addNotificationToList(notif, true);
            });

            // Show browser notification if supported
            if (Notification.permission === 'granted') {
                const latest = data.new[0];
                new Notification(latest.title, {
                    body: latest.message,
                    icon: '/static/img/icon.png'
                });
            }
        }
    } else if (data.type === 'heartbeat') {
        updateNotificationBadge(data.count);
    }
}

/**
 * Fetch notifications from API
 */
async function fetchNotifications() {
    try {
        const resp = await fetch(`/api/notifications/${currentUserId}?all=true`);
        const data = await resp.json();

        updateNotificationBadge(data.notifications.filter(n => !n.read).length);
        renderNotificationList(data.notifications);
    } catch (e) {
        console.error('Failed to fetch notifications:', e);
    }
}

/**
 * Update notification badge count
 */
function updateNotificationBadge(count) {
    const badge = document.getElementById('notification-count');
    badge.textContent = count;
    badge.classList.toggle('hidden', count === 0);
}

/**
 * Render full notification list
 */
function renderNotificationList(notifications) {
    const list = document.getElementById('notification-list');

    if (notifications.length === 0) {
        list.innerHTML = '<div class="notification-empty">No notifications</div>';
        return;
    }

    list.innerHTML = '';
    notifications.forEach(notif => {
        addNotificationToList(notif, false);
    });
}

/**
 * Add a notification to the list
 */
function addNotificationToList(notif, prepend = false) {
    const list = document.getElementById('notification-list');

    // Remove empty state if present
    const empty = list.querySelector('.notification-empty');
    if (empty) {
        empty.remove();
    }

    const item = document.createElement('div');
    item.className = `notification-item ${notif.read ? '' : 'unread'}`;
    item.dataset.notificationId = notif.id;
    item.dataset.workItemId = notif.work_item_id || '';

    const time = new Date(notif.created);
    const timeStr = formatRelativeTime(time);

    item.innerHTML = `
        <span class="notification-type">${notif.type}</span>
        <div class="notification-title">${escapeHtml(notif.title)}</div>
        <div class="notification-message">${escapeHtml(notif.message)}</div>
        <div class="notification-time">${timeStr}</div>
    `;

    // Click to handle notification
    item.addEventListener('click', () => handleNotificationClick(notif));

    if (prepend) {
        list.prepend(item);
    } else {
        list.appendChild(item);
    }
}

/**
 * Handle notification click
 */
async function handleNotificationClick(notif) {
    // Mark as read
    await markNotificationRead(notif.id);

    // Close panel
    document.getElementById('notification-panel').classList.remove('open');

    // If it's a handoff, load the work item
    if (notif.type === 'handoff' && notif.work_item_id) {
        await loadWorkItem(notif.work_item_id);
    }
}

/**
 * Mark a notification as read
 */
async function markNotificationRead(notificationId) {
    try {
        await fetch(`/api/notifications/${notificationId}/read`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ user_id: currentUserId })
        });

        // Update UI
        const item = document.querySelector(`[data-notification-id="${notificationId}"]`);
        if (item) {
            item.classList.remove('unread');
        }

        // Refresh count
        fetchNotifications();
    } catch (e) {
        console.error('Failed to mark notification read:', e);
    }
}

/**
 * Mark all notifications as read
 */
async function markAllNotificationsRead() {
    try {
        await fetch(`/api/notifications/${currentUserId}/read-all`, {
            method: 'POST'
        });

        // Update UI
        document.querySelectorAll('.notification-item').forEach(item => {
            item.classList.remove('unread');
        });

        updateNotificationBadge(0);
    } catch (e) {
        console.error('Failed to mark all notifications read:', e);
    }
}

/**
 * Load a work item and display it
 */
async function loadWorkItem(workItemId) {
    try {
        const resp = await fetch(`/api/work-items/${workItemId}`);
        const data = await resp.json();

        if (data.error) {
            showError(data.error);
            return;
        }

        currentWorkItemId = workItemId;

        // Display work item context
        displayWorkItemContext(data.work_item, data.handoff_chain);

        // If there's source input, process it
        if (data.work_item.source_input) {
            document.getElementById('main-input').value = data.work_item.source_input;
            await handleSubmit();
        }
    } catch (e) {
        console.error('Failed to load work item:', e);
        showError('Failed to load work item');
    }
}

/**
 * Display work item context and handoff chain
 */
function displayWorkItemContext(workItem, handoffChain) {
    // Show handoff chain if present
    if (handoffChain && handoffChain.length > 1) {
        const chainEl = document.getElementById('handoff-chain');
        chainEl.style.display = 'flex';
        chainEl.innerHTML = '';

        handoffChain.forEach((step, index) => {
            const user = USERS[step.assigned_to] || { name: step.assigned_to_name || 'Unknown', role: 'unknown' };

            const stepEl = document.createElement('div');
            stepEl.className = 'chain-step';
            stepEl.innerHTML = `
                <div class="chain-user">
                    <div class="chain-avatar ${user.role}">${user.name[0]}</div>
                    <div class="chain-name">${user.name}</div>
                </div>
            `;

            if (index < handoffChain.length - 1) {
                const arrowEl = document.createElement('div');
                arrowEl.className = 'chain-arrow';
                arrowEl.innerHTML = `
                    <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M5 12h14M12 5l7 7-7 7"/>
                    </svg>
                `;
                stepEl.appendChild(arrowEl);
            }

            chainEl.appendChild(stepEl);
        });
    }

    // Show work item context
    const contextEl = document.getElementById('work-item-context');
    contextEl.style.display = 'block';
    contextEl.innerHTML = `
        <h4>${escapeHtml(workItem.title)}</h4>
        <div class="work-item-meta">
            <span>Stage: ${workItem.stage}</span>
            <span>Status: ${workItem.status}</span>
            <span>From: ${workItem.created_by}</span>
        </div>
    `;
}

/**
 * Set up handoff UI
 */
function setupHandoffUI() {
    // Send handoff button
    document.getElementById('send-handoff').addEventListener('click', sendHandoff);
}

/**
 * Load handoff targets for current user
 */
async function loadHandoffTargets() {
    try {
        const resp = await fetch(`/api/handoff/targets/${currentUserId}`);
        const data = await resp.json();

        renderHandoffTargets(data.targets);
    } catch (e) {
        console.error('Failed to load handoff targets:', e);
    }
}

/**
 * Render handoff target options
 */
function renderHandoffTargets(targets) {
    const container = document.getElementById('handoff-targets');
    container.innerHTML = '';
    selectedHandoffTarget = null;

    targets.forEach(target => {
        const targetEl = document.createElement('div');
        targetEl.className = 'handoff-target';
        targetEl.dataset.userId = target.id;

        targetEl.innerHTML = `
            <div class="user-avatar ${target.role}">${target.name[0]}</div>
            <div class="handoff-target-info">
                <div class="handoff-target-name">${escapeHtml(target.name)}</div>
                <div class="handoff-target-role">${escapeHtml(target.title)}</div>
            </div>
        `;

        targetEl.addEventListener('click', () => {
            // Clear previous selection
            container.querySelectorAll('.handoff-target').forEach(t => t.classList.remove('selected'));
            // Select this target
            targetEl.classList.add('selected');
            selectedHandoffTarget = target;
        });

        container.appendChild(targetEl);
    });
}

/**
 * Send a handoff
 */
async function sendHandoff() {
    if (!selectedHandoffTarget) {
        showError('Please select someone to hand off to');
        return;
    }

    if (!currentWorkItemId) {
        showError('No work item to hand off');
        return;
    }

    const message = document.getElementById('handoff-message').value.trim();

    try {
        const resp = await fetch('/api/handoff', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                from_user_id: currentUserId,
                to_user_id: selectedHandoffTarget.id,
                work_item_id: currentWorkItemId,
                message: message
            })
        });

        const data = await resp.json();

        if (data.error) {
            showError(data.error);
            return;
        }

        showSuccess(`Work handed off to ${selectedHandoffTarget.name}`);

        // Reset form
        document.getElementById('handoff-message').value = '';
        selectedHandoffTarget = null;
        document.querySelectorAll('.handoff-target').forEach(t => t.classList.remove('selected'));

        // Hide handoff form
        document.getElementById('handoff-form-container').style.display = 'none';

        // Reset to welcome
        setTimeout(() => showWelcome(), 1500);
    } catch (e) {
        console.error('Failed to send handoff:', e);
        showError('Failed to send handoff');
    }
}

/**
 * Show handoff form after content generation
 */
function showHandoffForm() {
    const container = document.getElementById('handoff-form-container');
    container.style.display = 'block';
    loadHandoffTargets();
}

// Utility functions

function formatRelativeTime(date) {
    const now = new Date();
    const diff = now - date;
    const minutes = Math.floor(diff / 60000);
    const hours = Math.floor(diff / 3600000);
    const days = Math.floor(diff / 86400000);

    if (minutes < 1) return 'Just now';
    if (minutes < 60) return `${minutes}m ago`;
    if (hours < 24) return `${hours}h ago`;
    return `${days}d ago`;
}

function escapeHtml(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Request notification permission
if ('Notification' in window && Notification.permission === 'default') {
    Notification.requestPermission();
}

// Override handleSubmit to use role-aware endpoint
const originalHandleSubmit = typeof handleSubmit === 'function' ? handleSubmit : null;

async function handleSubmitRoleAware() {
    const input = document.getElementById('main-input');
    const userInput = input.value.trim();

    if (!userInput) return;

    // Show loading
    showLoading();

    try {
        const resp = await fetch('/api/input/role-aware', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                session_id: sessionId,
                user_id: currentUserId,
                input: userInput
            })
        });

        if (!resp.ok) {
            throw new Error('Request failed');
        }

        const data = await resp.json();

        // Hide loading, show content
        hideLoading();
        showContent();

        // Display generated HTML
        document.getElementById('form-fields').innerHTML = data.html;
        currentViewSpecId = data.view_spec_id;
        currentWorkItemId = data.work_item_id;

        // Show intent in header
        if (data.intent && data.intent.title) {
            document.getElementById('content-header').innerHTML = `
                <span class="intent-badge">${data.intent.intent}</span>
                <h3>${data.intent.title}</h3>
            `;
        }

        // Show extracted anchors if any
        if (data.anchors && data.anchors.length > 0) {
            console.log('Extracted anchors:', data.anchors);
        }

        // Show handoff form if appropriate
        if (data.handoff_targets && data.handoff_targets.length > 0) {
            showHandoffForm();
        }

        // If handoff was detected in text, pre-select target
        if (data.detected_handoff && data.detected_handoff.to_user) {
            const targetUserId = data.detected_handoff.to_user;
            setTimeout(() => {
                const targetEl = document.querySelector(`[data-user-id="${targetUserId}"]`);
                if (targetEl) {
                    targetEl.click();
                }
            }, 100);
        }

    } catch (e) {
        console.error('Submit error:', e);
        showError('Failed to generate interface. Please try again.');
        hideLoading();
    }

    input.value = '';
}

// Replace the default handleSubmit with role-aware version
// This will be called when user submits input
window.handleSubmit = handleSubmitRoleAware;
