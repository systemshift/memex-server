# Wikipedia Ingestion: Ready to Test

## ✅ What We Have

### Architecture Support
- **Content-addressed storage**: Each revision gets unique SHA256
- **Version links**: `revision_v1 → previous_version → revision_v2`
- **Cross-page links**: `wiki:Python → links_to → wiki:Java`
- **Temporal data**: Timestamps on all nodes and relationships
- **Transaction log**: Full audit trail

### Implementation Ready
- **Wikipedia API client**: Fetches pages + revision history
- **Wikitext parser**: Extracts `[[hyperlinks]]` and categories
- **LLM extraction**: Extracts entities and relationships
- **Batch ingestion**: Process multiple pages sequentially
- **Rate limiting**: Respects API limits

### What Gets Ingested

For each Wikipedia page:
1. **Source nodes** (one per revision)
   - Content-addressed (SHA256)
   - Raw wikitext
   - Metadata: timestamp, editor, revision ID

2. **WikiPage node** (ontology layer)
   - Page title, ID
   - Categories
   - Hyperlinks
   - Latest revision ID

3. **Entity nodes** (LLM extracted)
   - People, concepts, technologies
   - Relationships between entities

4. **Links**
   - Source → version_of → WikiPage
   - Source → previous_version → Source (temporal chain)
   - WikiPage → links_to → WikiPage (hyperlinks)
   - WikiPage → mentions → Entity (extracted concepts)

## 📊 Test Plan

### Phase 1: Small Test (3 pages, 5 revisions each)
```bash
cd extractor
python3 wikipedia_ingest.py
```

**Test pages:**
- Python (programming language)
- Artificial intelligence
- Graph database

**Expected:**
- ~15 Source nodes (3 pages × 5 revisions)
- ~3 WikiPage nodes
- ~30-50 Entity nodes (extracted)
- ~100-200 Links

**Cost estimate:** ~$0.15 (LLM extraction only)

### Phase 2: Medium Scale (100 pages, 10 revisions each)
**Target pages:**
- Top 100 most-edited pages (controversial, current events)
- Programming languages
- CS concepts

**Expected:**
- ~1,000 Source nodes
- ~100 WikiPage nodes
- ~1,000-2,000 Entity nodes
- ~5,000-10,000 Links

**Cost estimate:** ~$5 extraction + API calls

### Phase 3: Large Scale (1,000+ pages)
- Wait for Phase 2 results
- Measure query performance
- Validate storage efficiency
- Compare vs RAG baseline

## 🎯 Test Queries

Once ingested, test with MCP agent:

### Temporal Queries
"How has the definition of Python changed over the last 5 revisions?"

### Cross-Page Queries
"What programming languages does the Python article link to?"

### Concept Evolution
"When was 'type hints' first mentioned in the Python article?"

### Entity Relationships
"What entities are mentioned in both Python and Java articles?"

### Controversy Detection
"Which pages have the most revisions? (find edit wars)"

## 💰 Cost Comparison

### RAG Approach (baseline)
- Embed all revisions: 1,000 revisions × $0.0001 = $0.10 embedding
- Re-embed on every update
- Query: Re-embed query + vector search + LLM context = $0.05-0.10 per query
- 100 queries: $5-10

### Memex Approach
- Extract once: 100 pages × $0.05 = $5 (upfront)
- Query: Direct graph traversal = $0.001 per query (no embedding)
- 100 queries: $0.10

**Break-even:** ~100 queries
**Long-term savings:** 10-50x cheaper per query

## 🚀 Run It Now

```bash
# 1. Start Memex server (if not running)
./memex-server

# 2. Run Wikipedia ingestion
cd extractor
export OPENAI_API_KEY=your_key_here
python3 wikipedia_ingest.py

# 3. Query the results
curl 'http://localhost:8080/api/query/filter?type=WikiPage'
curl 'http://localhost:8080/api/query/search?q=Python'
curl 'http://localhost:8080/api/query/traverse?start=wiki:Python_(programming_language)&depth=2'

# 4. Or use MCP with Claude Desktop
# (see mcp-server/README.md for setup)
```

## 📈 Why Wikipedia is a Great Test

1. **Version control**: Real edit history (temporal graph)
2. **Cross-references**: Rich hyperlink structure
3. **Structured data**: Categories, infoboxes
4. **Controversial topics**: Edit wars, reverts
5. **Public dataset**: Easy to benchmark against
6. **Real-world queries**: "How has X changed?" "What's related to Y?"

## 🎁 Bonus Queries Enabled

With Wikipedia in Memex, you can answer:

1. **Historical**: "What did people think about AI in 2020 vs 2024?"
2. **Influence**: "What are the most-linked CS concepts?"
3. **Controversy**: "Which articles have most edit wars?"
4. **Evolution**: "When did 'machine learning' first appear?"
5. **Cross-reference**: "What do Python and Ruby articles both mention?"
6. **Author activity**: "What topics does this editor focus on?"

These queries are **impossible** with traditional RAG because RAG has no:
- Temporal understanding (no version history)
- Graph structure (no relationships)
- Efficient updates (must re-embed everything)

## Next Step

Ready to test? Run `python3 extractor/wikipedia_ingest.py`
