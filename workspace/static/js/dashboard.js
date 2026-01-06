/**
 * Memex Workspace - Dashboard (Boss View)
 * D3.js graph visualization and real-time activity feed
 */

let graphData = { nodes: [], edges: [] };
let simulation = null;
let svg = null;
let activityEventSource = null;
let currentFilter = 'all';

/**
 * Initialize dashboard
 */
async function initDashboard() {
    // Load initial data
    await Promise.all([
        loadStats(),
        loadGraph(),
        loadActivity(),
        loadWorkItems()
    ]);

    // Set up graph
    setupGraph();

    // Set up event listeners
    setupEventListeners();

    // Start activity stream
    startActivityStream();

    // Auto-refresh every 30 seconds
    setInterval(async () => {
        await loadStats();
        await loadWorkItems();
    }, 30000);
}

/**
 * Load workflow statistics
 */
async function loadStats() {
    try {
        const resp = await fetch('/api/dashboard/stats');
        const data = await resp.json();

        document.getElementById('stat-total').textContent = data.total || 0;
        document.getElementById('stat-handoffs').textContent = data.handoffs_total || 0;

        const byStatus = data.by_status || {};
        document.getElementById('stat-pending').textContent = byStatus.pending || 0;
        document.getElementById('stat-in-progress').textContent = byStatus.in_progress || 0;
        document.getElementById('stat-complete').textContent = byStatus.complete || 0;

        // Update team counts
        const byUser = data.by_user || {};
        document.getElementById('alex-count').textContent = byUser.alex || 0;
        document.getElementById('jordan-count').textContent = byUser.jordan || 0;
        document.getElementById('sam-count').textContent = byUser.sam || 0;
    } catch (e) {
        console.error('Failed to load stats:', e);
    }
}

/**
 * Load graph data
 */
async function loadGraph() {
    try {
        const resp = await fetch('/api/dashboard/graph');
        graphData = await resp.json();
        if (svg) {
            updateGraph();
        }
    } catch (e) {
        console.error('Failed to load graph:', e);
    }
}

/**
 * Set up D3 graph visualization
 */
function setupGraph() {
    const container = document.getElementById('graph-container');
    const width = container.clientWidth;
    const height = container.clientHeight || 400;

    // Create SVG
    svg = d3.select('#graph-container')
        .append('svg')
        .attr('width', width)
        .attr('height', height)
        .attr('viewBox', [0, 0, width, height]);

    // Add zoom behavior
    const g = svg.append('g');

    svg.call(d3.zoom()
        .scaleExtent([0.5, 3])
        .on('zoom', (event) => {
            g.attr('transform', event.transform);
        }));

    // Create groups for edges and nodes
    g.append('g').attr('class', 'edges');
    g.append('g').attr('class', 'nodes');

    // Initial render
    updateGraph();
}

/**
 * Update graph visualization
 */
function updateGraph() {
    if (!svg) return;

    const container = document.getElementById('graph-container');
    const width = container.clientWidth;
    const height = container.clientHeight || 400;

    const g = svg.select('g');
    const nodes = graphData.nodes || [];
    const edges = graphData.edges || [];

    // Create node map for edge references
    const nodeMap = new Map(nodes.map(n => [n.id, n]));

    // Process edges to reference node objects
    const links = edges
        .filter(e => nodeMap.has(e.source) && nodeMap.has(e.target))
        .map(e => ({
            source: e.source,
            target: e.target,
            type: e.type
        }));

    // Create simulation
    if (simulation) {
        simulation.stop();
    }

    simulation = d3.forceSimulation(nodes)
        .force('link', d3.forceLink(links).id(d => d.id).distance(100))
        .force('charge', d3.forceManyBody().strength(-200))
        .force('center', d3.forceCenter(width / 2, height / 2))
        .force('collision', d3.forceCollide().radius(40));

    // Draw edges
    const edgeSelection = g.select('.edges')
        .selectAll('line')
        .data(links, d => `${d.source.id || d.source}-${d.target.id || d.target}`);

    edgeSelection.exit().remove();

    const edgeEnter = edgeSelection.enter()
        .append('line')
        .attr('stroke', '#666')
        .attr('stroke-opacity', 0.6)
        .attr('stroke-width', 1.5);

    const edge = edgeEnter.merge(edgeSelection);

    // Draw nodes
    const nodeSelection = g.select('.nodes')
        .selectAll('g')
        .data(nodes, d => d.id);

    nodeSelection.exit().remove();

    const nodeEnter = nodeSelection.enter()
        .append('g')
        .attr('class', 'node')
        .call(d3.drag()
            .on('start', dragstarted)
            .on('drag', dragged)
            .on('end', dragended));

    nodeEnter.append('circle')
        .attr('r', 20)
        .attr('fill', d => getNodeColor(d.type))
        .attr('stroke', '#fff')
        .attr('stroke-width', 2);

    nodeEnter.append('text')
        .attr('text-anchor', 'middle')
        .attr('dy', '.35em')
        .attr('fill', '#fff')
        .attr('font-size', '10px')
        .attr('font-weight', '600')
        .text(d => getNodeLabel(d));

    nodeEnter.append('title')
        .text(d => `${d.type}: ${d.label || d.id}`);

    const node = nodeEnter.merge(nodeSelection);

    // Click handler for nodes
    node.on('click', (event, d) => showNodeDetail(d));

    // Update positions on tick
    simulation.on('tick', () => {
        edge
            .attr('x1', d => d.source.x)
            .attr('y1', d => d.source.y)
            .attr('x2', d => d.target.x)
            .attr('y2', d => d.target.y);

        node.attr('transform', d => `translate(${d.x},${d.y})`);
    });

    // Drag functions
    function dragstarted(event, d) {
        if (!event.active) simulation.alphaTarget(0.3).restart();
        d.fx = d.x;
        d.fy = d.y;
    }

    function dragged(event, d) {
        d.fx = event.x;
        d.fy = event.y;
    }

    function dragended(event, d) {
        if (!event.active) simulation.alphaTarget(0);
        d.fx = null;
        d.fy = null;
    }
}

/**
 * Get color for node type
 */
function getNodeColor(type) {
    const colors = {
        'Deal': '#10b981',
        'WorkItem': '#6366f1',
        'Company': '#f59e0b',
        'User': '#8b5cf6',
        'default': '#6b7280'
    };
    return colors[type] || colors.default;
}

/**
 * Get label for node
 */
function getNodeLabel(node) {
    const label = node.label || node.id;
    return label.slice(0, 2).toUpperCase();
}

/**
 * Show node detail modal
 */
function showNodeDetail(node) {
    const modal = document.getElementById('node-modal');
    const title = document.getElementById('modal-title');
    const body = document.getElementById('modal-body');

    title.textContent = node.label || node.id;

    body.innerHTML = `
        <div class="detail-row">
            <span class="detail-label">Type</span>
            <span class="detail-value">${node.type}</span>
        </div>
        <div class="detail-row">
            <span class="detail-label">ID</span>
            <span class="detail-value">${node.id}</span>
        </div>
        ${node.status ? `
        <div class="detail-row">
            <span class="detail-label">Status</span>
            <span class="detail-value status-${node.status}">${node.status}</span>
        </div>
        ` : ''}
        ${node.stage ? `
        <div class="detail-row">
            <span class="detail-label">Stage</span>
            <span class="detail-value">${node.stage}</span>
        </div>
        ` : ''}
        ${node.assigned_to ? `
        <div class="detail-row">
            <span class="detail-label">Assigned To</span>
            <span class="detail-value">${node.assigned_to}</span>
        </div>
        ` : ''}
        ${node.industry ? `
        <div class="detail-row">
            <span class="detail-label">Industry</span>
            <span class="detail-value">${node.industry}</span>
        </div>
        ` : ''}
    `;

    modal.style.display = 'flex';
}

/**
 * Load activity feed
 */
async function loadActivity() {
    try {
        const resp = await fetch('/api/dashboard/activity?limit=20');
        const data = await resp.json();
        renderActivityList(data.activities || []);
    } catch (e) {
        console.error('Failed to load activity:', e);
    }
}

/**
 * Start SSE activity stream
 */
function startActivityStream() {
    activityEventSource = new EventSource('/api/dashboard/activity/stream');

    activityEventSource.onmessage = (event) => {
        try {
            const data = JSON.parse(event.data);
            if (data.type === 'activity') {
                renderActivityList(data.activities || []);
            }
        } catch (e) {
            console.warn('Failed to parse activity event:', e);
        }
    };

    activityEventSource.onerror = () => {
        setTimeout(() => {
            if (activityEventSource.readyState === EventSource.CLOSED) {
                startActivityStream();
            }
        }, 5000);
    };
}

/**
 * Render activity list
 */
function renderActivityList(activities) {
    const list = document.getElementById('activity-list');

    if (activities.length === 0) {
        list.innerHTML = '<div class="activity-empty">No recent activity</div>';
        return;
    }

    list.innerHTML = activities.map(activity => {
        const time = new Date(activity.timestamp);
        const timeStr = formatRelativeTime(time);
        const icon = getActivityIcon(activity.type);

        return `
            <div class="activity-item">
                <div class="activity-icon">${icon}</div>
                <div class="activity-content">
                    <div class="activity-title">${escapeHtml(activity.title)}</div>
                    <div class="activity-meta">
                        ${activity.user_id ? `<span>${activity.user_id}</span>` : ''}
                        <span>${timeStr}</span>
                    </div>
                </div>
            </div>
        `;
    }).join('');
}

/**
 * Get icon for activity type
 */
function getActivityIcon(type) {
    const icons = {
        'work_item_created': `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 5v14M5 12h14"/></svg>`,
        'handoff': `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M5 12h14M12 5l7 7-7 7"/></svg>`,
        'handoff_accepted': `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M20 6L9 17l-5-5"/></svg>`,
        'status_change': `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><path d="M12 6v6l4 2"/></svg>`,
        'notification': `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"/></svg>`
    };
    return icons[type] || icons.notification;
}

/**
 * Load work items
 */
async function loadWorkItems() {
    try {
        const resp = await fetch('/api/dashboard/work-items');
        const data = await resp.json();
        renderWorkItemsList(data.work_items || []);
    } catch (e) {
        console.error('Failed to load work items:', e);
    }
}

/**
 * Render work items list
 */
function renderWorkItemsList(items) {
    const list = document.getElementById('items-list');

    // Filter items
    let filteredItems = items;
    if (currentFilter !== 'all') {
        filteredItems = items.filter(item => item.status === currentFilter);
    }

    if (filteredItems.length === 0) {
        list.innerHTML = '<div class="items-empty">No work items</div>';
        return;
    }

    list.innerHTML = filteredItems.map(item => {
        const statusClass = `status-${item.status.replace('_', '-')}`;
        const time = new Date(item.created);
        const timeStr = formatRelativeTime(time);

        return `
            <div class="work-item" data-id="${item.id}">
                <div class="work-item-header">
                    <span class="work-item-title">${escapeHtml(item.title)}</span>
                    <span class="work-item-status ${statusClass}">${item.status}</span>
                </div>
                <div class="work-item-meta">
                    <span>Stage: ${item.stage || 'N/A'}</span>
                    <span>Assigned: ${item.assigned_to || 'Unassigned'}</span>
                    <span>${timeStr}</span>
                </div>
            </div>
        `;
    }).join('');
}

/**
 * Set up event listeners
 */
function setupEventListeners() {
    // Refresh graph button
    document.getElementById('refresh-graph').addEventListener('click', async () => {
        await loadGraph();
    });

    // Modal close
    document.getElementById('modal-close').addEventListener('click', () => {
        document.getElementById('node-modal').style.display = 'none';
    });

    document.querySelector('.modal-backdrop').addEventListener('click', () => {
        document.getElementById('node-modal').style.display = 'none';
    });

    // Filter buttons
    document.querySelectorAll('.filter-btn').forEach(btn => {
        btn.addEventListener('click', () => {
            document.querySelectorAll('.filter-btn').forEach(b => b.classList.remove('active'));
            btn.classList.add('active');
            currentFilter = btn.dataset.filter;
            loadWorkItems();
        });
    });

    // Resize handler
    window.addEventListener('resize', () => {
        if (svg) {
            const container = document.getElementById('graph-container');
            svg.attr('width', container.clientWidth)
               .attr('height', container.clientHeight || 400);
            if (simulation) {
                simulation.alpha(0.3).restart();
            }
        }
    });
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
