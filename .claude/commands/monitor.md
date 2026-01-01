# Monitor Fly Logs

Sample and analyse Fly.io logs at regular intervals.

## Default usage

```bash
./scripts/monitor_logs.sh
```

Runs for 4 hours with 10-second intervals.

## Options

```bash
./scripts/monitor_logs.sh --run-id "descriptive-name"    # Custom name
./scripts/monitor_logs.sh --interval 30 --iterations 120 # 30s for 1 hour
```

## Output structure

```
logs/YYYYMMDD/HHMM_<name>_<interval>s_<duration>h/
├── raw/
│   ├── <timestamp>_iter1.log
│   ├── <timestamp>_iter2.log
│   └── ...
├── <timestamp>_iter1.json
├── <timestamp>_iter2.json
├── time_series.csv
└── summary.md
```

## What it captures

- Raw log samples from Fly
- JSON summaries per iteration
- Aggregated time series data
- Summary markdown report

Automatic aggregation runs via `scripts/aggregate_logs.py` after each iteration.
