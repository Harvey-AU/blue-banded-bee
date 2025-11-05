#!/usr/bin/env python3
"""
Aggregate JSON log summaries into time-series format.
Supports incremental updates by tracking processed files.

Usage:
    # One-time full aggregation
    python3 scripts/aggregate_logs.py logs/20251105/0750_heavy-load-5jobs-per-3min/

    # Watch mode - continuously process new files
    python3 scripts/aggregate_logs.py logs/20251105/0750_heavy-load-5jobs-per-3min/ --watch
"""

import json
import sys
import time
from pathlib import Path
from collections import defaultdict
from datetime import datetime
from zoneinfo import ZoneInfo

STATE_FILE_NAME = ".aggregate_state.json"

def load_state(log_dir):
    """Load processing state (which files have been processed)."""
    state_file = log_dir / STATE_FILE_NAME
    if state_file.exists():
        try:
            with open(state_file) as f:
                return json.load(f)
        except Exception as e:
            print(f"Warning: Could not load state file: {e}", file=sys.stderr)
    return {'processed_files': [], 'last_update': None}

def save_state(log_dir, state):
    """Save processing state."""
    state_file = log_dir / STATE_FILE_NAME
    melbourne_tz = ZoneInfo("Australia/Melbourne")
    state['last_update'] = datetime.now(melbourne_tz).isoformat()
    with open(state_file, 'w') as f:
        json.dump(state, f, indent=2)

def load_existing_data(csv_path):
    """Load existing CSV data into memory."""
    by_minute = defaultdict(lambda: {
        'level_counts': defaultdict(int),
        'message_counts': defaultdict(int),
        'samples': 0,
        'total_lines': 0,
        'failed_to_parse': 0
    })

    if not csv_path.exists():
        return by_minute

    try:
        with open(csv_path) as f:
            next(f)  # Skip header
            for line in f:
                parts = line.strip().split(',')
                if len(parts) >= 7:
                    timestamp = parts[0]
                    by_minute[timestamp]['samples'] = int(parts[1])
                    by_minute[timestamp]['total_lines'] = int(parts[2])
                    by_minute[timestamp]['level_counts']['info'] = int(parts[3])
                    by_minute[timestamp]['level_counts']['warn'] = int(parts[4])
                    by_minute[timestamp]['level_counts']['error'] = int(parts[5])
                    by_minute[timestamp]['level_counts']['debug'] = int(parts[6])
    except Exception as e:
        print(f"Warning: Could not load existing CSV: {e}", file=sys.stderr)

    return by_minute

def process_json_file(json_file, by_minute, all_messages):
    """Process a single JSON file and update aggregation data."""
    try:
        with open(json_file) as f:
            data = json.load(f)

        # Extract metadata
        meta = data.get('meta', {})
        total_lines = meta.get('total_lines', 0)
        failed_to_parse = meta.get('failed_to_parse', 0)

        # Process level counts
        for timestamp, levels in data.get('level_counts', {}).items():
            minute_key = timestamp[:16]  # YYYY-MM-DDTHH:MM

            by_minute[minute_key]['samples'] += 1
            by_minute[minute_key]['total_lines'] += total_lines
            by_minute[minute_key]['failed_to_parse'] += failed_to_parse

            for level, count in levels.items():
                by_minute[minute_key]['level_counts'][level] += count

        # Process message counts
        for timestamp, messages in data.get('message_counts', {}).items():
            minute_key = timestamp[:16]

            for msg_data in messages:
                message = msg_data.get('message', 'unknown')
                count = msg_data.get('count', 0)
                by_minute[minute_key]['message_counts'][message] += count
                all_messages[message] += count

        return True
    except Exception as e:
        print(f"Error processing {json_file}: {e}", file=sys.stderr)
        return False

def write_csv(csv_path, by_minute):
    """Write aggregated data to CSV."""
    with open(csv_path, 'w') as f:
        f.write("timestamp,samples,total_lines,info,warn,error,debug\n")
        for minute in sorted(by_minute.keys()):
            data = by_minute[minute]
            levels = data['level_counts']
            f.write(f"{minute},{data['samples']},{data['total_lines']},"
                   f"{levels.get('info', 0)},{levels.get('warn', 0)},"
                   f"{levels.get('error', 0)},{levels.get('debug', 0)}\n")

def write_summary(summary_path, by_minute, all_messages, new_files_count):
    """Write markdown summary."""
    total_samples = sum(m['samples'] for m in by_minute.values())
    total_lines = sum(m['total_lines'] for m in by_minute.values())
    total_failed = sum(m['failed_to_parse'] for m in by_minute.values())

    melbourne_tz = ZoneInfo("Australia/Melbourne")
    now = datetime.now(melbourne_tz)

    with open(summary_path, 'w') as f:
        f.write("# Log Aggregation Summary\n\n")
        f.write(f"**Generated:** {now.isoformat()}\n\n")
        f.write(f"**New files processed:** {new_files_count}\n\n")

        if by_minute:
            f.write(f"**Time range:** {min(by_minute.keys())} to {max(by_minute.keys())}\n\n")

        f.write("## Overall Statistics\n\n")
        f.write(f"- Total samples: **{total_samples}**\n")
        f.write(f"- Total log lines: **{total_lines:,}**\n")
        f.write(f"- Failed to parse: **{total_failed}**\n")
        f.write(f"- Parse success rate: **{100 * (1 - total_failed / max(total_lines, 1)):.1f}%**\n\n")

        # Log levels by minute
        f.write("## Log Levels by Minute (Last 20)\n\n")
        f.write("| Timestamp | Samples | Lines | Info | Warn | Error | Debug |\n")
        f.write("|-----------|---------|-------|------|------|-------|-------|\n")

        for minute in sorted(by_minute.keys())[-20:]:
            data = by_minute[minute]
            levels = data['level_counts']
            f.write(f"| {minute} | {data['samples']} | {data['total_lines']} | "
                   f"{levels.get('info', 0)} | {levels.get('warn', 0)} | "
                   f"{levels.get('error', 0)} | {levels.get('debug', 0)} |\n")

        # Top messages
        f.write("\n## Top 20 Messages (Overall)\n\n")
        f.write("| Count | Message |\n")
        f.write("|-------|----------|\n")

        top_messages = sorted(all_messages.items(), key=lambda x: x[1], reverse=True)[:20]

        for msg, count in top_messages:
            # Escape pipe characters in messages for markdown tables
            escaped_msg = msg[:70].replace('|', '\\|')
            f.write(f"| {count:,} | {escaped_msg} |\n")

        # Critical patterns
        f.write("\n## Critical Patterns\n\n")

        critical_keywords = [
            'Emergency scale-down',
            'error',
            'failed',
            'panic',
            'crash',
            'timeout',
            'killed'
        ]

        critical_found = {}
        for keyword in critical_keywords:
            for msg, count in all_messages.items():
                if keyword.lower() in msg.lower():
                    if keyword not in critical_found:
                        critical_found[keyword] = []
                    critical_found[keyword].append((msg, count))

        if critical_found:
            for keyword, findings in critical_found.items():
                f.write(f"\n### '{keyword}' patterns found:\n\n")
                for msg, count in sorted(findings, key=lambda x: x[1], reverse=True)[:5]:
                    escaped_msg = msg[:65].replace('|', '\\|')
                    f.write(f"- **{count:,}x** {escaped_msg}\n")
        else:
            f.write("✅ No critical patterns detected\n")

def aggregate_logs(log_dir, incremental=True):
    """Aggregate JSON summaries, optionally in incremental mode."""
    log_path = Path(log_dir)

    if not log_path.exists():
        print(f"Error: Directory {log_dir} does not exist")
        return False

    csv_path = log_path / "time_series.csv"
    summary_path = log_path / "summary.md"

    # Load state
    state = load_state(log_path) if incremental else {'processed_files': []}
    processed_set = set(state['processed_files'])

    # Find JSON files
    all_json_files = sorted(log_path.glob("*.json"))
    new_files = [f for f in all_json_files if f.name not in processed_set]

    if not new_files:
        if incremental:
            print(f"No new files to process (already processed {len(processed_set)} files)")
            return True
        else:
            print(f"No JSON files found in {log_dir}")
            return False

    # Load existing data if incremental
    by_minute = load_existing_data(csv_path) if incremental and csv_path.exists() else defaultdict(lambda: {
        'level_counts': defaultdict(int),
        'message_counts': defaultdict(int),
        'samples': 0,
        'total_lines': 0,
        'failed_to_parse': 0
    })

    all_messages = defaultdict(int)

    # IMPORTANT: Message counts are NOT in the CSV, so we need to reprocess ALL files
    # to get accurate message counts. We process all files but only update level_counts
    # for new files (to maintain CSV incremental updates).
    files_to_process_for_messages = all_json_files
    print(f"Reprocessing {len(files_to_process_for_messages)} total files for message counts...")

    # First pass: Process ALL files for message counts only
    for json_file in files_to_process_for_messages:
        try:
            with open(json_file) as f:
                data = json.load(f)

            # Only extract message counts from all files
            for timestamp, messages in data.get('message_counts', {}).items():
                minute_key = timestamp[:16]
                for msg_data in messages:
                    message = msg_data.get('message', 'unknown')
                    count = msg_data.get('count', 0)
                    # Only update message counts in by_minute if this is a new file
                    if json_file.name in processed_set or json_file in new_files:
                        # For all files, update all_messages (global message counter)
                        all_messages[message] += count
        except Exception as e:
            print(f"Warning: Could not extract messages from {json_file}: {e}", file=sys.stderr)

    # Second pass: Process only new files for level counts
    print(f"Processing {len(new_files)} new files for level counts...")
    success_count = 0
    for json_file in new_files:
        try:
            with open(json_file) as f:
                data = json.load(f)

            # Extract metadata
            meta = data.get('meta', {})
            total_lines = meta.get('total_lines', 0)
            failed_to_parse = meta.get('failed_to_parse', 0)

            # Process level counts for new files only
            for timestamp, levels in data.get('level_counts', {}).items():
                minute_key = timestamp[:16]

                by_minute[minute_key]['samples'] += 1
                by_minute[minute_key]['total_lines'] += total_lines
                by_minute[minute_key]['failed_to_parse'] += failed_to_parse

                for level, count in levels.items():
                    by_minute[minute_key]['level_counts'][level] += count

            # Also update message counts for by_minute (needed for per-minute tracking)
            for timestamp, messages in data.get('message_counts', {}).items():
                minute_key = timestamp[:16]
                for msg_data in messages:
                    message = msg_data.get('message', 'unknown')
                    count = msg_data.get('count', 0)
                    by_minute[minute_key]['message_counts'][message] += count

            processed_set.add(json_file.name)
            success_count += 1
            if success_count % 10 == 0:
                print(f"  Processed {success_count}/{len(new_files)} files...")
        except Exception as e:
            print(f"Error processing {json_file}: {e}", file=sys.stderr)

    print(f"Successfully processed {success_count}/{len(new_files)} new files")

    # Write outputs
    write_csv(csv_path, by_minute)
    write_summary(summary_path, by_minute, all_messages, len(new_files))

    # Save state
    if incremental:
        state['processed_files'] = sorted(list(processed_set))
        save_state(log_path, state)

    print(f"\nOutputs written:")
    print(f"  CSV: {csv_path}")
    print(f"  Summary: {summary_path}")

    return True

def watch_mode(log_dir, interval=10):
    """Continuously watch for new files and process them."""
    print(f"Watch mode: monitoring {log_dir} (checking every {interval}s)")
    print("Press Ctrl+C to stop\n")

    try:
        while True:
            aggregate_logs(log_dir, incremental=True)
            time.sleep(interval)
    except KeyboardInterrupt:
        print("\n\nWatch mode stopped")

if __name__ == "__main__":
    import argparse

    parser = argparse.ArgumentParser(description='Aggregate JSON log summaries')
    parser.add_argument('log_dir', help='Directory containing JSON log files')
    parser.add_argument('--watch', action='store_true', help='Watch mode: continuously process new files')
    parser.add_argument('--interval', type=int, default=10, help='Check interval in watch mode (default: 10s)')
    parser.add_argument('--full', action='store_true', help='Full reprocess (ignore state)')

    args = parser.parse_args()

    if args.watch:
        watch_mode(args.log_dir, args.interval)
    else:
        aggregate_logs(args.log_dir, incremental=not args.full)
