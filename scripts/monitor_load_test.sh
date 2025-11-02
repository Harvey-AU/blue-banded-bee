#!/bin/bash

# Monitor Fly logs every minute for 30 minutes
# Track key observability metrics during load test

DURATION_MINUTES=30
CHECK_INTERVAL=60  # seconds
mkdir -p ./logs
LOG_FILE="./logs/load_test_monitoring_$(date +%Y%m%d_%H%M%S).log"

echo "=== Blue Banded Bee Load Test Monitor ===" | tee -a "$LOG_FILE"
echo "Started at: $(date)" | tee -a "$LOG_FILE"
echo "Duration: ${DURATION_MINUTES} minutes" | tee -a "$LOG_FILE"
echo "Check interval: ${CHECK_INTERVAL} seconds" | tee -a "$LOG_FILE"
echo "" | tee -a "$LOG_FILE"

for i in $(seq 1 $DURATION_MINUTES); do
    echo "=== Check $i of $DURATION_MINUTES at $(date) ===" | tee -a "$LOG_FILE"

    # Get recent logs (no-tail to get snapshot)
    LOGS=$(flyctl logs --app blue-banded-bee --no-tail 2>&1 | tail -50)

    # Count key events in recent logs
    JOBS_ADDED=$(echo "$LOGS" | grep -c "Adding job with pending tasks to worker pool" || true)
    JOBS_RUNNING=$(echo "$LOGS" | grep -c "Updated job status to running" || true)
    JOBS_COMPLETED=$(echo "$LOGS" | grep -c "Updated job status to completed" || true)
    JOBS_FAILED=$(echo "$LOGS" | grep -c "Updated job status to failed" || true)
    ERRORS=$(echo "$LOGS" | grep -c '"level":"error"' || true)
    WARNINGS=$(echo "$LOGS" | grep -c '"level":"warn"' || true)

    # Look for new observability metrics
    METRICS=$(echo "$LOGS" | grep "metrics" || true)
    POOL_STATS=$(echo "$LOGS" | grep -E "worker pool|concurrency|tasks" || true)

    echo "Jobs added to pool: $JOBS_ADDED" | tee -a "$LOG_FILE"
    echo "Jobs set to running: $JOBS_RUNNING" | tee -a "$LOG_FILE"
    echo "Jobs completed: $JOBS_COMPLETED" | tee -a "$LOG_FILE"
    echo "Jobs failed: $JOBS_FAILED" | tee -a "$LOG_FILE"
    echo "Errors: $ERRORS" | tee -a "$LOG_FILE"
    echo "Warnings: $WARNINGS" | tee -a "$LOG_FILE"

    if [ -n "$METRICS" ]; then
        echo "" | tee -a "$LOG_FILE"
        echo "Observability Metrics:" | tee -a "$LOG_FILE"
        echo "$METRICS" | tee -a "$LOG_FILE"
    fi

    if [ -n "$POOL_STATS" ]; then
        echo "" | tee -a "$LOG_FILE"
        echo "Worker Pool Stats:" | tee -a "$LOG_FILE"
        echo "$POOL_STATS" | tee -a "$LOG_FILE"
    fi

    # Check for any errors or warnings
    if [ "$ERRORS" -gt 0 ]; then
        echo "" | tee -a "$LOG_FILE"
        echo "ERROR DETAILS:" | tee -a "$LOG_FILE"
        echo "$LOGS" | grep '"level":"error"' | tee -a "$LOG_FILE"
    fi

    if [ "$WARNINGS" -gt 0 ]; then
        echo "" | tee -a "$LOG_FILE"
        echo "WARNING DETAILS:" | tee -a "$LOG_FILE"
        echo "$LOGS" | grep '"level":"warn"' | tee -a "$LOG_FILE"
    fi

    echo "" | tee -a "$LOG_FILE"
    echo "---" | tee -a "$LOG_FILE"
    echo "" | tee -a "$LOG_FILE"

    # Sleep unless this is the last iteration
    if [ "$i" -lt "$DURATION_MINUTES" ]; then
        sleep $CHECK_INTERVAL
    fi
done

echo "=== Monitoring Complete at $(date) ===" | tee -a "$LOG_FILE"
echo "Full logs saved to: $LOG_FILE" | tee -a "$LOG_FILE"
