#!/bin/bash
# Parallel ingestion script
# Runs multiple workers processing different paragraph ranges

set -e

cd /home/deocy/memex/bench
source /home/deocy/bench_env/bin/activate

# Configuration
TOTAL_PARAGRAPHS=66635
NUM_WORKERS=10
CONCURRENCY=5  # Per worker
CHUNK_SIZE=$((TOTAL_PARAGRAPHS / NUM_WORKERS))

echo "=============================================="
echo "PARALLEL INGESTION STARTING"
echo "=============================================="
echo "Total paragraphs: $TOTAL_PARAGRAPHS"
echo "Workers: $NUM_WORKERS"
echo "Concurrency per worker: $CONCURRENCY"
echo "Total concurrent API calls: $((NUM_WORKERS * CONCURRENCY))"
echo "Chunk size: $CHUNK_SIZE paragraphs per worker"
echo ""

# Clear old logs
rm -f worker_*.log

# Start workers
for i in $(seq 0 $((NUM_WORKERS - 1))); do
    START=$((i * CHUNK_SIZE))

    # Last worker takes any remainder
    if [ $i -eq $((NUM_WORKERS - 1)) ]; then
        LIMIT=$((TOTAL_PARAGRAPHS - START))
    else
        LIMIT=$CHUNK_SIZE
    fi

    echo "Starting worker $i: paragraphs $START to $((START + LIMIT))"
    nohup python ingest_ai.py \
        --start $START \
        --limit $LIMIT \
        --concurrency $CONCURRENCY \
        --worker-id $i \
        > worker_${i}.log 2>&1 &

    echo "  PID: $!"
done

echo ""
echo "All workers started!"
echo ""
echo "Monitor progress with:"
echo "  python monitor.py --watch"
echo ""
echo "Check worker logs:"
echo "  tail -f worker_*.log"
echo ""
echo "Check for errors:"
echo "  grep -h ERROR worker_*.log"
