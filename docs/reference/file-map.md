# Blue Banded Bee File Map

```
blue-banded-bee/
├── .env
├── .air.toml
├── .gitignore
├── CHANGELOG.md
├── CONTRIBUTING.md
├── INIT.md
├── README.md
├── ROADMAP.md
├── SECURITY.md
├── TODO.md
├── fly.toml
├── go.mod
├── go.sum
│
├── cmd/
│   ├── app/
│   │   ├── main.go
│   │   └── main_test.go
│   │
│   ├── pg-test/
│   │   └── main.go
│   │
│   └── test_jobs/
│       └── main.go
│
├── internal/
│   ├── common/
│   │   └── queue.go
│   │
│   ├── crawler/
│   │   ├── config.go
│   │   ├── crawler.go
│   │   ├── crawler_test.go
│   │   ├── sitemap.go
│   │   └── types.go
│   │
│   ├── db/
│   │   ├── db.go
│   │   ├── health.go
│   │   ├── queue.go
│   │   └── worker.go
│   │
│   └── jobs/
│       ├── constants_test.go
│       ├── db.go
│       ├── manager.go
│       ├── queue_helpers.go
│       ├── types.go
│       └── worker.go
│
├── docs/
│   ├── architecture/
│   │   ├── gotchas.md
│   │   ├── implementation-details.md
│   │   ├── jobs.md
│   │   ├── mental-model.md
│   │   └── quick-reference.md
│   │
│   ├── guides/
│   │   ├── deployment.md
│   │   └── development.md
│   │
│   └── reference/
│       ├── api-reference.md
│       ├── codebase-structure.md
│       ├── database-config.md
│       └── file-map.md
│
└── .github/
    ├── ISSUE_TEMPLATE/
    │   ├── bug_report.md
    │   └── feature_request.md
    │
    └── workflows/
        └── fly-deploy.yml
```
