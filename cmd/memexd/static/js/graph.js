// Graph visualization using D3.js
const width = 900;
const height = 600;

// Create SVG container
const svg = d3.select('#graph')
    .attr('width', width)
    .attr('height', height);

// Create forces for graph layout
const simulation = d3.forceSimulation()
    .force('link', d3.forceLink().id(d => d.id).distance(100))
    .force('charge', d3.forceManyBody().strength(-300))
    .force('center', d3.forceCenter(width / 2, height / 2))
    .force('collision', d3.forceCollide().radius(50));

// Load and render graph data
async function loadGraph() {
    try {
        // Show loading state
        svg.append('text')
            .attr('class', 'loading')
            .attr('x', width / 2)
            .attr('y', height / 2)
            .attr('text-anchor', 'middle')
            .text('Loading graph...');

        const response = await fetch('/api/graph');
        if (!response.ok) throw new Error('Failed to load graph data');
        const graph = await response.json();
        
        // Clear loading state
        svg.selectAll('.loading').remove();

        // Create arrow marker for directed links
        svg.append('defs').selectAll('marker')
            .data(['arrow'])
            .join('marker')
            .attr('id', d => d)
            .attr('viewBox', '0 -5 10 10')
            .attr('refX', 20)
            .attr('refY', 0)
            .attr('markerWidth', 6)
            .attr('markerHeight', 6)
            .attr('orient', 'auto')
            .append('path')
            .attr('d', 'M0,-5L10,0L0,5')
            .attr('fill', '#999');

        // Create links
        const link = svg.append('g')
            .attr('class', 'links')
            .selectAll('line')
            .data(graph.links)
            .join('line')
            .attr('stroke', '#999')
            .attr('stroke-opacity', 0.6)
            .attr('stroke-width', 1)
            .attr('marker-end', 'url(#arrow)')
            .on('mouseover', function(event, d) {
                showTooltip(event, `Type: ${d.type}\nCreated: ${d.created}\nModified: ${d.modified}`);
            })
            .on('mouseout', hideTooltip);

        // Create nodes
        const node = svg.append('g')
            .attr('class', 'nodes')
            .selectAll('g')
            .data(graph.nodes)
            .join('g')
            .call(drag(simulation));

        // Add circles to nodes
        node.append('circle')
            .attr('r', 10)
            .attr('fill', d => getNodeColor(d.type))
            .on('mouseover', function(event, d) {
                const tooltip = `Type: ${d.type}\nCreated: ${d.created}\nModified: ${d.modified}`;
                showTooltip(event, tooltip);
            })
            .on('mouseout', hideTooltip)
            .on('click', async function(event, d) {
                try {
                    const response = await fetch(`/api/nodes/${d.id}`);
                    if (!response.ok) throw new Error('Failed to load node data');
                    const nodeData = await response.json();
                    showNodeDetails(nodeData);
                } catch (error) {
                    console.error('Error loading node details:', error);
                }
            });

        // Add labels to nodes
        node.append('text')
            .text(d => getNodeLabel(d))
            .attr('x', 15)
            .attr('y', 5)
            .attr('font-size', '12px');

        // Update simulation
        simulation.nodes(graph.nodes)
            .on('tick', () => {
                link
                    .attr('x1', d => d.source.x)
                    .attr('y1', d => d.source.y)
                    .attr('x2', d => d.target.x)
                    .attr('y2', d => d.target.y);

                node
                    .attr('transform', d => `translate(${d.x},${d.y})`);
            });

        simulation.force('link').links(graph.links);

        // Add zoom behavior
        const zoom = d3.zoom()
            .scaleExtent([0.1, 4])
            .on('zoom', (event) => {
                svg.selectAll('.nodes, .links')
                    .attr('transform', event.transform);
            });

        svg.call(zoom);

    } catch (error) {
        console.error('Error loading graph:', error);
        svg.selectAll('.loading').text('Error loading graph data');
    }
}

// Helper function to get node color based on type
function getNodeColor(type) {
    const colors = {
        'file': '#1f77b4',
        'note': '#2ca02c',
        'tag': '#ff7f0e',
        'default': '#7f7f7f'
    };
    return colors[type] || colors.default;
}

// Helper function to get node label
function getNodeLabel(node) {
    if (node.meta && node.meta.title) return node.meta.title;
    if (node.meta && node.meta.filename) return node.meta.filename;
    return node.id.substring(0, 8);
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

// Tooltip handling
const tooltip = d3.select('body').append('div')
    .attr('class', 'tooltip')
    .style('opacity', 0);

function showTooltip(event, text) {
    tooltip.transition()
        .duration(200)
        .style('opacity', .9);
    tooltip.html(text.replace(/\n/g, '<br/>'))
        .style('left', (event.pageX + 10) + 'px')
        .style('top', (event.pageY - 28) + 'px');
}

function hideTooltip() {
    tooltip.transition()
        .duration(500)
        .style('opacity', 0);
}

// Node details panel
function showNodeDetails(node) {
    const panel = document.getElementById('node-details');
    let content = `
        <h3>Node Details</h3>
        <p><strong>ID:</strong> ${node.id}</p>
        <p><strong>Type:</strong> ${node.type}</p>
        <p><strong>Created:</strong> ${node.created}</p>
        <p><strong>Modified:</strong> ${node.modified}</p>
    `;

    if (node.meta) {
        content += '<h4>Metadata</h4>';
        for (const [key, value] of Object.entries(node.meta)) {
            if (key !== 'chunks') { // Skip internal chunk metadata
                content += `<p><strong>${key}:</strong> ${value}</p>`;
            }
        }
    }

    if (node.content) {
        content += '<h4>Content</h4>';
        content += `<pre>${node.content}</pre>`;
    }

    if (node.type === 'file') {
        content += `<p><a href="/api/nodes/${node.id}/content" target="_blank" download>Download Content</a></p>`;
    }

    panel.innerHTML = content;
    panel.style.display = 'block';
}

// Load graph on page load
document.addEventListener('DOMContentLoaded', loadGraph);
