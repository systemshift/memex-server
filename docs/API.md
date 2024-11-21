# Memex HTTP API Documentation

This document describes the HTTP API endpoints provided by the memexd server.

## Base URL

All endpoints are relative to: `http://localhost:3000/`

## Content Types

- File uploads use `multipart/form-data`
- Form submissions use `application/x-www-form-urlencoded`
- Responses are HTML pages (web interface)

## Endpoints

### View Repository
```
GET /
```

View the repository contents, including files, notes, and their links.

**Response:**
- HTML page showing repository contents

### Add File
```
POST /add
Content-Type: multipart/form-data
```

Upload a file to the repository.

**Parameters:**
- `file`: The file to upload (form field)

**Response:**
- Redirects to `/` on success
- Error page on failure

### Delete Node
```
POST /delete
Content-Type: application/x-www-form-urlencoded
```

Delete a node from the repository.

**Parameters:**
- `id`: Node ID to delete

**Response:**
- Redirects to `/` on success
- Error page on failure

### Create Link
```
POST /link
Content-Type: application/x-www-form-urlencoded
```

Create a link between two nodes.

**Parameters:**
- `source`: Source node ID
- `target`: Target node ID
- `type`: Link type
- `note`: Optional note about the link

**Response:**
- Redirects to `/` on success
- Error page on failure

### Search
```
GET /search
```

Search for nodes based on query parameters.

**Parameters:**
- Query parameters are converted to search criteria

**Response:**
- HTML page showing search results

## Error Handling

All errors result in:
- HTTP error status code
- Error page with message

Common HTTP status codes:
- 400: Bad Request - Invalid input
- 404: Not Found - Resource doesn't exist
- 405: Method Not Allowed - Wrong HTTP method
- 500: Internal Server Error - Server-side error

## Examples

### Adding a File
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
- JSON API endpoints
- Authentication
- Batch operations
- WebSocket updates
- Content versioning
- Advanced search options
