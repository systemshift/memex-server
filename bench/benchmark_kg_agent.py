#!/usr/bin/env python3
"""
Agent-Based Knowledge Graph Retrieval Benchmark

Instead of fixed traversal, an LLM agent iteratively explores the graph.
The agent has tools to search, explore, and decide when it has enough context.

Usage:
    python benchmark_kg_agent.py --limit 20
"""

import argparse
import hashlib
import json
import os
import time
from pathlib import Path

from datasets import load_from_disk
from dotenv import load_dotenv
from neo4j import GraphDatabase
from openai import OpenAI

load_dotenv()

DATA_DIR = Path(__file__).parent / "data"
RESULTS_FILE = Path(__file__).parent / "results_kg_agent.json"

NEO4J_URI = "bolt://localhost:7687"
NEO4J_USER = "neo4j"
NEO4J_PASSWORD = "password"

MODEL = "gpt-4o-mini"
MAX_ITERATIONS = 8


def make_source_id(title: str, content: str) -> str:
    return "source:" + hashlib.sha256((title + content).encode()).hexdigest()[:16]


def get_ground_truth(example: dict) -> set[str]:
    relevant_titles = set(example["supporting_facts"]["title"])
    relevant_ids = set()
    for title, sents in zip(example["context"]["title"], example["context"]["sentences"]):
        if title in relevant_titles:
            content = " ".join(sents)
            relevant_ids.add(make_source_id(title, content))
    return relevant_ids


class GraphTools:
    """Tools the agent can use to explore the knowledge graph."""

    def __init__(self, driver):
        self.driver = driver

    def search_entities(self, query: str, limit: int = 10) -> list[dict]:
        """Search for entities by name. Returns entity IDs and names."""
        with self.driver.session() as s:
            result = s.run("""
                MATCH (n:Node)
                WHERE n.type <> 'Source'
                AND toLower(n.properties) CONTAINS toLower($search_term)
                RETURN n.id as id, n.type as type, n.properties as props
                LIMIT $lim
            """, search_term=query, lim=limit)
            entities = []
            for r in result:
                props = json.loads(r["props"]) if r["props"] else {}
                entities.append({
                    "id": r["id"],
                    "type": r["type"],
                    "name": props.get("name", r["id"])
                })
            return entities

    def get_entity_relationships(self, entity_id: str) -> list[dict]:
        """Get relationships for an entity."""
        with self.driver.session() as s:
            result = s.run("""
                MATCH (e:Node {id: $id})-[r:LINK]-(other:Node)
                WHERE other.type <> 'Source'
                RETURN other.id as id, other.type as type, r.type as rel_type,
                       other.properties as props
                LIMIT 15
            """, id=entity_id)
            rels = []
            for r in result:
                props = json.loads(r["props"]) if r["props"] else {}
                rels.append({
                    "entity_id": r["id"],
                    "entity_type": r["type"],
                    "relationship": r["rel_type"],
                    "name": props.get("name", r["id"])
                })
            return rels

    def get_entity_sources(self, entity_id: str) -> list[str]:
        """Get source document IDs linked to an entity."""
        with self.driver.session() as s:
            result = s.run("""
                MATCH (e:Node {id: $id})-[:LINK {type: 'EXTRACTED_FROM'}]->(s:Node {type: 'Source'})
                RETURN s.id as id
            """, id=entity_id)
            return [r["id"] for r in result]

    def get_tools_schema(self) -> list[dict]:
        """Return OpenAI function calling schema."""
        return [
            {
                "type": "function",
                "function": {
                    "name": "search_entities",
                    "description": "Search for entities (people, places, organizations, concepts) by name in the knowledge graph",
                    "parameters": {
                        "type": "object",
                        "properties": {
                            "query": {
                                "type": "string",
                                "description": "Search term (entity name or partial name)"
                            }
                        },
                        "required": ["query"]
                    }
                }
            },
            {
                "type": "function",
                "function": {
                    "name": "get_relationships",
                    "description": "Get entities related to a specific entity via relationships",
                    "parameters": {
                        "type": "object",
                        "properties": {
                            "entity_id": {
                                "type": "string",
                                "description": "The entity ID to explore"
                            }
                        },
                        "required": ["entity_id"]
                    }
                }
            },
            {
                "type": "function",
                "function": {
                    "name": "get_sources",
                    "description": "Get source document IDs for an entity",
                    "parameters": {
                        "type": "object",
                        "properties": {
                            "entity_id": {
                                "type": "string",
                                "description": "The entity ID to get sources for"
                            }
                        },
                        "required": ["entity_id"]
                    }
                }
            },
            {
                "type": "function",
                "function": {
                    "name": "submit_answer",
                    "description": "Submit final list of relevant source document IDs. Call this when you have found enough sources to answer the question.",
                    "parameters": {
                        "type": "object",
                        "properties": {
                            "source_ids": {
                                "type": "array",
                                "items": {"type": "string"},
                                "description": "List of source document IDs that contain relevant information"
                            }
                        },
                        "required": ["source_ids"]
                    }
                }
            }
        ]


class AgentRetriever:
    """LLM agent that explores the knowledge graph."""

    def __init__(self, driver, openai_client):
        self.tools = GraphTools(driver)
        self.openai = openai_client

    def retrieve(self, question: str, verbose: bool = False) -> set[str]:
        """Agent explores graph and returns source IDs."""

        messages = [
            {
                "role": "system",
                "content": """You are a research agent exploring a knowledge graph to find relevant source documents.

Your goal: Find source documents that contain information needed to answer the question.

Strategy:
1. Search for key entities mentioned in the question
2. Explore relationships to find connected entities
3. Get source documents for relevant entities
4. When you have sources for all key aspects of the question, submit your answer

Be efficient - don't explore unnecessarily. Submit your answer once you have relevant sources."""
            },
            {
                "role": "user",
                "content": f"Find source documents relevant to this question: {question}"
            }
        ]

        collected_sources = set()

        for iteration in range(MAX_ITERATIONS):
            response = self.openai.chat.completions.create(
                model=MODEL,
                messages=messages,
                tools=self.tools.get_tools_schema(),
                tool_choice="auto",
                temperature=0,
            )

            msg = response.choices[0].message

            # No tool call = agent is done (shouldn't happen with good prompt)
            if not msg.tool_calls:
                if verbose:
                    print(f"  Iteration {iteration}: No tool call, ending")
                break

            # Process tool calls
            messages.append(msg)

            for tool_call in msg.tool_calls:
                fn_name = tool_call.function.name
                fn_args = json.loads(tool_call.function.arguments)

                if verbose:
                    print(f"  Iteration {iteration}: {fn_name}({fn_args})")

                # Execute tool
                if fn_name == "search_entities":
                    result = self.tools.search_entities(fn_args["query"])
                elif fn_name == "get_relationships":
                    result = self.tools.get_entity_relationships(fn_args["entity_id"])
                elif fn_name == "get_sources":
                    result = self.tools.get_entity_sources(fn_args["entity_id"])
                    collected_sources.update(result)
                elif fn_name == "submit_answer":
                    # Agent is done
                    final_sources = set(fn_args.get("source_ids", []))
                    if verbose:
                        print(f"  Agent submitted {len(final_sources)} sources")
                    return final_sources
                else:
                    result = {"error": f"Unknown function: {fn_name}"}

                messages.append({
                    "role": "tool",
                    "tool_call_id": tool_call.id,
                    "content": json.dumps(result)
                })

        # Max iterations reached - return what we collected
        if verbose:
            print(f"  Max iterations reached, returning {len(collected_sources)} collected sources")
        return collected_sources


def calc_metrics(retrieved: set, truth: set) -> dict:
    if not retrieved or not truth:
        return {"precision": 0, "recall": 0, "f1": 0}
    tp = len(retrieved & truth)
    p = tp / len(retrieved) if retrieved else 0
    r = tp / len(truth) if truth else 0
    f1 = 2 * p * r / (p + r) if (p + r) > 0 else 0
    return {"precision": p, "recall": r, "f1": f1}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--limit", type=int, default=20, help="Number of questions (agent is slower)")
    parser.add_argument("--start", type=int, default=0)
    parser.add_argument("--split", default="validation")
    parser.add_argument("--verbose", action="store_true")
    args = parser.parse_args()

    if not os.environ.get("OPENAI_API_KEY"):
        print("Error: OPENAI_API_KEY not set")
        return

    print("Loading dataset...")
    ds = load_from_disk(str(DATA_DIR))
    dataset = ds[args.split]

    driver = GraphDatabase.driver(NEO4J_URI, auth=(NEO4J_USER, NEO4J_PASSWORD))
    retriever = AgentRetriever(driver, OpenAI())

    metrics = {"precision": 0, "recall": 0, "f1": 0, "count": 0}
    start_time = time.time()

    end_idx = min(args.start + args.limit, len(dataset))
    print(f"Testing {args.start} to {end_idx} ({end_idx - args.start} questions)")
    print("Note: Agent-based retrieval is slower due to multiple LLM calls per question\n")

    for i in range(args.start, end_idx):
        ex = dataset[i]
        truth = get_ground_truth(ex)
        if not truth:
            continue

        if args.verbose:
            print(f"\nQ{i}: {ex['question']}")

        retrieved = retriever.retrieve(ex["question"], verbose=args.verbose)
        m = calc_metrics(retrieved, truth)

        metrics["precision"] += m["precision"]
        metrics["recall"] += m["recall"]
        metrics["f1"] += m["f1"]
        metrics["count"] += 1

        if not args.verbose:
            n = metrics["count"]
            print(f"Q{i}: P={m['precision']:.2f} R={m['recall']:.2f} F1={m['f1']:.2f} | "
                  f"Avg P={metrics['precision']/n:.3f} R={metrics['recall']/n:.3f} F1={metrics['f1']/n:.3f}")

    driver.close()

    n = max(metrics["count"], 1)
    results = {
        "precision": metrics["precision"] / n,
        "recall": metrics["recall"] / n,
        "f1": metrics["f1"] / n,
        "questions": metrics["count"],
        "method": "agent-based",
        "model": MODEL,
        "max_iterations": MAX_ITERATIONS,
    }

    with open(RESULTS_FILE, "w") as f:
        json.dump(results, f, indent=2)

    elapsed = time.time() - start_time
    print(f"\n{'='*50}")
    print(f"AGENT KG RESULTS ({results['questions']} questions)")
    print(f"{'='*50}")
    print(f"Precision: {results['precision']:.3f}")
    print(f"Recall:    {results['recall']:.3f}")
    print(f"F1:        {results['f1']:.3f}")
    print(f"Time:      {elapsed:.1f}s ({elapsed/results['questions']:.1f}s per question)")
    print(f"Saved:     {RESULTS_FILE}")


if __name__ == "__main__":
    main()
