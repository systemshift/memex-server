/**
 * Memex Knowledge Graph Visualization
 * D3.js force-directed graph for navigating nodes and links
 */

let simulation, svg, g, zoom;
let nodes = [];
let links = [];

// Node type colors
const nodeColors = {
    Document: { fill: '#3b82f6', stroke: '#60a5fa' },
    Data: { fill: '#22c55e', stroke: '#4ade80' },
    Person: { fill: '#f59e0b', stroke: '#fbbf24' },
    Project: { fill: '#8b5cf6', stroke: '#a78bfa' },
    Task: { fill: '#ef4444', stroke: '#f87171' },
    WorkItem: { fill: '#06b6d4', stroke: '#22d3ee' }
};

// Node type icons (first letter)
const nodeIcons = {
    Document: 'D',
    Data: 'S',
    Person: 'P',
    Project: 'J',
    Task: 'T',
    WorkItem: 'W'
};

function initGraph() {
    const container = document.getElementById('graph-canvas');
    const width = container.clientWidth;
    const height = container.clientHeight;

    svg = d3.select('#graph-canvas')
        .attr('width', width)
        .attr('height', height);

    // Add zoom behavior
    zoom = d3.zoom()
        .scaleExtent([0.1, 4])
        .on('zoom', (event) => {
            g.attr('transform', event.transform);
        });

    svg.call(zoom);

    // Create container group for zoom/pan
    g = svg.append('g');

    // Add arrow markers for links
    svg.append('defs').append('marker')
        .attr('id', 'arrowhead')
        .attr('viewBox', '-0 -5 10 10')
        .attr('refX', 20)
        .attr('refY', 0)
        .attr('orient', 'auto')
        .attr('markerWidth', 6)
        .attr('markerHeight', 6)
        .append('path')
        .attr('d', 'M 0,-5 L 10 ,0 L 0,5')
        .attr('fill', '#555');

    // Initialize simulation
    simulation = d3.forceSimulation()
        .force('link', d3.forceLink().id(d => d.id).distance(100))
        .force('charge', d3.forceManyBody().strength(-300))
        .force('center', d3.forceCenter(width / 2, height / 2))
        .force('collision', d3.forceCollide().radius(40));

    // Setup zoom controls
    document.getElementById('zoom-in')?.addEventListener('click', () => {
        svg.transition().duration(300).call(zoom.scaleBy, 1.3);
    });

    document.getElementById('zoom-out')?.addEventListener('click', () => {
        svg.transition().duration(300).call(zoom.scaleBy, 0.7);
    });

    document.getElementById('reset-zoom')?.addEventListener('click', () => {
        svg.transition().duration(500).call(
            zoom.transform,
            d3.zoomIdentity.translate(width / 2, height / 2).scale(1)
        );
    });

    // Handle resize
    window.addEventListener('resize', () => {
        const newWidth = container.clientWidth;
        const newHeight = container.clientHeight;
        svg.attr('width', newWidth).attr('height', newHeight);
        simulation.force('center', d3.forceCenter(newWidth / 2, newHeight / 2));
        simulation.alpha(0.3).restart();
    });
}

function updateGraph(newNodes, newLinks) {
    nodes = newNodes;
    links = newLinks;

    // Clear existing elements
    g.selectAll('.graph-link').remove();
    g.selectAll('.graph-node').remove();

    // Draw links
    const link = g.selectAll('.graph-link')
        .data(links)
        .enter()
        .append('line')
        .attr('class', 'graph-link')
        .attr('stroke', '#333')
        .attr('stroke-width', 1.5)
        .attr('marker-end', 'url(#arrowhead)');

    // Draw nodes
    const node = g.selectAll('.graph-node')
        .data(nodes)
        .enter()
        .append('g')
        .attr('class', 'graph-node')
        .call(d3.drag()
            .on('start', dragstarted)
            .on('drag', dragged)
            .on('end', dragended))
        .on('click', (event, d) => handleNodeClick(d))
        .on('dblclick', (event, d) => handleNodeDoubleClick(d));

    // Node circles
    node.append('circle')
        .attr('r', 20)
        .attr('fill', d => nodeColors[d.type]?.fill || '#666')
        .attr('stroke', d => nodeColors[d.type]?.stroke || '#888')
        .attr('stroke-width', 2);

    // Node labels (icon)
    node.append('text')
        .attr('text-anchor', 'middle')
        .attr('dy', '0.35em')
        .attr('fill', 'white')
        .attr('font-weight', '600')
        .attr('font-size', '12px')
        .text(d => nodeIcons[d.type] || d.title?.charAt(0)?.toUpperCase() || '?');

    // Node title (below)
    node.append('text')
        .attr('text-anchor', 'middle')
        .attr('dy', '35px')
        .attr('fill', '#888')
        .attr('font-size', '10px')
        .text(d => truncate(d.title || d.id, 15));

    // Update simulation
    simulation.nodes(nodes).on('tick', () => {
        link
            .attr('x1', d => d.source.x)
            .attr('y1', d => d.source.y)
            .attr('x2', d => d.target.x)
            .attr('y2', d => d.target.y);

        node.attr('transform', d => `translate(${d.x},${d.y})`);
    });

    simulation.force('link').links(links);
    simulation.alpha(1).restart();
}

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

function handleNodeClick(node) {
    // Highlight node and show details
    console.log('Node clicked:', node);
}

function handleNodeDoubleClick(node) {
    // Navigate to node view
    const type = node.type?.toLowerCase();
    if (type === 'document') {
        window.location.href = `/docs/${node.id}`;
    } else if (type === 'data') {
        window.location.href = `/sheets/${node.id}`;
    }
}

function truncate(str, maxLength) {
    if (!str) return '';
    return str.length > maxLength ? str.substring(0, maxLength) + '...' : str;
}

// Load nodes from API
async function loadNodes() {
    try {
        const response = await fetch('/api/nodes');
        const data = await response.json();

        if (data.nodes && data.nodes.length > 0) {
            document.getElementById('empty-state')?.style.setProperty('display', 'none');

            // Populate node list
            renderNodeList(data.nodes);

            // Update graph
            updateGraph(data.nodes, data.links || []);
        } else {
            // Show empty state with demo data
            showDemoGraph();
        }
    } catch (error) {
        console.error('Error loading nodes:', error);
        showDemoGraph();
    }
}

function renderNodeList(nodeData) {
    const list = document.getElementById('node-list');
    const emptyState = document.getElementById('empty-state');

    if (!nodeData || nodeData.length === 0) {
        if (emptyState) emptyState.style.display = 'block';
        return;
    }

    if (emptyState) emptyState.style.display = 'none';

    // Clear existing items (except empty state)
    const items = list.querySelectorAll('.node-item');
    items.forEach(item => item.remove());

    // Sort by updated date (most recent first)
    const sorted = [...nodeData].sort((a, b) => {
        const dateA = new Date(a.updated || a.created || 0);
        const dateB = new Date(b.updated || b.created || 0);
        return dateB - dateA;
    });

    // Render each node
    sorted.slice(0, 20).forEach(node => {
        const item = document.createElement('a');
        item.className = 'node-item';
        item.href = getNodeUrl(node);

        const type = node.type?.toLowerCase() || 'document';
        const icon = nodeIcons[node.type] || 'N';

        item.innerHTML = `
            <div class="node-icon ${type}">${icon}</div>
            <div class="node-info">
                <div class="node-title">${escapeHtml(node.title || node.id)}</div>
                <div class="node-meta">${node.type} • ${formatDate(node.updated || node.created)}</div>
            </div>
        `;

        list.appendChild(item);
    });
}

function getNodeUrl(node) {
    const type = node.type?.toLowerCase();
    if (type === 'document') return `/docs/${node.id}`;
    if (type === 'data') return `/sheets/${node.id}`;
    return `/home?node=${node.id}`;
}

function formatDate(dateStr) {
    if (!dateStr) return '';
    const date = new Date(dateStr);
    const now = new Date();
    const diff = now - date;

    if (diff < 60000) return 'just now';
    if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`;
    if (diff < 86400000) return `${Math.floor(diff / 3600000)}h ago`;
    if (diff < 604800000) return `${Math.floor(diff / 86400000)}d ago`;

    return date.toLocaleDateString();
}

function escapeHtml(str) {
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

// Show demo graph when no data
function showDemoGraph() {
    const demoNodes = [
        { id: 'demo-1', type: 'Project', title: 'Acme Onboarding', x: 400, y: 300 },
        { id: 'demo-2', type: 'Document', title: 'Acme Proposal', x: 250, y: 200 },
        { id: 'demo-3', type: 'Data', title: 'Q1 Budget', x: 550, y: 200 },
        { id: 'demo-4', type: 'Person', title: 'Jordan', x: 300, y: 400 },
        { id: 'demo-5', type: 'Person', title: 'Alex', x: 500, y: 400 }
    ];

    const demoLinks = [
        { source: 'demo-2', target: 'demo-1' },
        { source: 'demo-3', target: 'demo-1' },
        { source: 'demo-4', target: 'demo-1' },
        { source: 'demo-5', target: 'demo-2' },
        { source: 'demo-4', target: 'demo-3' }
    ];

    updateGraph(demoNodes, demoLinks);
}

// Search functionality
document.getElementById('search-input')?.addEventListener('input', (e) => {
    const query = e.target.value.toLowerCase();

    // Filter node list
    const items = document.querySelectorAll('.node-item');
    items.forEach(item => {
        const title = item.querySelector('.node-title')?.textContent?.toLowerCase() || '';
        item.style.display = title.includes(query) ? 'flex' : 'none';
    });

    // Highlight matching nodes in graph
    if (query) {
        g.selectAll('.graph-node')
            .attr('opacity', d => {
                const title = (d.title || d.id).toLowerCase();
                return title.includes(query) ? 1 : 0.3;
            });
    } else {
        g.selectAll('.graph-node').attr('opacity', 1);
    }
});
