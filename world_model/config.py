"""
Configuration for the Memex World Model.
"""

from dataclasses import dataclass, field
from typing import Optional
import os


@dataclass
class WorldModelConfig:
    """Configuration for the world model architecture and training."""

    # Model architecture
    hidden_dim: int = 512
    num_layers: int = 4
    num_heads: int = 8
    dropout: float = 0.1

    # Entity encoding
    max_entities: int = 100000  # Maximum number of entities to support
    num_entity_types: int = 50  # Number of distinct entity types
    type_embed_dim: int = 64
    content_embed_dim: int = 384  # From sentence transformer

    # Graph encoding
    max_edges: int = 500000  # Maximum attention edges
    edge_feature_dim: int = 16  # Features per edge (weight, type, etc.)

    # Dynamics model
    dynamics_type: str = "gru"  # "gru", "transformer", or "mlp"
    context_length: int = 32  # Number of past observations to consider

    # Training
    learning_rate: float = 1e-4
    weight_decay: float = 0.01
    batch_size: int = 64
    num_epochs: int = 100
    warmup_steps: int = 1000

    # Loss weights
    contrastive_weight: float = 1.0
    link_prediction_weight: float = 1.0
    temporal_weight: float = 0.5

    # Contrastive learning
    temperature: float = 0.07
    num_negatives: int = 64

    # Memex connection
    memex_api: str = field(default_factory=lambda: os.getenv("MEMEX_API", "http://localhost:8080"))
    neo4j_uri: str = field(default_factory=lambda: os.getenv("NEO4J_URI", "bolt://localhost:7687"))
    neo4j_user: str = "neo4j"
    neo4j_password: str = field(default_factory=lambda: os.getenv("NEO4J_PASSWORD", "password"))

    # Checkpointing
    checkpoint_dir: str = "checkpoints/world_model"
    save_every: int = 10  # Save every N epochs
    log_every: int = 100  # Log every N steps

    # Device
    device: str = "cuda"  # "cuda" or "cpu"

    # Inference
    min_attention_weight: float = 0.3  # Minimum weight for attention edges
    prediction_threshold: float = 0.7  # Threshold for link predictions

    def __post_init__(self):
        """Validate configuration."""
        assert self.hidden_dim % self.num_heads == 0, "hidden_dim must be divisible by num_heads"
        assert self.dynamics_type in ("gru", "transformer", "mlp"), f"Invalid dynamics_type: {self.dynamics_type}"


@dataclass
class TrainingConfig:
    """Configuration specific to training runs."""

    # Data
    train_split: float = 0.8
    val_split: float = 0.1
    test_split: float = 0.1

    # Early stopping
    patience: int = 10
    min_delta: float = 1e-4

    # Gradient clipping
    max_grad_norm: float = 1.0

    # Mixed precision
    use_amp: bool = True

    # Logging
    use_wandb: bool = False
    wandb_project: str = "memex-world-model"
    wandb_entity: Optional[str] = None

    # Reproducibility
    seed: int = 42


# Default configurations
DEFAULT_CONFIG = WorldModelConfig()
DEFAULT_TRAINING_CONFIG = TrainingConfig()
