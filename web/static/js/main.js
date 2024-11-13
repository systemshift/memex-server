// Main JavaScript file for Memex web interface

document.addEventListener('DOMContentLoaded', function() {
    // Initialize tooltips
    const tooltips = document.querySelectorAll('[data-tooltip]');
    tooltips.forEach(element => {
        element.addEventListener('mouseenter', showTooltip);
        element.addEventListener('mouseleave', hideTooltip);
    });

    // Initialize copy buttons
    const copyButtons = document.querySelectorAll('.copy-button');
    copyButtons.forEach(button => {
        button.addEventListener('click', copyToClipboard);
    });

    // Initialize tag inputs
    const tagInputs = document.querySelectorAll('.tag-input');
    tagInputs.forEach(initializeTagInput);
});

// Tooltip handling
function showTooltip(event) {
    const tooltip = document.createElement('div');
    tooltip.className = 'tooltip';
    tooltip.textContent = this.dataset.tooltip;
    
    document.body.appendChild(tooltip);
    
    const rect = this.getBoundingClientRect();
    const tooltipRect = tooltip.getBoundingClientRect();
    
    tooltip.style.top = `${rect.top - tooltipRect.height - 5}px`;
    tooltip.style.left = `${rect.left + (rect.width - tooltipRect.width) / 2}px`;
}

function hideTooltip() {
    const tooltips = document.querySelectorAll('.tooltip');
    tooltips.forEach(tooltip => tooltip.remove());
}

// Copy to clipboard
function copyToClipboard(event) {
    const text = this.dataset.copy;
    navigator.clipboard.writeText(text).then(() => {
        // Show success message
        const original = this.textContent;
        this.textContent = 'Copied!';
        setTimeout(() => {
            this.textContent = original;
        }, 2000);
    });
}

// Tag input handling
function initializeTagInput(input) {
    const container = document.createElement('div');
    container.className = 'tag-container';
    input.parentNode.insertBefore(container, input);
    container.appendChild(input);

    const tagList = document.createElement('div');
    tagList.className = 'tag-list';
    container.insertBefore(tagList, input);

    input.addEventListener('keydown', function(e) {
        if (e.key === 'Enter' || e.key === ',') {
            e.preventDefault();
            const tag = this.value.trim();
            if (tag) {
                addTag(tagList, tag);
                this.value = '';
                updateHiddenInput(container);
            }
        } else if (e.key === 'Backspace' && !this.value) {
            const tags = tagList.querySelectorAll('.tag');
            if (tags.length) {
                tags[tags.length - 1].remove();
                updateHiddenInput(container);
            }
        }
    });

    // Initialize with existing tags
    if (input.value) {
        const tags = input.value.split(',').map(t => t.trim()).filter(t => t);
        tags.forEach(tag => addTag(tagList, tag));
        input.value = '';
        updateHiddenInput(container);
    }
}

function addTag(tagList, text) {
    const tag = document.createElement('span');
    tag.className = 'tag';
    tag.textContent = text;
    
    const remove = document.createElement('button');
    remove.className = 'remove-tag';
    remove.textContent = '×';
    remove.addEventListener('click', function() {
        tag.remove();
        updateHiddenInput(tagList.parentNode);
    });
    
    tag.appendChild(remove);
    tagList.appendChild(tag);
}

function updateHiddenInput(container) {
    const tags = Array.from(container.querySelectorAll('.tag'))
        .map(tag => tag.textContent.slice(0, -1)) // Remove × button
        .join(',');
    
    let hiddenInput = container.querySelector('input[type="hidden"]');
    if (!hiddenInput) {
        hiddenInput = document.createElement('input');
        hiddenInput.type = 'hidden';
        hiddenInput.name = container.querySelector('input').name;
        container.appendChild(hiddenInput);
    }
    hiddenInput.value = tags;
}

// Show notifications
function showNotification(message, type = 'info') {
    const notification = document.createElement('div');
    notification.className = `notification ${type}`;
    notification.textContent = message;
    
    document.body.appendChild(notification);
    
    setTimeout(() => {
        notification.classList.add('show');
        setTimeout(() => {
            notification.classList.remove('show');
            setTimeout(() => notification.remove(), 300);
        }, 3000);
    }, 100);
}

// Handle form submissions with fetch
function handleFormSubmit(form, options = {}) {
    form.addEventListener('submit', async function(e) {
        e.preventDefault();
        
        try {
            const formData = new FormData(this);
            const response = await fetch(this.action, {
                method: this.method,
                body: formData,
                ...options
            });
            
            if (!response.ok) throw new Error('Request failed');
            
            const data = await response.json();
            
            if (options.onSuccess) {
                options.onSuccess(data);
            } else {
                showNotification('Success!', 'success');
            }
        } catch (error) {
            console.error('Error:', error);
            if (options.onError) {
                options.onError(error);
            } else {
                showNotification('An error occurred', 'error');
            }
        }
    });
}
