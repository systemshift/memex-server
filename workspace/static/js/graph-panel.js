/**
 * Memex Graph Panel
 * Mini D3 force graph visualization for anchor connections
 */

(function() {
    'use strict';

    let currentEmail = null;
    let anchors = [];
    let selectedAnchor = null;
    let connections = [];

    // D3 elements
    let svg = null;
    let simulation = null;
    let nodes = [];
    let links = [];

    // Initialize graph panel
    window.initGraphPanel = function() {
        setupD3();
        setupEventListeners();
    };

    // Setup D3 force simulation
    function setupD3() {
        const container = document.getElementById('graph-viz');
        svg = d3.select('#graph-svg');

        const width = container.offsetWidth;
        const height = container.offsetHeight;

        svg.attr('viewBox', `0 0 ${width} ${height}`);

        // Create groups for links and nodes
        svg.append('g').attr('class', 'links');
        svg.append('g').attr('class', 'nodes');

        // Initialize simulation
        simulation = d3.forceSimulation()
            .force('link', d3.forceLink().id(d => d.id).distance(60))
            .force('charge', d3.forceManyBody().strength(-100))
            .force('center', d3.forceCenter(width / 2, height / 2))
            .force('collision', d3.forceCollide().radius(25));

        simulation.on('tick', ticked);
    }

    // Setup event listeners
    function setupEventListeners() {
        // Listen for email loaded
        window.addEventListener('email-loaded', (e) => {
            currentEmail = e.detail.email;
            anchors = e.detail.anchors;
            renderGraph();
        });

        // Listen for anchor selection
        window.addEventListener('anchor-selected', (e) => {
            selectedAnchor = e.detail.anchor;
            loadConnections(selectedAnchor.id);
            highlightNode(selectedAnchor.id);
        });
    }

    // Render graph with email and anchors
    function renderGraph() {
        if (!currentEmail || !anchors.length) {
            renderEmptyGraph();
            return;
        }

        // Build nodes: email + anchors
        nodes = [
            {
                id: currentEmail.id,
                type: 'email',
                label: currentEmail.subject?.substring(0, 20) || 'Email',
                isEmail: true
            },
            ...anchors.map(anchor => ({
                id: anchor.id,
                type: anchor.type,
                label: anchor.text?.substring(0, 15) || anchor.type,
                isAnchor: true
            }))
        ];

        // Build links: anchors -> email
        links = anchors.map(anchor => ({
            source: anchor.id,
            target: currentEmail.id,
            type: 'EXTRACTED_FROM'
        }));

        updateGraph();
    }

    // Render empty graph state
    function renderEmptyGraph() {
        svg.select('.nodes').selectAll('*').remove();
        svg.select('.links').selectAll('*').remove();
    }

    // Update graph visualization
    function updateGraph() {
        const container = document.getElementById('graph-viz');
        const width = container.offsetWidth;
        const height = container.offsetHeight;

        // Update links
        const linkSelection = svg.select('.links')
            .selectAll('line')
            .data(links, d => `${d.source.id || d.source}-${d.target.id || d.target}`);

        linkSelection.exit().remove();

        const linkEnter = linkSelection.enter()
            .append('line')
            .attr('class', 'graph-link')
            .attr('stroke-width', 1);

        // Update nodes
        const nodeSelection = svg.select('.nodes')
            .selectAll('.graph-node')
            .data(nodes, d => d.id);

        nodeSelection.exit().remove();

        const nodeEnter = nodeSelection.enter()
            .append('g')
            .attr('class', d => `graph-node ${d.type}`)
            .call(drag(simulation));

        nodeEnter.append('circle')
            .attr('r', d => d.isEmail ? 20 : 12)
            .attr('fill', d => getNodeColor(d));

        nodeEnter.append('text')
            .attr('dy', d => d.isEmail ? 30 : 22)
            .text(d => d.label);

        // Click handler
        nodeEnter.on('click', (event, d) => {
            if (d.isAnchor) {
                selectedAnchor = anchors.find(a => a.id === d.id);
                if (selectedAnchor) {
                    window.dispatchEvent(new CustomEvent('anchor-selected', {
                        detail: { anchor: selectedAnchor }
                    }));
                }
            }
        });

        // Merge enter and update
        const allNodes = nodeEnter.merge(nodeSelection);
        const allLinks = linkEnter.merge(linkSelection);

        // Update simulation
        simulation.nodes(nodes);
        simulation.force('link').links(links);
        simulation.alpha(0.3).restart();
    }

    // Tick function for simulation
    function ticked() {
        const container = document.getElementById('graph-viz');
        const width = container.offsetWidth;
        const height = container.offsetHeight;

        svg.select('.links').selectAll('line')
            .attr('x1', d => clamp(d.source.x, 20, width - 20))
            .attr('y1', d => clamp(d.source.y, 20, height - 20))
            .attr('x2', d => clamp(d.target.x, 20, width - 20))
            .attr('y2', d => clamp(d.target.y, 20, height - 20));

        svg.select('.nodes').selectAll('.graph-node')
            .attr('transform', d => {
                const x = clamp(d.x, 20, width - 20);
                const y = clamp(d.y, 20, height - 20);
                return `translate(${x}, ${y})`;
            });
    }

    // Drag behavior
    function drag(simulation) {
        function dragstarted(event) {
            if (!event.active) simulation.alphaTarget(0.3).restart();
            event.subject.fx = event.subject.x;
            event.subject.fy = event.subject.y;
        }

        function dragged(event) {
            event.subject.fx = event.x;
            event.subject.fy = event.y;
        }

        function dragended(event) {
            if (!event.active) simulation.alphaTarget(0);
            event.subject.fx = null;
            event.subject.fy = null;
        }

        return d3.drag()
            .on('start', dragstarted)
            .on('drag', dragged)
            .on('end', dragended);
    }

    // Highlight selected node
    function highlightNode(nodeId) {
        svg.selectAll('.graph-node')
            .classed('selected', d => d.id === nodeId);
    }

    // Load connections for selected anchor
    async function loadConnections(anchorId) {
        const connectionsSection = document.getElementById('connections-section');
        const connectionsList = document.getElementById('connections-list');

        try {
            const response = await fetch(`/api/anchors/${anchorId}/connections`);
            const data = await response.json();

            connections = data.connections || [];

            if (connections.length === 0) {
                connectionsSection.style.display = 'none';
                return;
            }

            connectionsSection.style.display = 'block';
            connectionsList.innerHTML = connections.map(conn => `
                <div class="connection-item">
                    <span class="connection-type">${conn.link_type}</span>
                    <span>${escapeHtml(conn.title || conn.id)}</span>
                </div>
            `).join('');

            // Add connected nodes to graph
            addConnectionsToGraph(connections);

        } catch (error) {
            console.error('Error loading connections:', error);
            connectionsSection.style.display = 'none';
        }
    }

    // Add connections to the graph
    function addConnectionsToGraph(connections) {
        if (!selectedAnchor) return;

        // Add connection nodes if not already present
        connections.forEach(conn => {
            if (!nodes.find(n => n.id === conn.id)) {
                nodes.push({
                    id: conn.id,
                    type: conn.type || 'entity',
                    label: (conn.title || conn.id).substring(0, 15),
                    isConnection: true
                });

                links.push({
                    source: selectedAnchor.id,
                    target: conn.id,
                    type: conn.link_type
                });
            }
        });

        updateGraph();
    }

    // Get color for node type
    function getNodeColor(node) {
        if (node.isEmail) return '#3b82f6';

        const colors = {
            person: '#f59e0b',
            organization: '#3b82f6',
            action_item: '#22c55e',
            decision: '#f97316',
            date: '#8b5cf6',
            topic: '#06b6d4',
            entity: '#666'
        };

        return colors[node.type] || '#666';
    }

    // Clamp value to range
    function clamp(val, min, max) {
        return Math.max(min, Math.min(max, val));
    }

    // Escape HTML
    function escapeHtml(str) {
        if (!str) return '';
        const div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    }

    // Check if D3 is loaded
    if (typeof d3 === 'undefined') {
        // Load D3 dynamically
        const script = document.createElement('script');
        script.src = 'https://d3js.org/d3.v7.min.js';
        script.onload = () => {
            console.log('D3 loaded');
        };
        document.head.appendChild(script);
    }

})();
