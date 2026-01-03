"""
Inference utilities for the Memex World Model.

Provides high-level functions for:
- Encoding the current knowledge state
- Predicting relevant entities
- Simulating future states
- Querying the world model
"""

from typing import List, Dict, Optional, Any, Tuple
from dataclasses import dataclass

import torch
from torch import Tensor
import torch.nn.functional as F

from .config import WorldModelConfig
from .model import WorldModel, create_world_model
from .attention_dag_integration import AttentionDAGIntegration, MemexClient


@dataclass
class PredictionResult:
    """Result of a world model prediction."""
    entity_id: str
    entity_type: str
    score: float
    reasoning: Optional[str] = None


@dataclass
class SimulationStep:
    """A single step in a simulation trajectory."""
    step: int
    latent_state: Tensor
    top_entities: List[PredictionResult]
    state_change: float  # How much z changed


class WorldModelInference:
    """
    High-level inference interface for the world model.
    """

    def __init__(
        self,
        model: WorldModel,
        config: WorldModelConfig,
        checkpoint_path: Optional[str] = None,
    ):
        self.model = model
        self.config = config

        # Setup device
        self.device = torch.device(config.device if torch.cuda.is_available() else "cpu")
        self.model.to(self.device)
        self.model.eval()

        # Load checkpoint if provided
        if checkpoint_path:
            self.load_checkpoint(checkpoint_path)

        # Setup memex integration
        self.integration = AttentionDAGIntegration(model, config)

        # Cache for current state
        self._current_z: Optional[Tensor] = None
        self._entity_embeds: Optional[Tensor] = None

    def load_checkpoint(self, path: str):
        """Load model weights from checkpoint."""
        checkpoint = torch.load(path, map_location=self.device, weights_only=False)
        self.model.load_state_dict(checkpoint["model_state_dict"])
        print(f"Loaded checkpoint from {path}")

    def refresh_state(self) -> Tensor:
        """
        Refresh the world state from current memex data.

        Returns the new latent state z_t.
        """
        # Load latest entities
        self.integration.load_entities()

        # Get tensors
        entity_ids, entity_types, edge_weights = self.integration.get_attention_dag_as_tensor()

        # Move to device
        entity_ids = entity_ids.unsqueeze(0).to(self.device)
        entity_types = entity_types.unsqueeze(0).to(self.device)
        edge_weights = edge_weights.unsqueeze(0).to(self.device)

        # Encode
        with torch.no_grad():
            self._current_z = self.model.encode(entity_ids, entity_types, edge_weights)
            self._entity_embeds = self.model.encoder.entity_encoder(
                entity_ids, entity_types
            ).squeeze(0)

        return self._current_z

    def get_current_state(self) -> Tensor:
        """Get current latent state, refreshing if needed."""
        if self._current_z is None:
            self.refresh_state()
        return self._current_z

    def predict_next_entities(
        self,
        top_k: int = 10,
        refresh: bool = False,
    ) -> List[PredictionResult]:
        """
        Predict which entities will be relevant next.

        Args:
            top_k: Number of top predictions to return
            refresh: Whether to refresh state from memex first

        Returns:
            List of predictions sorted by score
        """
        if refresh or self._current_z is None:
            self.refresh_state()

        with torch.no_grad():
            logits = self.model.predict_next_entity(self._current_z)
            probs = F.softmax(logits, dim=-1).squeeze(0)

            # Get top k
            top_probs, top_indices = probs.topk(top_k)

        results = []
        for prob, idx in zip(top_probs.tolist(), top_indices.tolist()):
            entity_id = self.integration.idx_to_entity.get(idx, f"entity_{idx}")
            entity_type = self.integration.entity_types.get(entity_id, "Unknown")
            results.append(PredictionResult(
                entity_id=entity_id,
                entity_type=entity_type,
                score=prob,
            ))

        return results

    def predict_link_probability(
        self,
        entity_i: str,
        entity_j: str,
    ) -> float:
        """
        Predict the probability that two entities should be linked.

        Args:
            entity_i: First entity ID
            entity_j: Second entity ID

        Returns:
            Probability between 0 and 1
        """
        if self._current_z is None or self._entity_embeds is None:
            self.refresh_state()

        idx_i = self.integration.entity_to_idx.get(entity_i)
        idx_j = self.integration.entity_to_idx.get(entity_j)

        if idx_i is None or idx_j is None:
            return 0.0

        with torch.no_grad():
            embed_i = self._entity_embeds[idx_i].unsqueeze(0)
            embed_j = self._entity_embeds[idx_j].unsqueeze(0)
            prob = self.model.predict_link(self._current_z, embed_i, embed_j)

        return prob.item()

    def find_related_entities(
        self,
        entity_id: str,
        top_k: int = 10,
        min_similarity: float = 0.5,
    ) -> List[PredictionResult]:
        """
        Find entities most related to a given entity.

        Uses embedding similarity in the learned space.
        """
        if self._entity_embeds is None:
            self.refresh_state()

        idx = self.integration.entity_to_idx.get(entity_id)
        if idx is None:
            return []

        with torch.no_grad():
            # Get query embedding
            query_embed = self._entity_embeds[idx]

            # Compute similarities to all entities
            similarities = F.cosine_similarity(
                query_embed.unsqueeze(0),
                self._entity_embeds,
                dim=-1,
            )

            # Mask self
            similarities[idx] = -1.0

            # Get top k
            top_sims, top_indices = similarities.topk(top_k)

        results = []
        for sim, idx in zip(top_sims.tolist(), top_indices.tolist()):
            if sim < min_similarity:
                continue
            entity_id = self.integration.idx_to_entity.get(idx, f"entity_{idx}")
            entity_type = self.integration.entity_types.get(entity_id, "Unknown")
            results.append(PredictionResult(
                entity_id=entity_id,
                entity_type=entity_type,
                score=sim,
            ))

        return results

    def simulate_trajectory(
        self,
        num_steps: int = 5,
        observation_source: str = "self",  # "self" or "random"
    ) -> List[SimulationStep]:
        """
        Simulate the world model forward for multiple steps.

        This is useful for planning and exploring possible futures.

        Args:
            num_steps: Number of simulation steps
            observation_source: Where to get observations
                - "self": Use current state as observation (self-prediction)
                - "random": Random observations

        Returns:
            List of simulation steps
        """
        if self._current_z is None:
            self.refresh_state()

        trajectory = []
        z = self._current_z.clone()

        for step in range(num_steps):
            with torch.no_grad():
                # Get observation
                if observation_source == "self":
                    obs = z
                else:
                    obs = torch.randn_like(z)

                # Predict next state
                z_next = self.model.predict_dynamics(z, obs)

                # Measure state change
                state_change = (z_next - z).norm().item()

                # Get top predicted entities at this step
                logits = self.model.predict_next_entity(z_next)
                probs = F.softmax(logits, dim=-1).squeeze(0)
                top_probs, top_indices = probs.topk(5)

                top_entities = []
                for prob, idx in zip(top_probs.tolist(), top_indices.tolist()):
                    entity_id = self.integration.idx_to_entity.get(idx, f"entity_{idx}")
                    entity_type = self.integration.entity_types.get(entity_id, "Unknown")
                    top_entities.append(PredictionResult(
                        entity_id=entity_id,
                        entity_type=entity_type,
                        score=prob,
                    ))

                trajectory.append(SimulationStep(
                    step=step,
                    latent_state=z_next.clone(),
                    top_entities=top_entities,
                    state_change=state_change,
                ))

                z = z_next

        return trajectory

    def query_relevance(
        self,
        query_text: str,
        top_k: int = 10,
    ) -> List[PredictionResult]:
        """
        Find entities most relevant to a text query.

        Note: Requires a text encoder (sentence transformer).
        For now, falls back to search.
        """
        # Fall back to memex search for now
        nodes = self.integration.memex.search_nodes(query_text, limit=top_k)

        results = []
        for i, node in enumerate(nodes):
            # Score decreases with rank
            score = 1.0 / (i + 1)
            results.append(PredictionResult(
                entity_id=node.id,
                entity_type=node.type,
                score=score,
            ))

        return results

    def get_state_summary(self) -> Dict[str, Any]:
        """
        Get a summary of the current world state.
        """
        if self._current_z is None:
            self.refresh_state()

        # Get top predictions
        top_entities = self.predict_next_entities(top_k=5)

        return {
            "num_entities": len(self.integration.entity_to_idx),
            "num_types": len(self.integration.type_to_idx),
            "latent_dim": self._current_z.shape[-1],
            "latent_norm": self._current_z.norm().item(),
            "top_predicted_entities": [
                {"id": e.entity_id, "type": e.entity_type, "score": e.score}
                for e in top_entities
            ],
        }


def load_inference_model(
    checkpoint_path: str,
    config: Optional[WorldModelConfig] = None,
) -> WorldModelInference:
    """
    Load a trained world model for inference.

    Args:
        checkpoint_path: Path to model checkpoint
        config: Model configuration (loaded from checkpoint if not provided)

    Returns:
        WorldModelInference instance ready for predictions
    """
    if config is None:
        config = WorldModelConfig()

    model = create_world_model(config)
    inference = WorldModelInference(model, config, checkpoint_path)

    return inference
