# Memex HTTP API Documentation

This document describes the HTTP API endpoints provided by the memexd server.

## Base URL

All endpoints are relative to: `http://localhost:3000/`

## Content Types

- File uploads use `multipart/form-data`
- Form submissions use `application/x-www-form-urlencoded`
- Responses are HTML pages (web interface)

## DAG Structure

The API operates on a Directed Acyclic Graph (DAG) where:
- Nodes represent files and notes
- Edges represent typed relationships between nodes
- Each node can have multiple versions
- Links maintain the acyclic property

## Endpoints

### View DAG
```
GET /
```

View the DAG structure, including nodes and their relationships.

**Response:**
- HTML page showing:
  - All nodes in the graph
  - Their relationships
  - Version information
  - Metadata

### Add Node
```
POST /add
Content-Type: multipart/form-data
```

Add a new node to the DAG.

**Parameters:**
- `file`: The file to upload (form field)

**Response:**
- Redirects to `/` on success
- Error page on failure

**Notes:**
- Creates a new node in the DAG
- Stores content as a blob
- Generates unique node ID
- Records metadata (filename, timestamp)

### Delete Node
```
POST /delete
Content-Type: application/x-www-form-urlencoded
```

Delete a node from the DAG.

**Parameters:**
- `id`: Node ID to delete

**Response:**
- Redirects to `/` on success
- Error page on failure

**Notes:**
- Removes node from DAG
- Maintains graph integrity
- Updates affected relationships

### Create Link
```
POST /link
Content-Type: application/x-www-form-urlencoded
```

Create a directed edge between nodes.

**Parameters:**
- `source`: Source node ID
- `target`: Target node ID
- `type`: Link type
- `note`: Optional note about the link

**Response:**
- Redirects to `/` on success
- Error page on failure

**Notes:**
- Creates directed relationship
- Validates acyclic property
- Stores link metadata

### Search
```
GET /search
```

Search the DAG based on query parameters.

**Parameters:**
- Query parameters are converted to search criteria

**Response:**
- HTML page showing:
  - Matching nodes
  - Their relationships
  - Path information

## Error Handling

All errors result in:
- HTTP error status code
- Error page with message

Common HTTP status codes:
- 400: Bad Request - Invalid input
- 404: Not Found - Node doesn't exist
- 405: Method Not Allowed - Wrong HTTP method
- 500: Internal Server Error - Server-side error

## Examples

### Adding a Node
```html
<form action="/add" method="post" enctype="multipart/form-data">
  <input type="file" name="file">
  <button type="submit">Upload</button>
</form>
```

### Creating a Link
```html
<form action="/link" method="post">
  <input type="hidden" name="source" value="abc123">
  <input type="hidden" name="target" value="def456">
  <input type="text" name="type" value="reference">
  <input type="text" name="note" value="Important connection">
  <button type="submit">Create Link</button>
</form>
```

### Searching
```
GET /search?type=file&filename=document.txt
```

## Future Enhancements

Planned improvements:
- Graph visualization endpoints
- Path finding between nodes
- Batch operations
- Advanced graph queries
- WebSocket updates for graph changes
- Version control operations
