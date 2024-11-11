// Main JavaScript file for Memex web interface

document.addEventListener('DOMContentLoaded', function() {
    // Initialize any UI components
    initializeUI();

    // Set up event handlers
    setupEventHandlers();
});

function initializeUI() {
    // Add active class to current nav link
    const currentPath = window.location.pathname;
    document.querySelectorAll('.nav-links a').forEach(link => {
        if (link.getAttribute('href') === currentPath) {
            link.classList.add('active');
        }
    });
}

function setupEventHandlers() {
    // Handle file upload form
    const uploadForm = document.getElementById('upload-form');
    if (uploadForm) {
        uploadForm.addEventListener('submit', handleFileUpload);
    }

    // Handle commit form
    const commitForm = document.getElementById('commit-form');
    if (commitForm) {
        commitForm.addEventListener('submit', handleCommit);
    }
}

async function handleFileUpload(event) {
    event.preventDefault();
    
    try {
        const formData = new FormData(event.target);
        const response = await fetch('/api/add', {
            method: 'POST',
            body: formData
        });

        if (!response.ok) {
            throw new Error('Upload failed');
        }

        const result = await response.json();
        showNotification('File uploaded successfully');
        
        // Refresh the file list if it exists
        refreshFileList();
    } catch (error) {
        showNotification('Error uploading file: ' + error.message, 'error');
    }
}

async function handleCommit(event) {
    event.preventDefault();
    
    try {
        const formData = new FormData(event.target);
        const response = await fetch('/api/commit', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                message: formData.get('message')
            })
        });

        if (!response.ok) {
            throw new Error('Commit failed');
        }

        const result = await response.json();
        showNotification('Changes committed successfully');
        
        // Refresh the status display
        refreshStatus();
    } catch (error) {
        showNotification('Error creating commit: ' + error.message, 'error');
    }
}

function showNotification(message, type = 'success') {
    // Create notification element
    const notification = document.createElement('div');
    notification.className = `notification ${type}`;
    notification.textContent = message;

    // Add to document
    document.body.appendChild(notification);

    // Remove after delay
    setTimeout(() => {
        notification.remove();
    }, 3000);
}

async function refreshFileList() {
    const fileList = document.querySelector('.uncommitted-files ul');
    if (fileList) {
        try {
            const response = await fetch('/api/status');
            const data = await response.json();
            
            // Update the file list
            fileList.innerHTML = data.uncommittedFiles
                .map(file => `
                    <li>
                        <span class="filename">${file.name}</span>
                        <span class="modified">${new Date(file.modified).toLocaleString()}</span>
                    </li>
                `)
                .join('');
        } catch (error) {
            console.error('Error refreshing file list:', error);
        }
    }
}

async function refreshStatus() {
    try {
        const response = await fetch('/api/status');
        const data = await response.json();
        
        // Update last commit info
        const lastCommit = document.querySelector('.last-commit');
        if (lastCommit && data.lastCommit) {
            lastCommit.innerHTML = `
                <h3>Last Commit</h3>
                <p>Hash: ${data.lastCommit.hash}</p>
                <p>Message: ${data.lastCommit.message}</p>
                <p>Date: ${new Date(data.lastCommit.timestamp).toLocaleString()}</p>
            `;
        }

        // Update uncommitted files
        refreshFileList();
    } catch (error) {
        console.error('Error refreshing status:', error);
    }
}
