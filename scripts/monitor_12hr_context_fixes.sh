#!/bin/bash

# 12-Hour Context Fixes Monitor
# Tracks context timeout fixes, retry backoff, and system health
# Samples every 5 minutes for 12 hours (144 checks)
# Captures debug-level logs to file for analysis

DURATION_MINUTES=720  # 12 hours
CHECK_INTERVAL=300    # 5 minutes in seconds
mkdir -p ./logs
LOG_FILE="./logs/monitor_12hr_$(date +%Y%m%d_%H%M%S).log"
RAW_LOGS_DIR="./logs/raw_$(date +%Y%m%d_%H%M%S)"
mkdir -p "$RAW_LOGS_DIR"

echo "=== Blue Banded Bee 12-Hour Context Fixes Monitor ===" | tee -a "$LOG_FILE"
echo "Started at: $(date)" | tee -a "$LOG_FILE"
echo "Duration: ${DURATION_MINUTES} minutes (12 hours)" | tee -a "$LOG_FILE"
echo "Check interval: ${CHECK_INTERVAL} seconds (5 minutes)" | tee -a "$LOG_FILE"
echo "Expected load: 3 jobs every 3 minutes = ~720 jobs total" | tee -a "$LOG_FILE"
echo "Raw logs saved to: $RAW_LOGS_DIR" | tee -a "$LOG_FILE"
echo "" | tee -a "$LOG_FILE"

CHECKS=$((DURATION_MINUTES / 5))

for i in $(seq 1 $CHECKS); do
    ELAPSED_MINS=$((i * 5))
    TIMESTAMP=$(date +%Y%m%d_%H%M%S)

    echo "===========================================" | tee -a "$LOG_FILE"
    echo "Check $i of $CHECKS - ${ELAPSED_MINS} minutes elapsed ($(date))" | tee -a "$LOG_FILE"
    echo "===========================================" | tee -a "$LOG_FILE"

    # Capture raw logs to file for detailed analysis
    RAW_LOG_FILE="$RAW_LOGS_DIR/logs_${TIMESTAMP}.txt"
    flyctl logs --app blue-banded-bee --no-tail 2>&1 | tail -1000 > "$RAW_LOG_FILE"
    LOGS=$(cat "$RAW_LOG_FILE")

    # Context timeout & retry metrics (NEW - key focus)
    TIMEOUT_CONTEXTS=$(echo "$LOGS" | grep -c "timeout.*30.*second\|30.*timeout" || true)
    CONTEXT_CANCELLED=$(echo "$LOGS" | grep -c "context canceled\|context deadline exceeded" || true)
    RETRY_BACKOFF=$(echo "$LOGS" | grep -c "backoff.*retry\|Retrying.*backoff" || true)
    WAIT_FOR_RETRY=$(echo "$LOGS" | grep -c "waitForRetry" || true)

    # Batch flushing metrics (NEW - verify fixes)
    BATCH_FLUSHES=$(echo "$LOGS" | grep -c "Batch update\|flushTaskUpdates" || true)
    BATCH_FALLBACK=$(echo "$LOGS" | grep -c "individual updates\|poison pill" || true)
    BATCH_TIMEOUT=$(echo "$LOGS" | grep -c "batch.*timeout\|flush.*timeout" || true)

    # DecrementRunningTasks metrics (NEW - verify bounded contexts)
    DECREMENT_CALLS=$(echo "$LOGS" | grep -c "DecrementRunningTasks called" || true)
    DECREMENT_EXECUTED=$(echo "$LOGS" | grep -c "DecrementRunningTasks executed" || true)

    # Job lifecycle metrics
    JOBS_ADDED=$(echo "$LOGS" | grep -c "Adding job with pending tasks to worker pool" || true)
    JOBS_RUNNING=$(echo "$LOGS" | grep -c "Updated job status to running" || true)
    JOBS_COMPLETED=$(echo "$LOGS" | grep -c "Updated job status to completed" || true)
    JOBS_FAILED=$(echo "$LOGS" | grep -c "Updated job status to failed" || true)

    # Task metrics
    TASKS_COMPLETED=$(echo "$LOGS" | grep -c '"message":"Crawler completed"' || true)
    TASKS_CLAIMED=$(echo "$LOGS" | grep -c '"message":"Found and claimed pending task"' || true)
    TASKS_FAILED=$(echo "$LOGS" | grep -c '"message":"Task failed permanently"' || true)

    # Error tracking
    ERRORS=$(echo "$LOGS" | grep -c '"level":"error"' || true)
    WARNINGS=$(echo "$LOGS" | grep -c '"level":"warn"' || true)
    DB_ERRORS=$(echo "$LOGS" | grep -c "database.*error\|failed to.*database" || true)
    BLOCKING_ERRORS=$(echo "$LOGS" | grep -c "indefinitely\|blocking\|stuck" || true)

    # Performance metrics
    AVG_RESPONSE=$(echo "$LOGS" | grep '"avg_response_time"' | tail -10 | grep -o '"avg_response_time":[0-9]*' | cut -d: -f2 | awk '{sum+=$1; count++} END {if(count>0) print int(sum/count); else print "N/A"}')
    SLOW_QUERIES=$(echo "$LOGS" | grep -c "Slow.*query\|Slow.*transaction" || true)

    echo "" | tee -a "$LOG_FILE"
    echo "🔒 CONTEXT TIMEOUT & RETRY (KEY METRICS):" | tee -a "$LOG_FILE"
    echo "  Timeout contexts created: $TIMEOUT_CONTEXTS" | tee -a "$LOG_FILE"
    echo "  Context cancellations:    $CONTEXT_CANCELLED" | tee -a "$LOG_FILE"
    echo "  Retry with backoff:       $RETRY_BACKOFF" | tee -a "$LOG_FILE"
    echo "  WaitForRetry calls:       $WAIT_FOR_RETRY" | tee -a "$LOG_FILE"
    echo "" | tee -a "$LOG_FILE"

    echo "📦 BATCH FLUSHING:" | tee -a "$LOG_FILE"
    echo "  Batch flushes:            $BATCH_FLUSHES" | tee -a "$LOG_FILE"
    echo "  Poison pill fallbacks:    $BATCH_FALLBACK" | tee -a "$LOG_FILE"
    echo "  Batch timeouts:           $BATCH_TIMEOUT" | tee -a "$LOG_FILE"
    echo "" | tee -a "$LOG_FILE"

    echo "⚙️  DECREMENT RUNNING TASKS:" | tee -a "$LOG_FILE"
    echo "  Calls:                    $DECREMENT_CALLS" | tee -a "$LOG_FILE"
    echo "  Successful executions:    $DECREMENT_EXECUTED" | tee -a "$LOG_FILE"
    echo "" | tee -a "$LOG_FILE"

    echo "📊 JOB LIFECYCLE:" | tee -a "$LOG_FILE"
    echo "  Jobs added:               $JOBS_ADDED" | tee -a "$LOG_FILE"
    echo "  Jobs running:             $JOBS_RUNNING" | tee -a "$LOG_FILE"
    echo "  Jobs completed:           $JOBS_COMPLETED" | tee -a "$LOG_FILE"
    echo "  Jobs failed:              $JOBS_FAILED" | tee -a "$LOG_FILE"
    echo "" | tee -a "$LOG_FILE"

    echo "📈 TASK THROUGHPUT:" | tee -a "$LOG_FILE"
    echo "  Tasks completed:          $TASKS_COMPLETED" | tee -a "$LOG_FILE"
    echo "  Tasks claimed:            $TASKS_CLAIMED" | tee -a "$LOG_FILE"
    echo "  Tasks failed:             $TASKS_FAILED" | tee -a "$LOG_FILE"
    echo "" | tee -a "$LOG_FILE"

    echo "⚠️  ERRORS & ISSUES:" | tee -a "$LOG_FILE"
    echo "  Total errors:             $ERRORS" | tee -a "$LOG_FILE"
    echo "  Total warnings:           $WARNINGS" | tee -a "$LOG_FILE"
    echo "  Database errors:          $DB_ERRORS" | tee -a "$LOG_FILE"
    echo "  Blocking errors:          $BLOCKING_ERRORS" | tee -a "$LOG_FILE"
    echo "" | tee -a "$LOG_FILE"

    echo "⚡ PERFORMANCE:" | tee -a "$LOG_FILE"
    echo "  Avg response (ms):        $AVG_RESPONSE" | tee -a "$LOG_FILE"
    echo "  Slow queries/transactions: $SLOW_QUERIES" | tee -a "$LOG_FILE"
    echo "" | tee -a "$LOG_FILE"

    # Show critical issues
    if [ "$ERRORS" -gt 0 ]; then
        echo "🚨 ERROR SAMPLES:" | tee -a "$LOG_FILE"
        echo "$LOGS" | grep '"level":"error"' | tail -5 | tee -a "$LOG_FILE"
        echo "" | tee -a "$LOG_FILE"
    fi

    if [ "$CONTEXT_CANCELLED" -gt 0 ]; then
        echo "⏱️  CONTEXT CANCELLATION SAMPLES:" | tee -a "$LOG_FILE"
        echo "$LOGS" | grep -i "context.*cancel\|deadline exceeded" | tail -5 | tee -a "$LOG_FILE"
        echo "" | tee -a "$LOG_FILE"
    fi

    if [ "$BLOCKING_ERRORS" -gt 0 ]; then
        echo "🚫 BLOCKING ERROR SAMPLES:" | tee -a "$LOG_FILE"
        echo "$LOGS" | grep -i "indefinitely\|blocking\|stuck" | tail -5 | tee -a "$LOG_FILE"
        echo "" | tee -a "$LOG_FILE"
    fi

    # Check machine health
    echo "🖥️  MACHINE STATUS:" | tee -a "$LOG_FILE"
    flyctl status --app blue-banded-bee 2>&1 | grep -E "STATE|started|stopped" | tee -a "$LOG_FILE"
    echo "" | tee -a "$LOG_FILE"

    echo "Raw logs for this check: $RAW_LOG_FILE" | tee -a "$LOG_FILE"
    echo "---" | tee -a "$LOG_FILE"
    echo "" | tee -a "$LOG_FILE"

    # Sleep unless this is the last iteration
    if [ "$i" -lt "$CHECKS" ]; then
        sleep $CHECK_INTERVAL
    fi
done

echo "===========================================" | tee -a "$LOG_FILE"
echo "=== 12-Hour Monitoring Complete ===" | tee -a "$LOG_FILE"
echo "Ended at: $(date)" | tee -a "$LOG_FILE"
echo "Summary saved to: $LOG_FILE" | tee -a "$LOG_FILE"
echo "Raw logs saved to: $RAW_LOGS_DIR" | tee -a "$LOG_FILE"
echo "===========================================" | tee -a "$LOG_FILE"

# Generate summary statistics
echo "" | tee -a "$LOG_FILE"
echo "=== FINAL SUMMARY ===" | tee -a "$LOG_FILE"
TOTAL_ERRORS=$(grep "Total errors:" "$LOG_FILE" | awk '{sum+=$3} END {print sum}')
TOTAL_CONTEXT_CANCELLED=$(grep "Context cancellations:" "$LOG_FILE" | awk '{sum+=$3} END {print sum}')
TOTAL_TASKS=$(grep "Tasks completed:" "$LOG_FILE" | awk '{sum+=$3} END {print sum}')
echo "Total errors across all checks: $TOTAL_ERRORS" | tee -a "$LOG_FILE"
echo "Total context cancellations: $TOTAL_CONTEXT_CANCELLED" | tee -a "$LOG_FILE"
echo "Total tasks completed: $TOTAL_TASKS" | tee -a "$LOG_FILE"
