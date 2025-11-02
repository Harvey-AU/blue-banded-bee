#!/bin/bash

# Detailed monitoring for 15 minutes, checking every minute
DURATION_MINUTES=15
CHECK_INTERVAL=60  # 60 seconds

echo "=== Blue Banded Bee Detailed Monitor ==="
echo "Started at: $(date)"
echo "Will run for $DURATION_MINUTES minutes with checks every minute"
echo ""

END_TIME=$(($(date +%s) + DURATION_MINUTES * 60))
CHECK_NUM=1

while [ $(date +%s) -lt $END_TIME ]; do
    echo "=========================================="
    echo "Check #$CHECK_NUM at $(date)"
    echo "=========================================="
    echo ""

    # Get last 2 minutes of logs
    LOGS=$(flyctl logs --app blue-banded-bee --no-tail 2>&1 | tail -200)

    # Count various metrics
    TASKS_COMPLETED=$(echo "$LOGS" | grep -c '"message":"Crawler completed"')
    TASKS_CLAIMED=$(echo "$LOGS" | grep -c '"message":"Found and claimed pending task"')
    BATCH_UPDATES=$(echo "$LOGS" | grep -c '"message":"Batch update successful"')

    # Errors and warnings
    ERRORS=$(echo "$LOGS" | grep -c '"level":"error"')
    WARNINGS=$(echo "$LOGS" | grep -c '"level":"warn"')
    ERROR_429=$(echo "$LOGS" | grep -c '429')
    BLOCKING_ERRORS=$(echo "$LOGS" | grep -c 'Blocking error')
    RETRY_SCHEDULED=$(echo "$LOGS" | grep -c 'retry scheduled')

    # Domain limiter activity
    ADAPTIVE_DELAY_UPDATES=$(echo "$LOGS" | grep -c 'Updated domain adaptive delay')
    DOMAIN_DELAY_MSGS=$(echo "$LOGS" | grep -c 'domain.*delay')

    # Performance metrics
    AVG_RESPONSE=$(echo "$LOGS" | grep '"avg_response_time"' | tail -5 | grep -o '"avg_response_time":[0-9]*' | cut -d: -f2 | awk '{sum+=$1; count++} END {if(count>0) print int(sum/count); else print "N/A"}')
    PERFORMANCE_SCALING=$(echo "$LOGS" | grep -c 'Job performance scaling triggered')

    echo "📊 THROUGHPUT (last 2min sample):"
    echo "  Tasks completed: $TASKS_COMPLETED"
    echo "  Tasks claimed:   $TASKS_CLAIMED"
    echo "  Batch updates:   $BATCH_UPDATES"
    echo ""

    echo "⚠️  ERRORS & ISSUES:"
    echo "  Total errors:        $ERRORS"
    echo "  Total warnings:      $WARNINGS"
    echo "  429 responses:       $ERROR_429"
    echo "  Blocking errors:     $BLOCKING_ERRORS"
    echo "  Retries scheduled:   $RETRY_SCHEDULED"
    echo ""

    echo "🔧 DOMAIN RATE LIMITER:"
    echo "  Adaptive delay updates: $ADAPTIVE_DELAY_UPDATES"
    echo "  Domain delay messages:  $DOMAIN_DELAY_MSGS"
    echo ""

    echo "⚡ PERFORMANCE:"
    echo "  Avg response time (ms): $AVG_RESPONSE"
    echo "  Scaling events:         $PERFORMANCE_SCALING"
    echo ""

    # Show any recent errors or adaptive delay updates
    if [ $ERRORS -gt 0 ] || [ $ADAPTIVE_DELAY_UPDATES -gt 0 ] || [ $ERROR_429 -gt 0 ]; then
        echo "🔍 RECENT ISSUES:"
        echo "$LOGS" | grep -E '"level":"error"|Updated domain adaptive delay|429|Blocking error' | tail -5
        echo ""
    fi

    # Show recent completed tasks
    echo "✅ RECENT COMPLETIONS (last 3):"
    echo "$LOGS" | grep '"message":"Crawler completed"' | tail -3 | grep -o '"status_code":[0-9]*' | cut -d: -f2 | awk '{print "  Status: " $1}'
    echo ""

    CHECK_NUM=$((CHECK_NUM + 1))

    # Don't sleep on the last iteration
    if [ $(date +%s) -lt $END_TIME ]; then
        echo "Waiting 60 seconds before next check..."
        echo ""
        sleep $CHECK_INTERVAL
    fi
done

echo "=========================================="
echo "Monitoring complete at $(date)"
echo "=========================================="
