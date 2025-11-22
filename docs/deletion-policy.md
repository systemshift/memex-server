# Deletion Policy: Maintaining DAG Integrity

## Problem

Naive deletion breaks:
1. **Version chains**: Temporal history becomes fragmented
2. **Content-addressed integrity**: SHA256 nodes should be immutable
3. **DAG structure**: Creates orphaned nodes and broken references

## Solution: Layered Deletion Policy

### Layer 1: Source Layer (Content-Addressed)
**Policy: IMMUTABLE - No deletion allowed**

- Source nodes are identified by `sha256:...` (content hash)
- Deletion violates content-addressed storage model
- Forms the immutable foundation of the knowledge graph

**Rationale:**
- Hash = Content (immutable mathematical property)
- Multiple nodes may reference same source
- Version history depends on source integrity

**Alternative:** Soft delete with tombstone (mark as deleted, keep in DB)

### Layer 2: Ontology Layer (Semantic Nodes)
**Policy: SAFE DELETE - Allowed with validation**

- WikiPage, Person, Concept, Entity nodes
- Can be deleted if not part of critical paths
- Must validate impact before deletion

**Safe to delete:**
- ✅ Isolated nodes (no relationships)
- ✅ Leaf nodes (no incoming critical links)
- ✅ Extracted entities (Person, Concept from LLM)
- ✅ Cross-page wiki links (can be regenerated)

**Unsafe to delete (requires cascade):**
- ❌ Nodes in version chains (breaks temporal history)
- ❌ Hub nodes with many dependents
- ❌ Source-layer references

### Layer 3: Relationship Layer
**Policy: SAFE DELETE - Allowed**

- Links between nodes can be removed
- Does not break content integrity
- May break graph traversal paths (acceptable)

**Safe examples:**
- Remove bad LLM extraction link
- Delete incorrect cross-reference
- Clean up duplicate relationships

## Implementation Strategy

### Option 1: Tombstone Pattern (Soft Delete)
```
Node {
  ID: "sha256:abc..."
  Type: "Source"
  Deleted: true  // Marked as deleted, still in DB
  DeletedAt: "2024-01-15T10:00:00Z"
}
```

**Pros:**
- Maintains DAG integrity
- Preserves version history
- Can "undelete" if needed
- Audit trail intact

**Cons:**
- Database grows (never truly removed)
- Queries must filter deleted nodes
- More complex query logic

### Option 2: Layer-Aware Deletion
```
DELETE /api/nodes/{id}
  → If Source layer: REJECT (or tombstone)
  → If Ontology layer: ALLOW (with validation)
  → If orphans created: WARN or CASCADE

DELETE /api/nodes/{id}?force=true
  → Bypass protection (admin only)
```

**Pros:**
- Clear deletion semantics per layer
- Protects critical infrastructure
- Flexible for different use cases

**Cons:**
- More complex API
- Users need to understand layers

### Option 3: Reference Counting
```
Before delete:
  1. Count incoming links to node
  2. If count > threshold: REJECT
  3. If count == 0: ALLOW
  4. If critical path (version chain): REJECT
```

**Pros:**
- Automatic protection of important nodes
- Simple user experience
- Prevents accidental damage

**Cons:**
- Doesn't protect content-addressed layer
- "Important" is subjective

## Recommended Approach

**Hybrid: Layer-Aware + Tombstone for Sources**

1. **Source nodes (SHA256)**: Soft delete only (tombstone)
2. **Ontology nodes**: Hard delete with validation
3. **Links**: Always safe to delete

### API Changes

```bash
# Source nodes - soft delete only
DELETE /api/nodes/sha256:abc...
→ Response: {"deleted": false, "tombstoned": true, "reason": "Source layer is immutable"}

# Ontology nodes - safe delete
DELETE /api/nodes/wiki:Python
→ Response: {"deleted": true} (if safe)
→ Error: "Cannot delete: 15 nodes depend on this" (if unsafe)

# Force delete (bypass protection)
DELETE /api/nodes/{id}?force=true
→ Response: {"deleted": true, "orphaned_nodes": 5, "broken_chains": 2}
```

## Migration Path

1. **Phase 1**: Add tombstone field to nodes
2. **Phase 2**: Update delete logic to check node type
3. **Phase 3**: Add validation (count dependencies)
4. **Phase 4**: Update queries to filter deleted=true

## Example Scenarios

### Scenario 1: Delete Bad LLM Extraction
```
User: Delete entity "gibberish-entity"
System: ✅ Safe - isolated entity node, no dependencies
Action: Hard delete
```

### Scenario 2: Delete Wikipedia Revision
```
User: Delete sha256:abc... (old Wikipedia revision)
System: ❌ Unsafe - Source layer, part of version chain
Action: Soft delete (tombstone)
```

### Scenario 3: Delete Test Data
```
User: Delete wiki:TestPage?cascade=true
System: ⚠️  Warning - Will orphan 5 revision nodes
Action: Tombstone all Source nodes, hard delete ontology nodes
```

## Query Impact

With tombstones, queries must filter:

```cypher
// Before
MATCH (n:Node) WHERE n.id = $id RETURN n

// After
MATCH (n:Node) WHERE n.id = $id AND (n.deleted IS NULL OR n.deleted = false) RETURN n
```

Add index:
```cypher
CREATE INDEX node_deleted_index IF NOT EXISTS FOR (n:Node) ON (n.deleted)
```

## Conclusion

**Current implementation is too aggressive and breaks DAG integrity.**

**Recommended fix:**
1. Protect Source layer (content-addressed = immutable)
2. Use tombstones for critical nodes
3. Add validation before deletion
4. Update queries to handle soft deletes

This maintains:
- ✅ Content-addressed integrity
- ✅ Version history chains
- ✅ DAG structure
- ✅ Ability to clean up bad data

**Trade-off:** More complexity, but preserves graph integrity.
