#!/bin/bash

# Monitor Blue Banded Bee throughput over 30 minutes
# Checks every 3 minutes

DURATION=1800  # 30 minutes
INTERVAL=180   # 3 minutes
END_TIME=$(($(date +%s) + DURATION))
CHECK_COUNT=0

echo "=== Blue Banded Bee Throughput Monitor ==="
echo "Started at: $(date)"
echo "Will run for 30 minutes with checks every 3 minutes"
echo ""

while [ $(date +%s) -lt $END_TIME ]; do
    CHECK_COUNT=$((CHECK_COUNT + 1))
    echo "=========================================="
    echo "Check #${CHECK_COUNT} at $(date)"
    echo "=========================================="

    # Get recent logs and analyse
    LOGS=$(flyctl logs --app blue-banded-bee --no-tail 2>&1 | head -200)

    # Count task completions in last few minutes
    COMPLETED=$(echo "$LOGS" | grep -c "Crawler completed")

    # Count errors
    ERRORS=$(echo "$LOGS" | grep -c '"level":"error"')

    # Count warnings
    WARNINGS=$(echo "$LOGS" | grep -c '"level":"warn"')

    # Count cache warnings
    CACHE_WARNINGS=$(echo "$LOGS" | grep -c "Cache did not become available")

    # Count database transaction failures
    DB_FAILURES=$(echo "$LOGS" | grep -c "Database transaction failed")

    # Count "no rows in result set" errors
    NO_ROWS=$(echo "$LOGS" | grep -c "sql: no rows in result set")

    # Count tasks found and claimed
    CLAIMED=$(echo "$LOGS" | grep -c "Found and claimed pending task")

    # Count performance scaling events
    SCALING=$(echo "$LOGS" | grep -c "Job performance scaling triggered")

    # Extract recent response times
    AVG_RESPONSE=$(echo "$LOGS" | grep "avg_response_time" | tail -5 | grep -o '"avg_response_time":[0-9]*' | cut -d: -f2)

    echo ""
    echo "📊 THROUGHPUT METRICS:"
    echo "  Tasks completed (in sample): $COMPLETED"
    echo "  Tasks claimed (in sample):   $CLAIMED"
    echo "  Performance scaling events:  $SCALING"

    echo ""
    echo "⚠️  ERROR & WARNING METRICS:"
    echo "  Total errors:                $ERRORS"
    echo "  Total warnings:              $WARNINGS"
    echo "  Cache warnings:              $CACHE_WARNINGS"
    echo "  DB transaction failures:     $DB_FAILURES"
    echo "  'No rows' errors:            $NO_ROWS"

    if [ -n "$AVG_RESPONSE" ]; then
        echo ""
        echo "⏱️  RECENT RESPONSE TIMES (ms):"
        echo "$AVG_RESPONSE" | head -5
    fi

    # Show some sample recent errors
    echo ""
    echo "🔍 SAMPLE RECENT ACTIVITY:"
    echo "$LOGS" | grep -E '"level":"(error|warn)"' | tail -3

    echo ""
    echo "Waiting 3 minutes before next check..."
    echo ""

    # Only sleep if not the last iteration
    if [ $(date +%s) -lt $((END_TIME - INTERVAL)) ]; then
        sleep $INTERVAL
    else
        break
    fi
done

echo ""
echo "=========================================="
echo "Monitoring completed at: $(date)"
echo "Total checks performed: $CHECK_COUNT"
echo "=========================================="
