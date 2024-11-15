# Memex HTTP API Documentation

This document describes the HTTP API endpoints provided by the memexd server.

## Base URL

All API URLs are relative to: `http://localhost:8080/api/`

## Content Types

- Request bodies should be `application/json` unless uploading files
- File uploads should use `multipart/form-data`
- Responses will be `application/json`

## Authentication

Currently, the API does not require authentication. This may change in future versions.

## Endpoints

### Add Content
```
POST /add
Content-Type: multipart/form-data
```

Upload a file to store in the system.

**Parameters:**
- `file`: The file to upload

**Response:**
```json
{
    "id": "abcd1234..."
}
```

### Create Link
```
POST /link
Content-Type: application/json
```

Create a link between two pieces of content.

**Request Body:**
```json
{
    "source": "source-id",
    "target": "target-id",
    "type": "link-type",
    "meta": {
        "note": "Optional link metadata"
    }
}
```

**Response:**
- Status: 201 Created

### Search
```
POST /search
Content-Type: application/json
```

Search for content based on criteria.

**Request Body:**
```json
{
    "type": "document",
    "tags": ["important"],
    "created_after": "2024-01-01T00:00:00Z"
}
```

**Response:**
```json
{
    "results": [
        {
            "id": "abcd1234...",
            "type": "document",
            "metadata": {
                "tags": ["important"],
                "created": "2024-01-15T10:30:00Z"
            }
        }
    ]
}
```

## Error Responses

All error responses follow this format:

```json
{
    "error": "Error message describing what went wrong"
}
```

Common HTTP status codes:
- 400: Bad Request - Invalid input
- 404: Not Found - Resource doesn't exist
- 500: Internal Server Error - Server-side error

## Rate Limiting

Currently, there are no rate limits implemented. This may change in future versions.

## Examples

### Adding a File
```bash
curl -X POST http://localhost:8080/api/add \
  -F "file=@document.pdf"
```

### Creating a Link
```bash
curl -X POST http://localhost:8080/api/link \
  -H "Content-Type: application/json" \
  -d '{
    "source": "abc123",
    "target": "def456",
    "type": "reference",
    "meta": {
      "note": "Important connection"
    }
  }'
```

### Searching
```bash
curl -X POST http://localhost:8080/api/search \
  -H "Content-Type: application/json" \
  -d '{
    "type": "document",
    "tags": ["important"]
  }'
```

## Future Endpoints

Planned for future releases:
- Content versioning
- Batch operations
- Advanced querying
- User management
- Access control
