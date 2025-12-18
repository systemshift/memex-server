"""
Bidirectional integration between World Model and Attention DAG.

The Attention DAG stores learned entity relationships as weighted edges.
The World Model:
- Reads from the DAG for training supervision
- Writes predictions back to propose new edges
"""

import json
from typing import List, Dict, Optional, Tuple, Any
from dataclasses import dataclass

import requests
import torch
from torch import Tensor
import numpy as np

from .config import WorldModelConfig
from .model import WorldModel


@dataclass
class AttentionEdge:
    """Represents an attention edge in the DAG."""
    source: str
    target: str
    weight: float
    query_count: int = 0
    last_updated: Optional[str] = None


@dataclass
class GraphNode:
    """Represents a node in the memex graph."""
    id: str
    type: str
    content: Optional[str] = None
    meta: Optional[Dict[str, Any]] = None


class MemexClient:
    """Client for interacting with the Memex API."""

    def __init__(self, api_url: str = "http://localhost:8080"):
        self.api_url = api_url.rstrip("/")

    def get_nodes(self, limit: int = 1000, node_type: Optional[str] = None) -> List[GraphNode]:
        """Get nodes from memex."""
        params = {"limit": limit}
        if node_type:
            params["type"] = node_type

        try:
            resp = requests.get(f"{self.api_url}/api/nodes", params=params, timeout=30)
            resp.raise_for_status()
            data = resp.json()

            nodes = []
            for node_id in data.get("nodes", []):
                node_data = self.get_node(node_id)
                if node_data:
                    nodes.append(node_data)
            return nodes
        except Exception as e:
            print(f"Error fetching nodes: {e}")
            return []

    def get_node(self, node_id: str) -> Optional[GraphNode]:
        """Get a single node by ID."""
        try:
            resp = requests.get(f"{self.api_url}/api/nodes/{node_id}", timeout=10)
            if resp.status_code == 200:
                data = resp.json()
                return GraphNode(
                    id=data.get("ID", node_id),
                    type=data.get("Type", "Unknown"),
                    content=data.get("Content"),
                    meta=data.get("Meta"),
                )
        except Exception as e:
            print(f"Error fetching node {node_id}: {e}")
        return None

    def get_attention_edges(self, min_weight: float = 0.0, limit: int = 10000) -> List[AttentionEdge]:
        """Get attention edges from the DAG."""
        try:
            resp = requests.get(
                f"{self.api_url}/api/query/attention_subgraph",
                params={"min_weight": min_weight, "limit": limit},
                timeout=30,
            )
            if resp.status_code == 200:
                data = resp.json()
                edges = []
                for edge in data.get("edges", []):
                    edges.append(AttentionEdge(
                        source=edge.get("source"),
                        target=edge.get("target"),
                        weight=edge.get("weight", 0.0),
                        query_count=edge.get("query_count", 0),
                        last_updated=edge.get("last_updated"),
                    ))
                return edges
        except Exception as e:
            print(f"Error fetching attention edges: {e}")
        return []

    def update_attention_edge(
        self,
        source: str,
        target: str,
        weight: float,
        query_id: Optional[str] = None,
    ) -> bool:
        """Create or update an attention edge."""
        try:
            resp = requests.post(
                f"{self.api_url}/api/edges/attention",
                json={
                    "source": source,
                    "target": target,
                    "weight": weight,
                    "query_id": query_id or "world_model",
                },
                timeout=10,
            )
            return resp.status_code in (200, 201)
        except Exception as e:
            print(f"Error updating attention edge: {e}")
            return False

    def search_nodes(self, query: str, limit: int = 50) -> List[GraphNode]:
        """Search for nodes by text."""
        try:
            resp = requests.get(
                f"{self.api_url}/api/query/search",
                params={"q": query, "limit": limit},
                timeout=10,
            )
            if resp.status_code == 200:
                data = resp.json()
                nodes = []
                for node_id in data.get("nodes", []):
                    node = self.get_node(node_id)
                    if node:
                        nodes.append(node)
                return nodes
        except Exception as e:
            print(f"Error searching nodes: {e}")
        return []


class AttentionDAGIntegration:
    """
    Manages bidirectional data flow between World Model and Attention DAG.
    """

    def __init__(
        self,
        world_model: WorldModel,
        config: WorldModelConfig,
        memex_client: Optional[MemexClient] = None,
    ):
        self.model = world_model
        self.config = config
        self.memex = memex_client or MemexClient(config.memex_api)

        # Entity mappings
        self.entity_to_idx: Dict[str, int] = {}
        self.idx_to_entity: Dict[int, str] = {}
        self.entity_types: Dict[str, str] = {}

        # Type mappings
        self.type_to_idx: Dict[str, int] = {"Unknown": 0}
        self.idx_to_type: Dict[int, str] = {0: "Unknown"}

    def load_entities(self, limit: int = 10000) -> int:
        """
        Load entities from memex and build mappings.

        Returns number of entities loaded.
        """
        nodes = self.memex.get_nodes(limit=limit)

        for i, node in enumerate(nodes):
            self.entity_to_idx[node.id] = i
            self.idx_to_entity[i] = node.id
            self.entity_types[node.id] = node.type

            if node.type not in self.type_to_idx:
                idx = len(self.type_to_idx)
                self.type_to_idx[node.type] = idx
                self.idx_to_type[idx] = node.type

        print(f"Loaded {len(nodes)} entities, {len(self.type_to_idx)} types")
        return len(nodes)

    def get_attention_dag_as_tensor(
        self,
        num_entities: Optional[int] = None,
    ) -> Tuple[Tensor, Tensor, Tensor]:
        """
        Convert attention DAG to tensors for training.

        Returns:
            entity_ids: [num_entities]
            entity_types: [num_entities]
            edge_weights: [num_entities, num_entities]
        """
        if num_entities is None:
            num_entities = len(self.entity_to_idx)

        # Get attention edges
        edges = self.memex.get_attention_edges(min_weight=self.config.min_attention_weight)

        # Build tensors
        entity_ids = torch.arange(num_entities)
        entity_types = torch.zeros(num_entities, dtype=torch.long)
        edge_weights = torch.zeros(num_entities, num_entities)

        # Fill entity types
        for entity_id, idx in self.entity_to_idx.items():
            if idx < num_entities:
                entity_type = self.entity_types.get(entity_id, "Unknown")
                type_idx = self.type_to_idx.get(entity_type, 0)
                entity_types[idx] = type_idx

        # Fill edge weights
        for edge in edges:
            src_idx = self.entity_to_idx.get(edge.source)
            tgt_idx = self.entity_to_idx.get(edge.target)
            if src_idx is not None and tgt_idx is not None:
                if src_idx < num_entities and tgt_idx < num_entities:
                    edge_weights[src_idx, tgt_idx] = edge.weight

        return entity_ids, entity_types, edge_weights

    def get_positive_pairs(self, min_weight: float = 0.3) -> List[Tuple[int, int, float]]:
        """
        Get positive pairs from attention edges for contrastive learning.

        Returns list of (source_idx, target_idx, weight) tuples.
        """
        edges = self.memex.get_attention_edges(min_weight=min_weight)

        pairs = []
        for edge in edges:
            src_idx = self.entity_to_idx.get(edge.source)
            tgt_idx = self.entity_to_idx.get(edge.target)
            if src_idx is not None and tgt_idx is not None:
                pairs.append((src_idx, tgt_idx, edge.weight))

        return pairs

    def sample_negative_pairs(
        self,
        num_samples: int,
        exclude_pairs: Optional[set] = None,
    ) -> List[Tuple[int, int]]:
        """
        Sample negative pairs (entities that aren't connected).

        Returns list of (source_idx, target_idx) tuples.
        """
        if exclude_pairs is None:
            exclude_pairs = set()

        num_entities = len(self.entity_to_idx)
        negatives = []

        while len(negatives) < num_samples:
            src = np.random.randint(0, num_entities)
            tgt = np.random.randint(0, num_entities)
            if src != tgt and (src, tgt) not in exclude_pairs:
                negatives.append((src, tgt))
                exclude_pairs.add((src, tgt))

        return negatives

    def propose_new_edges(
        self,
        z_t: Tensor,
        entity_embeds: Tensor,
        threshold: float = 0.7,
        max_proposals: int = 100,
    ) -> List[Tuple[str, str, float]]:
        """
        Use world model to propose new attention edges.

        Args:
            z_t: Current latent state
            entity_embeds: Entity embeddings from encoder
            threshold: Minimum confidence for proposals
            max_proposals: Maximum number of edges to propose

        Returns:
            List of (source_id, target_id, confidence) tuples
        """
        self.model.eval()

        proposals = []
        num_entities = entity_embeds.shape[0]

        with torch.no_grad():
            # Sample random pairs and predict links
            for _ in range(max_proposals * 10):  # Sample more than needed
                i = np.random.randint(0, num_entities)
                j = np.random.randint(0, num_entities)
                if i == j:
                    continue

                entity_i = entity_embeds[i].unsqueeze(0)
                entity_j = entity_embeds[j].unsqueeze(0)

                prob = self.model.predict_link(z_t, entity_i, entity_j).item()

                if prob >= threshold:
                    src_id = self.idx_to_entity.get(i)
                    tgt_id = self.idx_to_entity.get(j)
                    if src_id and tgt_id:
                        proposals.append((src_id, tgt_id, prob))

                if len(proposals) >= max_proposals:
                    break

        return proposals

    def write_proposals_to_dag(
        self,
        proposals: List[Tuple[str, str, float]],
        dry_run: bool = False,
    ) -> int:
        """
        Write proposed edges back to the attention DAG.

        Args:
            proposals: List of (source_id, target_id, confidence)
            dry_run: If True, don't actually write

        Returns:
            Number of edges written
        """
        written = 0

        for src_id, tgt_id, confidence in proposals:
            if dry_run:
                print(f"  [DRY RUN] Would write: {src_id} -> {tgt_id} (conf={confidence:.3f})")
            else:
                success = self.memex.update_attention_edge(
                    source=src_id,
                    target=tgt_id,
                    weight=confidence,
                    query_id="world_model_proposal",
                )
                if success:
                    written += 1

        return written

    def sync_with_dag(
        self,
        write_proposals: bool = True,
        proposal_threshold: float = 0.7,
    ) -> Dict[str, Any]:
        """
        Full sync cycle: read from DAG, update model, write predictions back.

        Returns stats about the sync.
        """
        stats = {}

        # 1. Load entities
        num_entities = self.load_entities()
        stats["num_entities"] = num_entities

        # 2. Get current state
        entity_ids, entity_types, edge_weights = self.get_attention_dag_as_tensor()
        stats["num_edges"] = (edge_weights > 0).sum().item()

        # 3. Encode current state
        self.model.eval()
        with torch.no_grad():
            z_t = self.model.encode(
                entity_ids.unsqueeze(0),
                entity_types.unsqueeze(0),
                edge_weights.unsqueeze(0),
            )

            # Get entity embeddings
            entity_embeds = self.model.encoder.entity_encoder(
                entity_ids.unsqueeze(0),
                entity_types.unsqueeze(0),
            ).squeeze(0)

        stats["latent_norm"] = z_t.norm().item()

        # 4. Propose new edges
        if write_proposals:
            proposals = self.propose_new_edges(
                z_t, entity_embeds,
                threshold=proposal_threshold,
            )
            stats["num_proposals"] = len(proposals)

            # Write back to DAG
            written = self.write_proposals_to_dag(proposals)
            stats["edges_written"] = written

        return stats
