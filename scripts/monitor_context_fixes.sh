#!/bin/bash

# Monitor for context timeout and retry backoff fixes
# This script specifically checks for signs that our fixes are working

echo "=== Context Timeout & Retry Fixes Monitor ==="
echo "Started at: $(date)"
echo "Monitoring for: timeout contexts, retry backoff, batch flushing"
echo ""

# Function to check deployment status
check_deployment() {
    flyctl status --app blue-banded-bee 2>&1 | grep -q "running"
    return $?
}

# Wait for deployment to complete
echo "Checking if deployment is ready..."
while ! check_deployment; do
    echo "Waiting for deployment... (checking every 10s)"
    sleep 10
done

echo "✅ Deployment is live, starting log monitoring..."
echo ""

# Monitor logs with debug level
echo "Fetching logs with debug level..."
flyctl logs --app blue-banded-bee --no-tail 2>&1 | tail -500 > /tmp/bbb_logs.txt

# Analysis
echo "=========================================="
echo "📊 CONTEXT & TIMEOUT ANALYSIS"
echo "=========================================="

# Check for timeout-related logs
TIMEOUT_CONTEXTS=$(grep -c "30.*second.*timeout\|timeout.*30" /tmp/bbb_logs.txt)
CONTEXT_CANCELLED=$(grep -c "context canceled\|context deadline exceeded" /tmp/bbb_logs.txt)
RETRY_BACKOFF=$(grep -c "Retrying.*backoff\|backoff.*retry" /tmp/bbb_logs.txt)

echo "Timeout contexts created: $TIMEOUT_CONTEXTS"
echo "Context cancellations: $CONTEXT_CANCELLED"
echo "Retry with backoff: $RETRY_BACKOFF"
echo ""

# Check for batch flushing
echo "=========================================="
echo "📦 BATCH FLUSHING ANALYSIS"
echo "=========================================="

BATCH_FLUSHES=$(grep -c "Batch update successful\|flushTaskUpdates" /tmp/bbb_logs.txt)
BATCH_FALLBACK=$(grep -c "individual updates\|poison pill" /tmp/bbb_logs.txt)
BATCH_TIMEOUT=$(grep -c "batch.*timeout\|flush.*timeout" /tmp/bbb_logs.txt)

echo "Batch flushes: $BATCH_FLUSHES"
echo "Fallback to individual updates: $BATCH_FALLBACK"
echo "Batch timeouts: $BATCH_TIMEOUT"
echo ""

# Check for worker activity
echo "=========================================="
echo "⚙️  WORKER ACTIVITY"
echo "=========================================="

TASKS_COMPLETED=$(grep -c "Crawler completed" /tmp/bbb_logs.txt)
TASKS_CLAIMED=$(grep -c "Found and claimed pending task" /tmp/bbb_logs.txt)
DECREMENT_CALLS=$(grep -c "DecrementRunningTasks" /tmp/bbb_logs.txt)

echo "Tasks completed: $TASKS_COMPLETED"
echo "Tasks claimed: $TASKS_CLAIMED"
echo "Running tasks decrements: $DECREMENT_CALLS"
echo ""

# Check for errors
echo "=========================================="
echo "⚠️  ERROR ANALYSIS"
echo "=========================================="

ERRORS=$(grep -c '"level":"error"' /tmp/bbb_logs.txt)
DB_ERRORS=$(grep -c "database.*error\|failed to.*database" /tmp/bbb_logs.txt)
BLOCKING_ERRORS=$(grep -c "indefinitely\|blocking\|stuck" /tmp/bbb_logs.txt)

echo "Total errors: $ERRORS"
echo "Database errors: $DB_ERRORS"
echo "Blocking/stuck issues: $BLOCKING_ERRORS"
echo ""

# Show recent errors if any
if [ $ERRORS -gt 0 ]; then
    echo "🔍 RECENT ERRORS (last 5):"
    grep '"level":"error"' /tmp/bbb_logs.txt | tail -5
    echo ""
fi

# Show context-related messages
echo "=========================================="
echo "🔍 RECENT CONTEXT ACTIVITY (last 10):"
echo "=========================================="
grep -E "timeout|context|backoff|retry" /tmp/bbb_logs.txt | tail -10
echo ""

# Continuous monitoring
echo "=========================================="
echo "🔄 CONTINUOUS MONITORING (press Ctrl+C to stop)"
echo "=========================================="
echo ""

flyctl logs --app blue-banded-bee
