# Memex LLM Extractor

Extracts entities and relationships from content using OpenAI.

## Setup

```bash
pip install -r requirements.txt
export OPENAI_API_KEY=your-key-here
export MEMEX_URL=http://localhost:8080  # optional
```

## Usage

Extract from text:
```bash
python extract.py "Alice met Bob in Paris to discuss the merger."
```

Extract from file:
```bash
python extract.py --file ./git-log.txt git-log
```

## What it does

1. **Ingest**: Stores raw content in Memex (content-addressed)
2. **Extract**: Uses OpenAI to extract entities and relationships
3. **Store**: Creates nodes and links in the graph

## Output

- Source node: `sha256:...` with raw content
- Entity nodes: Extracted entities with types
- Links: `extracted_from` + semantic relationships
- Transaction: Records the extraction operation
