/**
 * Memex Workspace - Client-side JavaScript
 */

let sessionId = null;
let currentViewSpecId = null;

/**
 * Initialize the workspace
 */
async function initWorkspace() {
    // Create session
    try {
        const resp = await fetch('/api/session', { method: 'POST' });
        const data = await resp.json();
        sessionId = data.session_id;
        document.getElementById('session-info').textContent = `Session: ${sessionId}`;
    } catch (e) {
        console.error('Failed to create session:', e);
    }

    // Set up event listeners
    setupEventListeners();
}

/**
 * Set up event listeners
 */
function setupEventListeners() {
    // Main input
    const input = document.getElementById('main-input');
    const submitBtn = document.getElementById('submit-btn');

    input.addEventListener('keypress', (e) => {
        if (e.key === 'Enter') {
            handleSubmit();
        }
    });

    submitBtn.addEventListener('click', handleSubmit);

    // Quick actions
    document.querySelectorAll('.quick-action').forEach(btn => {
        btn.addEventListener('click', () => {
            input.value = btn.dataset.input;
            input.focus();
        });
    });

    // Examples
    document.querySelectorAll('.example').forEach(example => {
        example.addEventListener('click', () => {
            input.value = example.dataset.input;
            handleSubmit();
        });
    });

    // Form submission
    document.getElementById('generated-form').addEventListener('submit', handleFormSubmit);
}

/**
 * Handle input submission
 */
async function handleSubmit() {
    const input = document.getElementById('main-input');
    const userInput = input.value.trim();

    if (!userInput) return;

    // Show loading
    showLoading();

    try {
        // Use streaming endpoint
        await handleStreamingSubmit(userInput);
    } catch (e) {
        console.error('Submit error:', e);
        showError('Failed to generate interface. Please try again.');
    }

    input.value = '';
}

/**
 * Handle streaming submission using Server-Sent Events
 */
async function handleStreamingSubmit(userInput) {
    const formFields = document.getElementById('form-fields');
    const contentHeader = document.getElementById('content-header');

    // Clear previous content
    formFields.innerHTML = '';
    contentHeader.innerHTML = '';

    // Make request
    const resp = await fetch('/api/input/stream', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            session_id: sessionId,
            input: userInput
        })
    });

    if (!resp.ok) {
        throw new Error('Request failed');
    }

    // Read stream
    const reader = resp.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';

    while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });

        // Process complete SSE messages
        const lines = buffer.split('\n');
        buffer = lines.pop(); // Keep incomplete line in buffer

        for (const line of lines) {
            if (line.startsWith('data: ')) {
                const jsonStr = line.slice(6);
                try {
                    const event = JSON.parse(jsonStr);
                    handleStreamEvent(event);
                } catch (e) {
                    console.warn('Failed to parse event:', jsonStr);
                }
            }
        }
    }
}

/**
 * Handle a stream event
 */
function handleStreamEvent(event) {
    const formFields = document.getElementById('form-fields');
    const contentHeader = document.getElementById('content-header');

    switch (event.type) {
        case 'intent':
            // Show intent info in header
            if (event.data.title) {
                contentHeader.innerHTML = `
                    <span class="intent-badge">${event.data.intent}</span>
                    <h3>${event.data.title}</h3>
                `;
            }
            break;

        case 'component':
            // Hide loading, show content
            hideLoading();
            showContent();

            // Append component HTML
            const div = document.createElement('div');
            div.innerHTML = event.html;
            formFields.appendChild(div.firstElementChild || div);
            break;

        case 'complete':
            currentViewSpecId = event.view_spec_id;
            console.log('Generation complete:', event.view_spec_id);
            break;
    }
}

/**
 * Handle non-streaming submission (fallback)
 */
async function handleRegularSubmit(userInput) {
    const resp = await fetch('/api/input', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            session_id: sessionId,
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

    // Show intent in header
    if (data.intent && data.intent.title) {
        document.getElementById('content-header').innerHTML = `
            <span class="intent-badge">${data.intent.intent}</span>
            <h3>${data.intent.title}</h3>
        `;
    }
}

/**
 * Handle form submission
 */
async function handleFormSubmit(e) {
    e.preventDefault();

    if (!currentViewSpecId) {
        console.error('No view spec ID');
        return;
    }

    const form = e.target;
    const formData = new FormData(form);
    const fields = {};

    for (const [key, value] of formData.entries()) {
        fields[key] = value;
    }

    try {
        // Update fields
        await fetch('/api/update', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                session_id: sessionId,
                view_spec_id: currentViewSpecId,
                fields: fields
            })
        });

        // Save to Memex
        const saveResp = await fetch('/api/save', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                session_id: sessionId,
                view_spec_id: currentViewSpecId
            })
        });

        const saveData = await saveResp.json();

        if (saveData.memex_id) {
            showSuccess(`Saved to Memex: ${saveData.memex_id}`);
        } else {
            showSuccess('Saved locally');
        }
    } catch (e) {
        console.error('Save error:', e);
        showError('Failed to save. Please try again.');
    }
}

/**
 * Show loading state
 */
function showLoading() {
    document.getElementById('welcome').style.display = 'none';
    document.getElementById('generated-content').style.display = 'none';
    document.getElementById('loading').style.display = 'flex';
}

/**
 * Hide loading state
 */
function hideLoading() {
    document.getElementById('loading').style.display = 'none';
}

/**
 * Show generated content
 */
function showContent() {
    document.getElementById('welcome').style.display = 'none';
    document.getElementById('loading').style.display = 'none';
    document.getElementById('generated-content').style.display = 'block';
}

/**
 * Show welcome state
 */
function showWelcome() {
    document.getElementById('welcome').style.display = 'block';
    document.getElementById('generated-content').style.display = 'none';
    document.getElementById('loading').style.display = 'none';
}

/**
 * Show success message
 */
function showSuccess(message) {
    // Simple alert for now - could use a toast notification
    alert(message);
}

/**
 * Show error message
 */
function showError(message) {
    alert('Error: ' + message);
}

/**
 * Set input value (used by quick actions and examples)
 */
function setInput(text) {
    document.getElementById('main-input').value = text;
    document.getElementById('main-input').focus();
}
