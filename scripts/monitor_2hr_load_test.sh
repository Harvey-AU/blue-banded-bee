#!/bin/bash

# 2-Hour Load Test Monitor
# Tracks key metrics for 3 jobs/minute load test
# Samples every 5 minutes for 2 hours (24 checks)

DURATION_MINUTES=120
CHECK_INTERVAL=300  # 5 minutes in seconds
mkdir -p ./logs
LOG_FILE="./logs/load_test_2hr_$(date +%Y%m%d_%H%M%S).log"

echo "=== Blue Banded Bee 2-Hour Load Test Monitor ===" | tee -a "$LOG_FILE"
echo "Started at: $(date)" | tee -a "$LOG_FILE"
echo "Duration: ${DURATION_MINUTES} minutes (2 hours)" | tee -a "$LOG_FILE"
echo "Check interval: ${CHECK_INTERVAL} seconds (5 minutes)" | tee -a "$LOG_FILE"
echo "Expected load: 3 jobs/minute = ~360 jobs total" | tee -a "$LOG_FILE"
echo "" | tee -a "$LOG_FILE"

CHECKS=$((DURATION_MINUTES / 5))

for i in $(seq 1 $CHECKS); do
    ELAPSED_MINS=$((i * 5))
    echo "===========================================" | tee -a "$LOG_FILE"
    echo "Check $i of $CHECKS - ${ELAPSED_MINS} minutes elapsed" | tee -a "$LOG_FILE"
    echo "Time: $(date)" | tee -a "$LOG_FILE"
    echo "===========================================" | tee -a "$LOG_FILE"

    # Get recent logs (larger sample for 5min intervals)
    LOGS=$(flyctl logs --app blue-banded-bee --no-tail 2>&1 | tail -500)

    # Job lifecycle metrics
    JOBS_ADDED=$(echo "$LOGS" | grep -c "Adding job with pending tasks to worker pool" || true)
    JOBS_RUNNING=$(echo "$LOGS" | grep -c "Updated job status to running" || true)
    JOBS_COMPLETED=$(echo "$LOGS" | grep -c "Updated job status to completed" || true)
    JOBS_FAILED=$(echo "$LOGS" | grep -c "Updated job status to failed" || true)
    JOBS_CANCELLED=$(echo "$LOGS" | grep -c "Updated job status to cancelled" || true)

    # Task metrics
    TASKS_COMPLETED=$(echo "$LOGS" | grep -c '"message":"Crawler completed"' || true)
    TASKS_CLAIMED=$(echo "$LOGS" | grep -c '"message":"Found and claimed pending task"' || true)
    BATCH_UPDATES=$(echo "$LOGS" | grep -c '"message":"Batch update successful"' || true)

    # Domain rate limiter metrics
    ADAPTIVE_DELAY_UPDATES=$(echo "$LOGS" | grep -c 'Updated domain adaptive delay' || true)
    DOMAIN_BACKOFF=$(echo "$LOGS" | grep -c 'domain.*backoff' || true)
    CONCURRENCY_REDUCTIONS=$(echo "$LOGS" | grep -c 'Reduced job concurrency' || true)

    # Error tracking
    ERRORS=$(echo "$LOGS" | grep -c '"level":"error"' || true)
    WARNINGS=$(echo "$LOGS" | grep -c '"level":"warn"' || true)
    ERROR_429=$(echo "$LOGS" | grep -c '429' || true)
    BLOCKING_ERRORS=$(echo "$LOGS" | grep -c 'Blocking error' || true)
    RETRIES=$(echo "$LOGS" | grep -c 'retry scheduled' || true)

    # Job failure guardrail
    JOB_FAILURES=$(echo "$LOGS" | grep -c 'Job failed after.*consecutive task failures' || true)
    ORPHANED_TASKS=$(echo "$LOGS" | grep -c 'Cleaned up orphaned tasks' || true)

    # Performance metrics
    AVG_RESPONSE=$(echo "$LOGS" | grep '"avg_response_time"' | tail -10 | grep -o '"avg_response_time":[0-9]*' | cut -d: -f2 | awk '{sum+=$1; count++} END {if(count>0) print int(sum/count); else print "N/A"}')
    SCALING_EVENTS=$(echo "$LOGS" | grep -c 'Job performance scaling triggered' || true)
    POOL_SCALING=$(echo "$LOGS" | grep -c 'Scaling.*worker pool' || true)

    echo "" | tee -a "$LOG_FILE"
    echo "📊 JOB LIFECYCLE:" | tee -a "$LOG_FILE"
    echo "  Jobs added to pool:  $JOBS_ADDED" | tee -a "$LOG_FILE"
    echo "  Jobs running:        $JOBS_RUNNING" | tee -a "$LOG_FILE"
    echo "  Jobs completed:      $JOBS_COMPLETED" | tee -a "$LOG_FILE"
    echo "  Jobs failed:         $JOBS_FAILED" | tee -a "$LOG_FILE"
    echo "  Jobs cancelled:      $JOBS_CANCELLED" | tee -a "$LOG_FILE"
    echo "" | tee -a "$LOG_FILE"

    echo "📈 TASK THROUGHPUT:" | tee -a "$LOG_FILE"
    echo "  Tasks completed:     $TASKS_COMPLETED" | tee -a "$LOG_FILE"
    echo "  Tasks claimed:       $TASKS_CLAIMED" | tee -a "$LOG_FILE"
    echo "  Batch updates:       $BATCH_UPDATES" | tee -a "$LOG_FILE"
    echo "" | tee -a "$LOG_FILE"

    echo "🔧 DOMAIN RATE LIMITER:" | tee -a "$LOG_FILE"
    echo "  Adaptive delays:     $ADAPTIVE_DELAY_UPDATES" | tee -a "$LOG_FILE"
    echo "  Domain backoffs:     $DOMAIN_BACKOFF" | tee -a "$LOG_FILE"
    echo "  Concurrency reduced: $CONCURRENCY_REDUCTIONS" | tee -a "$LOG_FILE"
    echo "" | tee -a "$LOG_FILE"

    echo "⚠️  ERRORS & ISSUES:" | tee -a "$LOG_FILE"
    echo "  Total errors:        $ERRORS" | tee -a "$LOG_FILE"
    echo "  Total warnings:      $WARNINGS" | tee -a "$LOG_FILE"
    echo "  429 responses:       $ERROR_429" | tee -a "$LOG_FILE"
    echo "  Blocking errors:     $BLOCKING_ERRORS" | tee -a "$LOG_FILE"
    echo "  Retries scheduled:   $RETRIES" | tee -a "$LOG_FILE"
    echo "" | tee -a "$LOG_FILE"

    echo "🛡️  JOB FAILURE GUARDRAIL:" | tee -a "$LOG_FILE"
    echo "  Jobs auto-failed:    $JOB_FAILURES" | tee -a "$LOG_FILE"
    echo "  Orphaned tasks cleaned: $ORPHANED_TASKS" | tee -a "$LOG_FILE"
    echo "" | tee -a "$LOG_FILE"

    echo "⚡ PERFORMANCE:" | tee -a "$LOG_FILE"
    echo "  Avg response (ms):   $AVG_RESPONSE" | tee -a "$LOG_FILE"
    echo "  Job scaling events:  $SCALING_EVENTS" | tee -a "$LOG_FILE"
    echo "  Pool scaling events: $POOL_SCALING" | tee -a "$LOG_FILE"
    echo "" | tee -a "$LOG_FILE"

    # Show critical errors if any
    if [ "$ERRORS" -gt 0 ]; then
        echo "🚨 ERROR SAMPLES:" | tee -a "$LOG_FILE"
        echo "$LOGS" | grep '"level":"error"' | tail -3 | tee -a "$LOG_FILE"
        echo "" | tee -a "$LOG_FILE"
    fi

    # Show adaptive delay updates if any
    if [ "$ADAPTIVE_DELAY_UPDATES" -gt 0 ]; then
        echo "🔄 ADAPTIVE DELAY UPDATES:" | tee -a "$LOG_FILE"
        echo "$LOGS" | grep 'Updated domain adaptive delay' | tail -5 | tee -a "$LOG_FILE"
        echo "" | tee -a "$LOG_FILE"
    fi

    # Show job failures if any
    if [ "$JOB_FAILURES" -gt 0 ]; then
        echo "❌ JOB FAILURE GUARDRAIL TRIGGERED:" | tee -a "$LOG_FILE"
        echo "$LOGS" | grep 'Job failed after.*consecutive task failures' | tee -a "$LOG_FILE"
        echo "" | tee -a "$LOG_FILE"
    fi

    echo "---" | tee -a "$LOG_FILE"
    echo "" | tee -a "$LOG_FILE"

    # Sleep unless this is the last iteration
    if [ "$i" -lt "$CHECKS" ]; then
        sleep $CHECK_INTERVAL
    fi
done

echo "===========================================" | tee -a "$LOG_FILE"
echo "=== Monitoring Complete ===" | tee -a "$LOG_FILE"
echo "Ended at: $(date)" | tee -a "$LOG_FILE"
echo "Full logs saved to: $LOG_FILE" | tee -a "$LOG_FILE"
echo "===========================================" | tee -a "$LOG_FILE"
