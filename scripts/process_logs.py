#!/usr/bin/env python3

"""Utility to transform raw Fly.io logs into simple per-minute summaries."""

from __future__ import annotations

import json
import sys
from collections import Counter, defaultdict
from datetime import datetime
from pathlib import Path
from typing import Any, Dict, Iterable, Tuple
from zoneinfo import ZoneInfo


def _normalise_timestamp(record: Dict[str, Any]) -> str:
    """Return an ISO minute string for a log record."""

    for key in ("time", "timestamp", "@timestamp", "ts", "created_at"):
        if key in record and record[key]:
            raw = str(record[key])
            break
    else:
        return "unknown"

    cleaned = raw.replace("Z", "+00:00")

    try:
        dt = datetime.fromisoformat(cleaned)
    except ValueError:
        # Fallback: take first 16 characters (YYYY-MM-DDTHH:MM)
        return raw[:16] if len(raw) >= 16 else raw

    return dt.strftime("%Y-%m-%dT%H:%M")


def _iter_records(lines: Iterable[str]) -> Iterable[Tuple[Dict[str, Any], str]]:
    for line in lines:
        idx = line.find("{")
        if idx == -1:
            yield None, line  # type: ignore[misc]
            continue
        payload = line[idx:]
        try:
            data = json.loads(payload)
        except json.JSONDecodeError:
            yield None, line  # type: ignore[misc]
            continue
        yield data, line


def summarise_logs(raw_path: Path) -> Dict[str, Any]:
    level_counts: Dict[str, Counter] = defaultdict(Counter)
    message_counts: Dict[str, Counter] = defaultdict(Counter)

    total = 0
    parsed = 0
    errors = 0

    with raw_path.open("r", encoding="utf-8", errors="ignore") as handle:
        for record, original in _iter_records(handle):
            total += 1
            if record is None:
                errors += 1
                continue

            parsed += 1
            minute = _normalise_timestamp(record)

            level = str(record.get("level", "unknown"))
            level_counts[minute][level] += 1

            message = str(record.get("message", "<no message>"))
            message_counts[minute][message] += 1

    message_summary: Dict[str, Any] = {}
    for minute, counter in message_counts.items():
        top = counter.most_common(20)
        message_summary[minute] = [
            {"message": message, "count": count} for message, count in top
        ]

    # Generate timestamp in Melbourne timezone
    melbourne_tz = ZoneInfo("Australia/Melbourne")
    now = datetime.now(melbourne_tz)

    summary = {
        "meta": {
            "source": str(raw_path),
            "total_lines": total,
            "parsed": parsed,
            "failed_to_parse": errors,
            "generated_at": now.isoformat(),
        },
        "level_counts": {minute: dict(counter) for minute, counter in level_counts.items()},
        "message_counts": message_summary,
    }

    return summary


def main() -> int:
    if len(sys.argv) != 3:
        print("Usage: process_logs.py <raw_log_file> <output_json>", file=sys.stderr)
        return 1

    raw_path = Path(sys.argv[1])
    output_path = Path(sys.argv[2])

    if not raw_path.exists():
        print(f"Raw log file not found: {raw_path}", file=sys.stderr)
        return 1

    summary = summarise_logs(raw_path)

    output_path.write_text(json.dumps(summary, indent=2, sort_keys=True), encoding="utf-8")

    meta = summary["meta"]
    print(
        f"Processed {meta['parsed']}/{meta['total_lines']} lines from {raw_path.name};"
        f" summary written to {output_path.name}"
    )

    return 0


if __name__ == "__main__":
    sys.exit(main())
