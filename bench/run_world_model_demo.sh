#!/bin/bash
# World Model Demo Pipeline
# Tests whether the world model can discover hidden patterns in synthetic org data

set -e

echo "=================================================="
echo "MEMEX WORLD MODEL DEMO"
echo "=================================================="
echo ""
echo "This demo:"
echo "  1. Generates synthetic organization data with HIDDEN patterns"
echo "  2. Loads explicit relationships into Memex"
echo "  3. Trains the world model on the graph structure"
echo "  4. Evaluates if the model discovers what we didn't tell it"
echo ""

# Check if memex is running
echo "Checking Memex connection..."
if ! curl -s http://localhost:8080/health > /dev/null; then
    echo "ERROR: Memex server not running. Start it with:"
    echo "  go run ./cmd/memex-server/main.go"
    exit 1
fi
echo "  ✓ Memex is running"

# Step 1: Generate data
echo ""
echo "Step 1: Generating synthetic organization data..."
python3 bench/generate_synthetic_org.py

# Step 2: Load into Memex
echo ""
echo "Step 2: Loading data into Memex..."
python3 bench/load_synthetic_to_memex.py

# Step 3: Train world model
echo ""
echo "Step 3: Training world model..."
echo "  (This will learn the structure of the organization)"
python3 bench/train_world_model.py \
    --epochs 50 \
    --batch-size 32 \
    --hidden-dim 256 \
    --num-layers 2

# Step 4: Evaluate
echo ""
echo "Step 4: Evaluating hidden pattern discovery..."
python3 bench/evaluate_world_model.py

echo ""
echo "=================================================="
echo "DEMO COMPLETE"
echo "=================================================="
echo ""
echo "Check bench/evaluation_results.json for detailed results"
