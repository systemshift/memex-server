"""
Training utilities for the Memex World Model.

Implements:
- Contrastive loss (InfoNCE)
- Link prediction loss
- Temporal prediction loss (JEPA-style)
- Training loop
"""

import os
from typing import Optional, Dict, List, Tuple, Any
from dataclasses import dataclass

import torch
import torch.nn as nn
import torch.nn.functional as F
from torch import Tensor
from torch.utils.data import DataLoader, Dataset
from torch.optim import AdamW
from torch.optim.lr_scheduler import CosineAnnealingWarmRestarts

from .config import WorldModelConfig, TrainingConfig
from .model import WorldModel


class ContrastiveLoss(nn.Module):
    """
    InfoNCE contrastive loss.

    Pulls together embeddings of connected entities (positive pairs from attention DAG),
    pushes apart embeddings of unconnected entities (negative samples).
    """

    def __init__(self, temperature: float = 0.07):
        super().__init__()
        self.temperature = temperature

    def forward(
        self,
        anchor: Tensor,      # [batch, hidden_dim]
        positive: Tensor,    # [batch, hidden_dim]
        negatives: Tensor,   # [batch, num_negatives, hidden_dim]
    ) -> Tensor:
        """
        Compute InfoNCE loss.

        Args:
            anchor: Anchor embeddings
            positive: Positive pair embeddings (connected via attention edge)
            negatives: Negative samples (random entities)

        Returns:
            loss: Scalar loss value
        """
        # Normalize embeddings
        anchor = F.normalize(anchor, dim=-1)
        positive = F.normalize(positive, dim=-1)
        negatives = F.normalize(negatives, dim=-1)

        # Positive similarity: [batch]
        pos_sim = (anchor * positive).sum(dim=-1) / self.temperature

        # Negative similarities: [batch, num_negatives]
        neg_sim = torch.bmm(negatives, anchor.unsqueeze(-1)).squeeze(-1) / self.temperature

        # InfoNCE: log(exp(pos) / (exp(pos) + sum(exp(neg))))
        logits = torch.cat([pos_sim.unsqueeze(-1), neg_sim], dim=-1)  # [batch, 1 + num_neg]
        labels = torch.zeros(anchor.shape[0], dtype=torch.long, device=anchor.device)

        return F.cross_entropy(logits, labels)


class LinkPredictionLoss(nn.Module):
    """
    Binary cross-entropy loss for link prediction.

    Trains the model to predict whether two entities should be connected.
    """

    def __init__(self):
        super().__init__()

    def forward(
        self,
        predictions: Tensor,  # [batch] - predicted probabilities
        labels: Tensor,       # [batch] - ground truth (0 or 1)
        weights: Optional[Tensor] = None,  # [batch] - edge weights for soft labels
    ) -> Tensor:
        """
        Compute link prediction loss.
        """
        if weights is not None:
            # Use edge weights as soft labels
            labels = labels.float() * weights

        return F.binary_cross_entropy(predictions, labels.float())


class TemporalPredictionLoss(nn.Module):
    """
    JEPA-style temporal prediction loss.

    Predicts future latent states, comparing in latent space (not observation space).
    """

    def __init__(self):
        super().__init__()

    def forward(
        self,
        z_pred: Tensor,    # [batch, hidden_dim] - predicted next state
        z_target: Tensor,  # [batch, hidden_dim] - actual next state (from encoder)
    ) -> Tensor:
        """
        Compute temporal prediction loss (MSE in latent space).
        """
        # Normalize to unit sphere for stability
        z_pred = F.normalize(z_pred, dim=-1)
        z_target = F.normalize(z_target, dim=-1)

        # Cosine similarity loss (1 - similarity)
        similarity = (z_pred * z_target).sum(dim=-1)
        return (1 - similarity).mean()


@dataclass
class TrainingBatch:
    """A batch of training data."""
    entity_ids: Tensor
    entity_types: Tensor
    edge_weights: Optional[Tensor]
    content_embeds: Optional[Tensor]

    # For contrastive learning
    positive_pairs: Optional[Tensor]  # [batch, 2] - indices of positive pairs
    negative_indices: Optional[Tensor]  # [batch, num_neg] - indices of negatives

    # For link prediction
    link_pairs: Optional[Tensor]  # [batch, 2] - entity pairs
    link_labels: Optional[Tensor]  # [batch] - 0/1 labels
    link_weights: Optional[Tensor]  # [batch] - attention weights

    # For temporal prediction
    next_entity_ids: Optional[Tensor]
    next_entity_types: Optional[Tensor]
    next_edge_weights: Optional[Tensor]


class WorldModelTrainer:
    """
    Trainer for the world model.
    """

    def __init__(
        self,
        model: WorldModel,
        config: WorldModelConfig,
        training_config: TrainingConfig,
    ):
        self.model = model
        self.config = config
        self.training_config = training_config

        # Move model to device
        self.device = torch.device(config.device if torch.cuda.is_available() else "cpu")
        self.model.to(self.device)

        # Loss functions
        self.contrastive_loss = ContrastiveLoss(config.temperature)
        self.link_loss = LinkPredictionLoss()
        self.temporal_loss = TemporalPredictionLoss()

        # Optimizer
        self.optimizer = AdamW(
            model.parameters(),
            lr=config.learning_rate,
            weight_decay=config.weight_decay,
        )

        # Scheduler
        self.scheduler = CosineAnnealingWarmRestarts(
            self.optimizer,
            T_0=10,
            T_mult=2,
        )

        # Mixed precision
        self.scaler = torch.cuda.amp.GradScaler() if training_config.use_amp else None

        # Tracking
        self.global_step = 0
        self.best_loss = float("inf")

    def train_step(self, batch: TrainingBatch) -> Dict[str, float]:
        """
        Single training step.

        Returns dict of loss values.
        """
        self.model.train()
        self.optimizer.zero_grad()

        # Move batch to device
        entity_ids = batch.entity_ids.to(self.device)
        entity_types = batch.entity_types.to(self.device)
        edge_weights = batch.edge_weights.to(self.device) if batch.edge_weights is not None else None
        content_embeds = batch.content_embeds.to(self.device) if batch.content_embeds is not None else None

        losses = {}
        total_loss = 0.0

        with torch.cuda.amp.autocast(enabled=self.training_config.use_amp):
            # Forward pass
            outputs = self.model(
                entity_ids=entity_ids,
                entity_types=entity_types,
                edge_weights=edge_weights,
                content_embeds=content_embeds,
            )
            z_t = outputs["z_t"]

            # 1. Contrastive loss
            if batch.positive_pairs is not None and batch.negative_indices is not None:
                # Get embeddings for positive pairs
                pos_pairs = batch.positive_pairs.to(self.device)
                neg_indices = batch.negative_indices.to(self.device)

                # Get entity embeddings from encoder
                entity_embeds = self.model.encoder.entity_encoder(
                    entity_ids, entity_types, content_embeds
                )

                # Gather anchor, positive, negative embeddings
                batch_indices = torch.arange(entity_ids.shape[0], device=self.device)
                anchor = entity_embeds[batch_indices, pos_pairs[:, 0]]
                positive = entity_embeds[batch_indices, pos_pairs[:, 1]]
                negatives = entity_embeds[batch_indices.unsqueeze(1), neg_indices]

                loss_contrastive = self.contrastive_loss(anchor, positive, negatives)
                losses["contrastive"] = loss_contrastive.item()
                total_loss = total_loss + self.config.contrastive_weight * loss_contrastive

            # 2. Link prediction loss
            if batch.link_pairs is not None and batch.link_labels is not None:
                link_pairs = batch.link_pairs.to(self.device)
                link_labels = batch.link_labels.to(self.device)
                link_weights = batch.link_weights.to(self.device) if batch.link_weights is not None else None

                # Get entity embeddings
                entity_embeds = self.model.encoder.entity_encoder(
                    entity_ids, entity_types, content_embeds
                )

                batch_indices = torch.arange(entity_ids.shape[0], device=self.device)
                entity_i = entity_embeds[batch_indices, link_pairs[:, 0]]
                entity_j = entity_embeds[batch_indices, link_pairs[:, 1]]

                link_pred = self.model.predict_link(z_t, entity_i, entity_j)
                loss_link = self.link_loss(link_pred, link_labels, link_weights)
                losses["link_prediction"] = loss_link.item()
                total_loss = total_loss + self.config.link_prediction_weight * loss_link

            # 3. Temporal prediction loss
            if batch.next_entity_ids is not None:
                next_ids = batch.next_entity_ids.to(self.device)
                next_types = batch.next_entity_types.to(self.device)
                next_edges = batch.next_edge_weights.to(self.device) if batch.next_edge_weights is not None else None

                # Encode next state (target)
                with torch.no_grad():
                    z_target = self.model.encode(next_ids, next_types, next_edges)

                # Predict next state
                z_pred = self.model.predict_dynamics(z_t, z_t)  # Self-prediction for now

                loss_temporal = self.temporal_loss(z_pred, z_target)
                losses["temporal"] = loss_temporal.item()
                total_loss = total_loss + self.config.temporal_weight * loss_temporal

        losses["total"] = total_loss.item()

        # Backward pass
        if self.scaler is not None:
            self.scaler.scale(total_loss).backward()
            self.scaler.unscale_(self.optimizer)
            torch.nn.utils.clip_grad_norm_(self.model.parameters(), self.training_config.max_grad_norm)
            self.scaler.step(self.optimizer)
            self.scaler.update()
        else:
            total_loss.backward()
            torch.nn.utils.clip_grad_norm_(self.model.parameters(), self.training_config.max_grad_norm)
            self.optimizer.step()

        self.scheduler.step()
        self.global_step += 1

        return losses

    def train_epoch(self, dataloader: DataLoader) -> Dict[str, float]:
        """
        Train for one epoch.

        Returns average losses.
        """
        epoch_losses: Dict[str, List[float]] = {}

        for batch in dataloader:
            losses = self.train_step(batch)

            for k, v in losses.items():
                if k not in epoch_losses:
                    epoch_losses[k] = []
                epoch_losses[k].append(v)

            # Log
            if self.global_step % self.config.log_every == 0:
                avg_loss = sum(epoch_losses.get("total", [0])) / max(len(epoch_losses.get("total", [1])), 1)
                print(f"Step {self.global_step}: loss={avg_loss:.4f}")

        # Average losses
        return {k: sum(v) / len(v) for k, v in epoch_losses.items()}

    def save_checkpoint(self, path: str, epoch: int):
        """Save model checkpoint."""
        os.makedirs(os.path.dirname(path), exist_ok=True)
        torch.save({
            "epoch": epoch,
            "global_step": self.global_step,
            "model_state_dict": self.model.state_dict(),
            "optimizer_state_dict": self.optimizer.state_dict(),
            "scheduler_state_dict": self.scheduler.state_dict(),
            "config": self.config,
            "best_loss": self.best_loss,
        }, path)
        print(f"Saved checkpoint to {path}")

    def load_checkpoint(self, path: str) -> int:
        """Load model checkpoint. Returns epoch number."""
        checkpoint = torch.load(path, map_location=self.device)
        self.model.load_state_dict(checkpoint["model_state_dict"])
        self.optimizer.load_state_dict(checkpoint["optimizer_state_dict"])
        self.scheduler.load_state_dict(checkpoint["scheduler_state_dict"])
        self.global_step = checkpoint["global_step"]
        self.best_loss = checkpoint.get("best_loss", float("inf"))
        print(f"Loaded checkpoint from {path} (epoch {checkpoint['epoch']})")
        return checkpoint["epoch"]


def train_world_model(
    model: WorldModel,
    train_dataloader: DataLoader,
    val_dataloader: Optional[DataLoader] = None,
    config: Optional[WorldModelConfig] = None,
    training_config: Optional[TrainingConfig] = None,
    resume_from: Optional[str] = None,
) -> WorldModel:
    """
    Main training function.

    Args:
        model: WorldModel instance
        train_dataloader: Training data
        val_dataloader: Validation data (optional)
        config: Model config
        training_config: Training config
        resume_from: Path to checkpoint to resume from

    Returns:
        Trained model
    """
    if config is None:
        config = WorldModelConfig()
    if training_config is None:
        training_config = TrainingConfig()

    trainer = WorldModelTrainer(model, config, training_config)

    start_epoch = 0
    if resume_from is not None:
        start_epoch = trainer.load_checkpoint(resume_from)

    print(f"Training world model for {config.num_epochs} epochs...")
    print(f"Device: {trainer.device}")

    for epoch in range(start_epoch, config.num_epochs):
        print(f"\nEpoch {epoch + 1}/{config.num_epochs}")

        # Train
        train_losses = trainer.train_epoch(train_dataloader)
        print(f"Train losses: {train_losses}")

        # Validate
        if val_dataloader is not None:
            val_losses = trainer.train_epoch(val_dataloader)  # TODO: proper eval mode
            print(f"Val losses: {val_losses}")

            # Save best model
            if val_losses["total"] < trainer.best_loss:
                trainer.best_loss = val_losses["total"]
                trainer.save_checkpoint(
                    os.path.join(config.checkpoint_dir, "best.pt"),
                    epoch,
                )

        # Save periodic checkpoint
        if (epoch + 1) % config.save_every == 0:
            trainer.save_checkpoint(
                os.path.join(config.checkpoint_dir, f"epoch_{epoch + 1}.pt"),
                epoch,
            )

    # Save final model
    trainer.save_checkpoint(
        os.path.join(config.checkpoint_dir, "final.pt"),
        config.num_epochs,
    )

    return model
