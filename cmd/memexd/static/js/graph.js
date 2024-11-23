// Create a network
var container = document.getElementById('graph');
var nodeInfo = document.getElementById('nodeInfo');
var nodeTitle = document.getElementById('nodeTitle');
var nodeType = document.getElementById('nodeType');
var nodeContent = document.getElementById('nodeContent');
var nodeLinks = document.getElementById('nodeLinks');

// Load graph data from API
fetch('/api/graph')
    .then(response => response.json())
    .then(data => {
        // Create nodes with different colors based on type
        var nodes = new vis.DataSet(data.nodes.map(node => ({
            id: node.id,
            label: node.meta.filename || 'Note',
            color: {
                background: node.type === 'file' ? '#e3f2fd' : '#fce4ec',
                border: node.type === 'file' ? '#1976d2' : '#e91e63',
                highlight: {
                    background: node.type === 'file' ? '#bbdefb' : '#f8bbd0',
                    border: node.type === 'file' ? '#1565c0' : '#c2185b'
                }
            },
            font: {
                color: '#333',
                size: 14
            },
            data: node
        })));

        // Create edges with labels
        var edges = new vis.DataSet(data.edges.map(edge => ({
            from: edge.source,
            to: edge.target,
            label: edge.type,
            arrows: 'to',
            color: {
                color: '#666',
                highlight: '#333'
            },
            font: {
                size: 12,
                color: '#666',
                strokeWidth: 0,
                background: 'white'
            },
            data: edge
        })));

        // Create network
        var network = new vis.Network(container, {
            nodes: nodes,
            edges: edges
        }, {
            nodes: {
                shape: 'dot',
                size: 20,
                borderWidth: 2,
                shadow: true
            },
            edges: {
                width: 2,
                smooth: {
                    type: 'continuous'
                },
                shadow: true
            },
            physics: {
                enabled: true,
                barnesHut: {
                    gravitationalConstant: -2000,
                    centralGravity: 0.3,
                    springLength: 150,
                    springConstant: 0.04,
                    damping: 0.09
                },
                stabilization: {
                    iterations: 100,
                    updateInterval: 50
                }
            },
            interaction: {
                hover: true,
                tooltipDelay: 200,
                zoomView: true,
                dragView: true
            },
            layout: {
                improvedLayout: true,
                hierarchical: {
                    enabled: false
                }
            }
        });

        // Show node info on click
        network.on('click', function(params) {
            if (params.nodes.length > 0) {
                var nodeId = params.nodes[0];
                var node = nodes.get(nodeId);
                
                // Show node info panel
                nodeTitle.textContent = node.label;
                nodeType.textContent = node.data.type;
                
                // Load content if available
                if (node.data.meta.content) {
                    fetch(`/api/content/${node.data.meta.content}`)
                        .then(response => response.text())
                        .then(content => {
                            nodeContent.textContent = content;
                        });
                } else {
                    nodeContent.textContent = 'No content available';
                }

                // Show links
                var nodeEdges = edges.get({
                    filter: edge => edge.from === nodeId || edge.to === nodeId
                });
                nodeLinks.innerHTML = nodeEdges.map(edge => {
                    var direction = edge.from === nodeId ? 'to' : 'from';
                    var otherNode = nodes.get(direction === 'to' ? edge.to : edge.from);
                    var note = edge.data.meta.note ? ` (${edge.data.meta.note})` : '';
                    return `<div onclick="network.selectNodes(['${direction === 'to' ? edge.to : edge.from}'])">${direction} ${otherNode.label} [${edge.label}]${note}</div>`;
                }).join('');

                nodeInfo.style.display = 'block';
            } else {
                nodeInfo.style.display = 'none';
            }
        });

        // Double click to open file/note
        network.on('doubleClick', function(params) {
            if (params.nodes.length > 0) {
                var nodeId = params.nodes[0];
                window.location.href = `/node/${nodeId}`;
            }
        });

        // Stabilize the network
        network.once('stabilizationIterationsDone', function() {
            network.setOptions({ physics: false });
            document.getElementById('physics').classList.remove('active');
        });

        // Toolbar actions
        document.getElementById('zoomIn').onclick = function() {
            network.zoomIn();
        };
        document.getElementById('zoomOut').onclick = function() {
            network.zoomOut();
        };
        document.getElementById('fitGraph').onclick = function() {
            network.fit();
        };
        document.getElementById('physics').onclick = function() {
            var physics = !network.physics.options.enabled;
            network.setOptions({ physics: { enabled: physics } });
            this.classList.toggle('active', physics);
        };
    });
