#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

APP="blue-banded-bee"
INTERVAL=60
SAMPLES=400
ITERATIONS=60
RUN_ID=""
OUTPUT_ROOT="logs"

usage() {
    cat <<'USAGE'
Usage: monitor_logs.sh [options]

Fetch recent Fly logs on a fixed cadence, archive the raw output, and write
per-minute summaries describing how often each log level/message occurred.

Options:
  --app NAME          Fly application name (default: blue-banded-bee)
  --interval SECONDS  Seconds to wait between samples (default: 60)
  --samples N         Number of log lines to request each run (default: 400)
  --iterations N      Number of iterations to perform (0 = run forever)
  --run-id ID         Identifier used when naming output directories
  -h, --help          Show this message and exit

Environment variables with the same names (APP, INTERVAL, SAMPLES, ITERATIONS,
RUN_ID) override the defaults as well.
USAGE
}

# Allow environment variables to override defaults
APP=${APP:-$APP}
INTERVAL=${INTERVAL:-$INTERVAL}
SAMPLES=${SAMPLES:-$SAMPLES}
ITERATIONS=${ITERATIONS:-$ITERATIONS}
RUN_ID=${RUN_ID:-$RUN_ID}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --app)
            APP="$2"
            shift 2
            ;;
        --interval)
            INTERVAL="$2"
            shift 2
            ;;
        --samples)
            SAMPLES="$2"
            shift 2
            ;;
        --iterations)
            ITERATIONS="$2"
            shift 2
            ;;
        --run-id)
            RUN_ID="$2"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "Unknown option: $1" >&2
            usage
            exit 1
            ;;
    esac
done

if ! [[ "$INTERVAL" =~ ^[0-9]+$ && "$INTERVAL" -gt 0 ]]; then
    echo "interval must be a positive integer" >&2
    exit 1
fi

if ! [[ "$SAMPLES" =~ ^[0-9]+$ && "$SAMPLES" -ge 1 && "$SAMPLES" -le 10000 ]]; then
    echo "samples must be an integer between 1 and 10000" >&2
    exit 1
fi

if ! [[ "$ITERATIONS" =~ ^[0-9]+$ ]]; then
    echo "iterations must be an integer >= 0" >&2
    exit 1
fi

if [[ -z "$RUN_ID" ]]; then
    RUN_ID=$(date -u +"%Y%m%d_%H%M%S")
fi

RAW_DIR="$OUTPUT_ROOT/raw_${RUN_ID}"
SUMMARY_DIR="$OUTPUT_ROOT/summary_${RUN_ID}"
LOG_FILE="$OUTPUT_ROOT/monitor_${RUN_ID}.log"

mkdir -p "$RAW_DIR" "$SUMMARY_DIR"

echo "[$(date -u +"%Y-%m-%dT%H:%M:%SZ")] Starting log monitor" | tee -a "$LOG_FILE"
echo "App: $APP | Interval: ${INTERVAL}s | Samples: $SAMPLES | Iterations: $ITERATIONS" | tee -a "$LOG_FILE"
echo "Raw logs: $RAW_DIR" | tee -a "$LOG_FILE"
echo "Summaries: $SUMMARY_DIR" | tee -a "$LOG_FILE"

iteration=0

while true; do
    iteration=$((iteration + 1))

    ts=$(date -u +"%Y%m%dT%H%M%SZ")
    raw_file="$RAW_DIR/${ts}_iter${iteration}.log"
    summary_file="$SUMMARY_DIR/${ts}_iter${iteration}.json"

    echo "[$(date -u +"%Y-%m-%dT%H:%M:%SZ")] Iteration $iteration: capturing logs" | tee -a "$LOG_FILE"

    if flyctl logs --app "$APP" --no-tail --max-lines "$SAMPLES" > "$raw_file" 2>&1; then
        if ! python3 "$SCRIPT_DIR/process_logs.py" "$raw_file" "$summary_file" >> "$LOG_FILE" 2>&1; then
            echo "[$(date -u +"%Y-%m-%dT%H:%M:%SZ")] Failed to process logs (see output above)" | tee -a "$LOG_FILE"
        fi
    else
        echo "[$(date -u +"%Y-%m-%dT%H:%M:%SZ")] Failed to fetch logs from Fly; raw output stored in $raw_file" | tee -a "$LOG_FILE"
    fi

    if [[ "$ITERATIONS" -ne 0 && "$iteration" -ge "$ITERATIONS" ]]; then
        break
    fi

    sleep "$INTERVAL"
done

echo "[$(date -u +"%Y-%m-%dT%H:%M:%SZ")] Monitoring finished after $iteration iteration(s)" | tee -a "$LOG_FILE"
