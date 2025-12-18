#!/usr/bin/env python3
"""
Train the Memex World Model.

This script:
1. Loads entities and attention edges from memex
2. Builds training data (positive/negative pairs)
3. Trains the world model with contrastive + link prediction losses
4. Saves checkpoints and syncs predictions back to attention DAG

Usage:
    python train_world_model.py --epochs 100 --batch-size 64
    python train_world_model.py --resume checkpoints/world_model/latest.pt
    python train_world_model.py --eval-only --checkpoint checkpoints/world_model/best.pt
"""

import argparse
import os
import sys
from typing import List, Tuple, Optional
import random

import numpy as np
import torch
from torch.utils.data import Dataset, DataLoader

# Add parent directory to path for imports
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from world_model import (
    WorldModelConfig,
    TrainingConfig,
    WorldModel,
    create_world_model,
    WorldModelTrainer,
    AttentionDAGIntegration,
    MemexClient,
    WorldModelInference,
)
from world_model.training import TrainingBatch


class MemexDataset(Dataset):
    """
    Dataset that loads from memex for world model training.
    """

    def __init__(
        self,
        integration: AttentionDAGIntegration,
        num_entities: int = 1000,
        num_negatives: int = 64,
        samples_per_epoch: int = 1000,
    ):
        self.integration = integration
        self.num_entities = num_entities
        self.num_negatives = num_negatives
        self.samples_per_epoch = samples_per_epoch

        # Load data
        self.entity_ids, self.entity_types, self.edge_weights = \
            integration.get_attention_dag_as_tensor(num_entities)

        # Get positive pairs from attention edges
        self.positive_pairs = integration.get_positive_pairs(min_weight=0.3)
        print(f"Loaded {len(self.positive_pairs)} positive pairs from attention DAG")

        # Create set of positive pairs for negative sampling
        self.positive_set = {(p[0], p[1]) for p in self.positive_pairs}

    def __len__(self):
        return self.samples_per_epoch

    def __getitem__(self, idx):
        # Sample a positive pair
        if self.positive_pairs:
            pos_idx = idx % len(self.positive_pairs)
            src, tgt, weight = self.positive_pairs[pos_idx]
        else:
            # Random pair if no attention edges
            src = random.randint(0, self.num_entities - 1)
            tgt = random.randint(0, self.num_entities - 1)
            weight = 0.5

        # Sample negative indices
        negatives = []
        while len(negatives) < self.num_negatives:
            neg = random.randint(0, self.num_entities - 1)
            if neg != src and neg != tgt and (src, neg) not in self.positive_set:
                negatives.append(neg)

        # Create link prediction samples (mix of positive and negative)
        link_pairs = [(src, tgt)]
        link_labels = [1.0]
        link_weights_list = [weight]

        for neg in negatives[:8]:  # Use some negatives for link prediction
            link_pairs.append((src, neg))
            link_labels.append(0.0)
            link_weights_list.append(0.0)

        return {
            "entity_ids": self.entity_ids,
            "entity_types": self.entity_types,
            "edge_weights": self.edge_weights,
            "positive_pairs": torch.tensor([src, tgt]),
            "negative_indices": torch.tensor(negatives),
            "link_pairs": torch.tensor(link_pairs),
            "link_labels": torch.tensor(link_labels),
            "link_weights": torch.tensor(link_weights_list),
        }


def collate_fn(batch: List[dict]) -> TrainingBatch:
    """Collate function for DataLoader."""
    return TrainingBatch(
        entity_ids=torch.stack([b["entity_ids"] for b in batch]),
        entity_types=torch.stack([b["entity_types"] for b in batch]),
        edge_weights=torch.stack([b["edge_weights"] for b in batch]),
        content_embeds=None,
        positive_pairs=torch.stack([b["positive_pairs"] for b in batch]),
        negative_indices=torch.stack([b["negative_indices"] for b in batch]),
        link_pairs=torch.stack([b["link_pairs"] for b in batch]),
        link_labels=torch.stack([b["link_labels"] for b in batch]),
        link_weights=torch.stack([b["link_weights"] for b in batch]),
        next_entity_ids=None,
        next_entity_types=None,
        next_edge_weights=None,
    )


def set_seed(seed: int):
    """Set random seeds for reproducibility."""
    random.seed(seed)
    np.random.seed(seed)
    torch.manual_seed(seed)
    if torch.cuda.is_available():
        torch.cuda.manual_seed_all(seed)


def main():
    parser = argparse.ArgumentParser(description="Train the Memex World Model")

    # Training arguments
    parser.add_argument("--epochs", type=int, default=100, help="Number of epochs")
    parser.add_argument("--batch-size", type=int, default=32, help="Batch size")
    parser.add_argument("--lr", type=float, default=1e-4, help="Learning rate")
    parser.add_argument("--hidden-dim", type=int, default=256, help="Hidden dimension")
    parser.add_argument("--num-layers", type=int, default=4, help="Number of transformer layers")

    # Data arguments
    parser.add_argument("--num-entities", type=int, default=1000, help="Max entities to load")
    parser.add_argument("--num-negatives", type=int, default=64, help="Negative samples per positive")
    parser.add_argument("--samples-per-epoch", type=int, default=1000, help="Samples per epoch")

    # Checkpointing
    parser.add_argument("--checkpoint-dir", type=str, default="checkpoints/world_model")
    parser.add_argument("--resume", type=str, default=None, help="Resume from checkpoint")
    parser.add_argument("--save-every", type=int, default=10, help="Save every N epochs")

    # Evaluation
    parser.add_argument("--eval-only", action="store_true", help="Only run evaluation")
    parser.add_argument("--checkpoint", type=str, default=None, help="Checkpoint for eval")

    # Memex connection
    parser.add_argument("--memex-api", type=str, default="http://localhost:8080")

    # Other
    parser.add_argument("--seed", type=int, default=42, help="Random seed")
    parser.add_argument("--device", type=str, default="cuda", help="Device (cuda/cpu)")

    args = parser.parse_args()

    # Set seed
    set_seed(args.seed)

    # Create config
    config = WorldModelConfig(
        hidden_dim=args.hidden_dim,
        num_layers=args.num_layers,
        learning_rate=args.lr,
        num_epochs=args.epochs,
        checkpoint_dir=args.checkpoint_dir,
        save_every=args.save_every,
        memex_api=args.memex_api,
        device=args.device if torch.cuda.is_available() else "cpu",
        num_negatives=args.num_negatives,
    )

    training_config = TrainingConfig(seed=args.seed)

    print("=" * 60)
    print("Memex World Model Training")
    print("=" * 60)
    print(f"Hidden dim: {config.hidden_dim}")
    print(f"Num layers: {config.num_layers}")
    print(f"Learning rate: {config.learning_rate}")
    print(f"Device: {config.device}")
    print(f"Memex API: {config.memex_api}")
    print("=" * 60)

    # Create model
    model = create_world_model(config)
    print(f"Model parameters: {sum(p.numel() for p in model.parameters()):,}")

    # Setup memex integration
    memex_client = MemexClient(config.memex_api)
    integration = AttentionDAGIntegration(model, config, memex_client)

    # Load entities
    num_loaded = integration.load_entities(limit=args.num_entities)
    if num_loaded == 0:
        print("Warning: No entities loaded from memex. Is the server running?")
        print("Try: curl http://localhost:8080/api/nodes")
        return

    # Evaluation only mode
    if args.eval_only:
        if args.checkpoint is None:
            print("Error: --checkpoint required for --eval-only")
            return

        print(f"\nLoading checkpoint: {args.checkpoint}")
        inference = WorldModelInference(model, config, args.checkpoint)

        print("\nRunning evaluation...")
        summary = inference.get_state_summary()
        print(f"State summary: {summary}")

        print("\nTop predicted entities:")
        predictions = inference.predict_next_entities(top_k=10)
        for i, pred in enumerate(predictions):
            print(f"  {i+1}. [{pred.entity_type}] {pred.entity_id} (score: {pred.score:.4f})")

        print("\nSimulating 5 steps forward...")
        trajectory = inference.simulate_trajectory(num_steps=5)
        for step in trajectory:
            print(f"  Step {step.step}: state_change={step.state_change:.4f}")
            print(f"    Top: {step.top_entities[0].entity_id} ({step.top_entities[0].score:.4f})")

        return

    # Create dataset and dataloader
    print("\nCreating dataset...")
    dataset = MemexDataset(
        integration,
        num_entities=args.num_entities,
        num_negatives=args.num_negatives,
        samples_per_epoch=args.samples_per_epoch,
    )

    dataloader = DataLoader(
        dataset,
        batch_size=args.batch_size,
        shuffle=True,
        collate_fn=collate_fn,
        num_workers=0,  # Keep simple for now
    )

    # Create trainer
    trainer = WorldModelTrainer(model, config, training_config)

    # Resume if specified
    start_epoch = 0
    if args.resume:
        start_epoch = trainer.load_checkpoint(args.resume)

    # Training loop
    print(f"\nStarting training from epoch {start_epoch + 1}...")
    os.makedirs(args.checkpoint_dir, exist_ok=True)

    for epoch in range(start_epoch, args.epochs):
        print(f"\nEpoch {epoch + 1}/{args.epochs}")

        # Train epoch
        losses = trainer.train_epoch(dataloader)
        print(f"Losses: {losses}")

        # Save checkpoint
        if (epoch + 1) % args.save_every == 0:
            checkpoint_path = os.path.join(args.checkpoint_dir, f"epoch_{epoch + 1}.pt")
            trainer.save_checkpoint(checkpoint_path, epoch)

        # Sync with attention DAG periodically
        if (epoch + 1) % 20 == 0:
            print("Syncing predictions with attention DAG...")
            stats = integration.sync_with_dag(
                write_proposals=True,
                proposal_threshold=0.8,
            )
            print(f"Sync stats: {stats}")

    # Save final model
    final_path = os.path.join(args.checkpoint_dir, "final.pt")
    trainer.save_checkpoint(final_path, args.epochs)
    print(f"\nTraining complete! Model saved to {final_path}")


if __name__ == "__main__":
    main()
