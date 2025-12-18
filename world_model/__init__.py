"""
Memex World Model - Learned latent understanding of the knowledge graph.

The world model maintains a compressed representation (z_t) of the knowledge space
that enables prediction, simulation, and reasoning.

Architecture:
    Memex (Graph Memory) → Attention DAG (Learned Edges) → World Model (Latent State)

Components:
    - EntityEncoder: Encode graph nodes into vectors
    - GraphStateEncoder: Compress full graph state to z_t
    - DynamicsModel: Predict state transitions
    - WorldModel: Full model with prediction heads

Usage:
    from world_model import WorldModel, WorldModelConfig, WorldModelInference

    # Create model
    config = WorldModelConfig(hidden_dim=512)
    model = WorldModel(config)

    # For inference
    inference = WorldModelInference(model, config, checkpoint_path="model.pt")
    predictions = inference.predict_next_entities(top_k=10)
"""

from .config import WorldModelConfig, TrainingConfig
from .model import (
    EntityEncoder,
    GraphStateEncoder,
    DynamicsModel,
    WorldModel,
    create_world_model,
)
from .training import (
    ContrastiveLoss,
    LinkPredictionLoss,
    TemporalPredictionLoss,
    WorldModelTrainer,
    train_world_model,
)
from .attention_dag_integration import (
    AttentionDAGIntegration,
    MemexClient,
    AttentionEdge,
    GraphNode,
)
from .inference import (
    WorldModelInference,
    PredictionResult,
    SimulationStep,
    load_inference_model,
)

__version__ = "0.1.0"
__all__ = [
    # Config
    "WorldModelConfig",
    "TrainingConfig",
    # Model
    "EntityEncoder",
    "GraphStateEncoder",
    "DynamicsModel",
    "WorldModel",
    "create_world_model",
    # Training
    "ContrastiveLoss",
    "LinkPredictionLoss",
    "TemporalPredictionLoss",
    "WorldModelTrainer",
    "train_world_model",
    # Integration
    "AttentionDAGIntegration",
    "MemexClient",
    "AttentionEdge",
    "GraphNode",
    # Inference
    "WorldModelInference",
    "PredictionResult",
    "SimulationStep",
    "load_inference_model",
]
