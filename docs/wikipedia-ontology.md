# Wikipedia Ontology for Memex

## Overview

Wikipedia has rich structure that maps well to Memex's layered architecture:
- **Source Layer**: Page versions (content-addressed)
- **Ontology Layer**: Pages, categories, people, concepts
- **Transaction Layer**: Edits, revisions, authorship

## Node Types

### Source Nodes
```
ID: sha256:abc123...
Type: Source
Content: Raw wikitext or parsed text
Meta:
  format: "wikipedia"
  page_title: "Python (programming language)"
  page_id: 23862
  revision_id: 1234567890
  timestamp: "2024-01-15T10:30:00Z"
  editor: "User:JohnDoe"
  size_bytes: 45000
```

### WikiPage Nodes
```
ID: "wiki:Python_(programming_language)"
Type: WikiPage
Meta:
  title: "Python (programming language)"
  page_id: 23862
  namespace: 0
  latest_revision: 1234567890
  categories: ["Programming languages", "Python"]
  infobox_type: "programming language"
```

### Person Nodes (from extraction)
```
ID: "Guido van Rossum"
Type: Person
Meta:
  extracted_from: "wiki:Python_(programming_language)"
  occupation: "programmer"
```

### Concept Nodes
```
ID: "object-oriented-programming"
Type: Concept
Meta:
  label: "Object-oriented programming"
  extracted_from: "wiki:Python_(programming_language)"
```

## Link Types

### Version Links (temporal)
```
sha256:abc... --[version_of]--> wiki:Python_page
sha256:def... --[version_of]--> wiki:Python_page
sha256:abc... --[previous_version]--> sha256:def...
sha256:def... --[next_version]--> sha256:abc...
```

### Content Links (extracted)
```
wiki:Python_page --[mentions]--> "Guido van Rossum"
wiki:Python_page --[implements]--> "object-oriented-programming"
wiki:Python_page --[influenced_by]--> wiki:C_language
```

### Hyperlinks (from wikitext)
```
wiki:Python_page --[links_to]--> wiki:Java_page
wiki:Python_page --[links_to]--> wiki:Ruby_page
```

### Category Links
```
wiki:Python_page --[in_category]--> wiki:Category:Programming_languages
```

### Edit Links
```
Transaction:edit_2024... --[edited]--> wiki:Python_page
Transaction:edit_2024... --[by_editor]--> "User:JohnDoe"
Transaction:edit_2024... --[created_version]--> sha256:abc...
```

## Query Patterns

### 1. Page Evolution
"How has the Python article changed over time?"
```cypher
MATCH (page:WikiPage {title: "Python (programming language)"})
MATCH (source:Source)-[:version_of]->(page)
RETURN source.timestamp, source.size_bytes
ORDER BY source.timestamp
```

### 2. Cross-Page Connections
"What programming languages does Python link to?"
```cypher
MATCH (python:WikiPage {title: "Python (programming language)"})
      -[:links_to]->(other:WikiPage)
      -[:in_category]->(cat:WikiPage {title: "Category:Programming languages"})
RETURN other.title
```

### 3. Edit History
"Who has edited the Python article?"
```cypher
MATCH (tx:Transaction)-[:edited]->(page:WikiPage {title: "Python (programming language)"})
MATCH (tx)-[:by_editor]->(editor)
RETURN editor, tx.timestamp, tx.size_change
ORDER BY tx.timestamp DESC
```

### 4. Concept Evolution
"When was 'type hints' first mentioned in the Python article?"
```cypher
MATCH (source:Source)-[:version_of]->(page:WikiPage {title: "Python (programming language)"})
WHERE source.content CONTAINS "type hints"
RETURN MIN(source.timestamp) as first_mention
```

### 5. Cross-Language Influence
"What languages influenced Python, according to Wikipedia?"
```cypher
MATCH (python:WikiPage {title: "Python (programming language)"})
      -[:influenced_by]->(lang:WikiPage)
RETURN lang.title
```

## Data Model Benefits

### ✅ Version Control
- Each edit creates new Source node (SHA256)
- Temporal links preserve history
- Can diff between versions
- Transaction log tracks all changes

### ✅ Deduplication
- Identical page reverts use same SHA256
- No storage waste for reverted vandalism

### ✅ Cross-Page Links
- Hyperlinks preserved as relationships
- Category structure maintained
- Concept graph extracted by LLM

### ✅ Temporal Queries
- Filter by timestamp range
- Track concept emergence
- Analyze edit patterns

## Implementation Notes

### Ingestion Pipeline
```
Wikipedia API → Raw Page Data → Memex Ingest
    ↓
1. Create Source node (sha256 of content)
2. Parse wikitext → extract hyperlinks
3. LLM extraction → entities and relationships
4. Create WikiPage node + links
5. Record Transaction
```

### Rate Limiting
- Wikipedia API: 200 requests/second max
- LLM extraction: batch pages, use cheap model
- Estimate: ~$0.01 per page (including hyperlinks)

### Scaling
- 100 pages: $1 extraction cost, ~2-5 MB storage
- 1,000 pages: $10, ~20-50 MB
- 10,000 pages: $100, ~200-500 MB
- 1M pages (10% of Wikipedia): $10,000, ~20-50 GB

### Start Small
1. Top 100 most-edited pages (controversial topics, current events)
2. Extract all revisions for last year
3. Build temporal query patterns
4. Measure token savings vs RAG

## Test Questions

Once ingested, we can answer:

1. **Evolution**: "How has the definition of 'AI' changed over time?"
2. **Influence**: "What are the most-linked programming languages?"
3. **Controversy**: "Which pages have the most reverts?"
4. **Cross-reference**: "What do articles about Python and Java both mention?"
5. **Emergence**: "When did 'machine learning' first appear in CS articles?"

## Advantages Over RAG

**RAG approach:**
- Embed every page version → huge vector DB
- No temporal understanding
- No cross-page relationships
- Re-embed on every Wikipedia update

**Memex approach:**
- Content-addressed versions (dedup)
- Temporal graph (edit history preserved)
- Cross-page relationships (hyperlinks, categories, concepts)
- Incremental updates (only new edits need extraction)
- Query once, answer many times
