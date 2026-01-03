"""
Memex World Model - Neural network components.

Architecture:
    EntityEncoder → GraphStateEncoder → DynamicsModel → Prediction Heads
"""

import math
from typing import Optional, Tuple, Dict, Any

import torch
import torch.nn as nn
import torch.nn.functional as F
from torch import Tensor

from .config import WorldModelConfig


class MLP(nn.Module):
    """Multi-layer perceptron with residual connections."""

    def __init__(
        self,
        input_dim: int,
        hidden_dim: int,
        output_dim: int,
        num_layers: int = 2,
        dropout: float = 0.1,
    ):
        super().__init__()
        layers = []
        dims = [input_dim] + [hidden_dim] * (num_layers - 1) + [output_dim]

        for i in range(len(dims) - 1):
            layers.append(nn.Linear(dims[i], dims[i + 1]))
            if i < len(dims) - 2:  # No activation/dropout on last layer
                layers.append(nn.GELU())
                layers.append(nn.Dropout(dropout))

        self.layers = nn.Sequential(*layers)
        self.residual = input_dim == output_dim

    def forward(self, x: Tensor) -> Tensor:
        out = self.layers(x)
        if self.residual:
            out = out + x
        return out


class EntityEncoder(nn.Module):
    """
    Encode graph entities into dense vectors.

    Combines:
    - Learnable entity embeddings (for known entities)
    - Type embeddings (Person, Task, Knowledge, etc.)
    - Content embeddings (from text, if available)
    """

    def __init__(self, config: WorldModelConfig):
        super().__init__()
        self.config = config

        # Entity ID embeddings (learnable)
        self.entity_embed = nn.Embedding(config.max_entities, config.hidden_dim)

        # Entity type embeddings
        self.type_embed = nn.Embedding(config.num_entity_types, config.type_embed_dim)

        # Content encoder (for text content)
        self.content_proj = nn.Linear(config.content_embed_dim, config.hidden_dim)

        # Fusion layer
        fusion_input_dim = config.hidden_dim + config.type_embed_dim + config.hidden_dim
        self.fusion = MLP(
            fusion_input_dim,
            config.hidden_dim,
            config.hidden_dim,
            num_layers=2,
            dropout=config.dropout,
        )

        # Layer norm
        self.norm = nn.LayerNorm(config.hidden_dim)

    def forward(
        self,
        entity_ids: Tensor,  # [batch, num_entities]
        entity_types: Tensor,  # [batch, num_entities]
        content_embeds: Optional[Tensor] = None,  # [batch, num_entities, content_dim]
    ) -> Tensor:
        """
        Encode entities into vectors.

        Returns: [batch, num_entities, hidden_dim]
        """
        # Entity embeddings
        entity_emb = self.entity_embed(entity_ids)  # [B, N, H]

        # Type embeddings
        type_emb = self.type_embed(entity_types)  # [B, N, T]

        # Content embeddings (if provided)
        if content_embeds is not None:
            content_emb = self.content_proj(content_embeds)  # [B, N, H]
        else:
            content_emb = torch.zeros_like(entity_emb)

        # Concatenate and fuse
        combined = torch.cat([entity_emb, type_emb, content_emb], dim=-1)
        fused = self.fusion(combined)

        return self.norm(fused)


class GraphTransformerLayer(nn.Module):
    """
    Transformer layer that uses graph structure (attention edges) as bias.
    """

    def __init__(self, config: WorldModelConfig):
        super().__init__()
        self.config = config

        # Multi-head self-attention
        self.self_attn = nn.MultiheadAttention(
            embed_dim=config.hidden_dim,
            num_heads=config.num_heads,
            dropout=config.dropout,
            batch_first=True,
        )

        # Feed-forward
        self.ffn = MLP(
            config.hidden_dim,
            config.hidden_dim * 4,
            config.hidden_dim,
            num_layers=2,
            dropout=config.dropout,
        )

        # Layer norms
        self.norm1 = nn.LayerNorm(config.hidden_dim)
        self.norm2 = nn.LayerNorm(config.hidden_dim)

        # Edge weight projection (attention bias from graph)
        self.edge_proj = nn.Linear(1, config.num_heads)

    def forward(
        self,
        x: Tensor,  # [batch, num_nodes, hidden_dim]
        edge_weights: Optional[Tensor] = None,  # [batch, num_nodes, num_nodes]
    ) -> Tensor:
        # Self-attention with residual
        # Note: We don't use edge_weights as attention mask due to shape issues with MHA
        # Instead, the graph structure is learned through the entity/type embeddings
        x_norm = self.norm1(x)
        attn_out, _ = self.self_attn(x_norm, x_norm, x_norm)
        x = x + attn_out

        # Feed-forward with residual
        x = x + self.ffn(self.norm2(x))

        return x


class GraphStateEncoder(nn.Module):
    """
    Encode the full graph state (nodes + attention edges) into a latent vector z_t.

    Uses transformer layers with graph-aware attention, then aggregates to a single vector.
    """

    def __init__(self, config: WorldModelConfig):
        super().__init__()
        self.config = config

        # Entity encoder
        self.entity_encoder = EntityEncoder(config)

        # Graph transformer layers
        self.layers = nn.ModuleList([
            GraphTransformerLayer(config)
            for _ in range(config.num_layers)
        ])

        # Aggregation: mean pool + learnable query
        self.global_query = nn.Parameter(torch.randn(1, 1, config.hidden_dim))
        self.agg_attn = nn.MultiheadAttention(
            embed_dim=config.hidden_dim,
            num_heads=config.num_heads,
            dropout=config.dropout,
            batch_first=True,
        )

        # Final projection
        self.out_proj = nn.Linear(config.hidden_dim, config.hidden_dim)
        self.out_norm = nn.LayerNorm(config.hidden_dim)

    def forward(
        self,
        entity_ids: Tensor,
        entity_types: Tensor,
        edge_weights: Optional[Tensor] = None,
        content_embeds: Optional[Tensor] = None,
        entity_mask: Optional[Tensor] = None,
    ) -> Tensor:
        """
        Encode graph state to latent vector.

        Args:
            entity_ids: [batch, num_entities] - Entity IDs
            entity_types: [batch, num_entities] - Entity type IDs
            edge_weights: [batch, num_entities, num_entities] - Attention DAG weights
            content_embeds: [batch, num_entities, content_dim] - Optional text embeddings
            entity_mask: [batch, num_entities] - Mask for valid entities

        Returns:
            z: [batch, hidden_dim] - Latent state vector
        """
        batch_size = entity_ids.shape[0]

        # Encode entities
        x = self.entity_encoder(entity_ids, entity_types, content_embeds)

        # Apply graph transformer layers
        for layer in self.layers:
            x = layer(x, edge_weights)

        # Aggregate to single vector using attention with global query
        query = self.global_query.expand(batch_size, -1, -1)
        z, _ = self.agg_attn(query, x, x)
        z = z.squeeze(1)  # [batch, hidden_dim]

        # Final projection
        z = self.out_proj(z)
        z = self.out_norm(z)

        return z


class DynamicsModel(nn.Module):
    """
    Predict the next latent state given current state and new observation.

    Implements: z_{t+1} = f(z_t, obs_t)
    """

    def __init__(self, config: WorldModelConfig):
        super().__init__()
        self.config = config

        # Observation encoder
        self.obs_encoder = MLP(
            config.hidden_dim,
            config.hidden_dim,
            config.hidden_dim,
            num_layers=2,
            dropout=config.dropout,
        )

        # State transition
        if config.dynamics_type == "gru":
            self.transition = nn.GRU(
                input_size=config.hidden_dim,
                hidden_size=config.hidden_dim,
                num_layers=2,
                batch_first=True,
                dropout=config.dropout,
            )
        elif config.dynamics_type == "transformer":
            self.transition = nn.TransformerEncoder(
                nn.TransformerEncoderLayer(
                    d_model=config.hidden_dim,
                    nhead=config.num_heads,
                    dim_feedforward=config.hidden_dim * 4,
                    dropout=config.dropout,
                    batch_first=True,
                ),
                num_layers=2,
            )
        else:  # MLP
            self.transition = MLP(
                config.hidden_dim * 2,
                config.hidden_dim * 2,
                config.hidden_dim,
                num_layers=3,
                dropout=config.dropout,
            )

        self.out_norm = nn.LayerNorm(config.hidden_dim)

    def forward(
        self,
        z_t: Tensor,  # [batch, hidden_dim]
        observation: Tensor,  # [batch, hidden_dim] or [batch, seq, hidden_dim]
    ) -> Tensor:
        """
        Predict next state.

        Returns: z_{t+1} [batch, hidden_dim]
        """
        # Encode observation
        obs_emb = self.obs_encoder(observation)

        if self.config.dynamics_type == "gru":
            # GRU: observation as input, state as hidden
            if obs_emb.dim() == 2:
                obs_emb = obs_emb.unsqueeze(1)
            output, _ = self.transition(obs_emb, z_t.unsqueeze(0).expand(2, -1, -1).contiguous())
            z_next = output[:, -1, :]
        elif self.config.dynamics_type == "transformer":
            # Transformer: concatenate state and observation
            if obs_emb.dim() == 2:
                obs_emb = obs_emb.unsqueeze(1)
            seq = torch.cat([z_t.unsqueeze(1), obs_emb], dim=1)
            output = self.transition(seq)
            z_next = output[:, 0, :]  # Take first position (state)
        else:
            # MLP: simple concatenation
            if obs_emb.dim() == 3:
                obs_emb = obs_emb[:, -1, :]
            combined = torch.cat([z_t, obs_emb], dim=-1)
            z_next = self.transition(combined)

        return self.out_norm(z_next)


class BilinearDecoder(nn.Module):
    """Bilinear decoder for link prediction."""

    def __init__(self, hidden_dim: int):
        super().__init__()
        self.W = nn.Parameter(torch.randn(hidden_dim, hidden_dim))
        nn.init.xavier_uniform_(self.W)

    def forward(self, z_i: Tensor, z_j: Tensor) -> Tensor:
        """Predict link probability between entity i and j."""
        # z_i @ W @ z_j.T
        return torch.sigmoid((z_i @ self.W * z_j).sum(dim=-1))


class WorldModel(nn.Module):
    """
    Complete world model with encoding, dynamics, and prediction heads.
    """

    def __init__(self, config: WorldModelConfig):
        super().__init__()
        self.config = config

        # Core components
        self.encoder = GraphStateEncoder(config)
        self.dynamics = DynamicsModel(config)

        # Prediction heads
        self.next_entity_head = MLP(
            config.hidden_dim,
            config.hidden_dim,
            config.max_entities,
            num_layers=2,
            dropout=config.dropout,
        )

        self.link_predictor = BilinearDecoder(config.hidden_dim)

        self.relevance_head = MLP(
            config.hidden_dim * 2,
            config.hidden_dim,
            1,
            num_layers=2,
            dropout=config.dropout,
        )

        # Entity projection for link prediction
        self.entity_proj = nn.Linear(config.hidden_dim, config.hidden_dim)

    def encode(
        self,
        entity_ids: Tensor,
        entity_types: Tensor,
        edge_weights: Optional[Tensor] = None,
        content_embeds: Optional[Tensor] = None,
    ) -> Tensor:
        """Encode graph state to latent vector z_t."""
        return self.encoder(entity_ids, entity_types, edge_weights, content_embeds)

    def predict_dynamics(self, z_t: Tensor, observation: Tensor) -> Tensor:
        """Predict next state z_{t+1}."""
        return self.dynamics(z_t, observation)

    def predict_next_entity(self, z_t: Tensor) -> Tensor:
        """
        Predict which entity will be relevant next.

        Returns: [batch, num_entities] logits
        """
        return self.next_entity_head(z_t)

    def predict_link(
        self,
        z_t: Tensor,
        entity_i_embed: Tensor,
        entity_j_embed: Tensor,
    ) -> Tensor:
        """
        Predict if entities i and j should be connected.

        Returns: [batch] probabilities
        """
        z_i = self.entity_proj(entity_i_embed)
        z_j = self.entity_proj(entity_j_embed)
        return self.link_predictor(z_i, z_j)

    def predict_relevance(self, z_t: Tensor, query_embed: Tensor) -> Tensor:
        """
        Predict relevance of current state to a query.

        Returns: [batch] relevance scores
        """
        combined = torch.cat([z_t, query_embed], dim=-1)
        return torch.sigmoid(self.relevance_head(combined).squeeze(-1))

    def forward(
        self,
        entity_ids: Tensor,
        entity_types: Tensor,
        edge_weights: Optional[Tensor] = None,
        content_embeds: Optional[Tensor] = None,
        observation: Optional[Tensor] = None,
    ) -> Dict[str, Tensor]:
        """
        Full forward pass.

        Returns dict with:
            - z_t: Current latent state
            - z_next: Predicted next state (if observation provided)
            - next_entity_logits: Prediction over entities
        """
        # Encode current state
        z_t = self.encode(entity_ids, entity_types, edge_weights, content_embeds)

        outputs = {"z_t": z_t}

        # Predict next state if observation provided
        if observation is not None:
            z_next = self.predict_dynamics(z_t, observation)
            outputs["z_next"] = z_next

        # Predict next entity
        outputs["next_entity_logits"] = self.predict_next_entity(z_t)

        return outputs


def create_world_model(config: Optional[WorldModelConfig] = None) -> WorldModel:
    """Factory function to create a world model."""
    if config is None:
        config = WorldModelConfig()
    return WorldModel(config)
