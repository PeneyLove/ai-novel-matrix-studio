# Legacy v0.x — Python Implementation (Maintenance Mode)

**This branch is frozen.** Only critical bug fixes will be merged.
No new features are accepted here.

## Why was this archived?

The v0.x architecture (Python + FastAPI + Celery + PyQt6 + Docker) proved too
heavy for the target audience of individual creators. Key pain points:

- **Deployment burden**: MySQL + MongoDB + Redis + Docker required for basic use
- **Extension friction**: Adding a new agent genre meant editing Python source
- **No package distribution**: Users had to clone the repo and run docker compose
- **Tight coupling**: Crawler/corpus/model/pipeline layers were deeply intertwined

## What replaces it?

See the `main` (v1.x) branch — a Go single-binary harness with pluggable skills,
local file storage under `.novelAgent/`, and npm-based distribution.

## Migration

If you have data in this v0.x system, use the v1.x migration tool:

```bash
novel-agent migrate --from v0.x --mysql-url "mysql+aiomysql://..." --mongodb-url "mongodb://..."
```

## Last release

- **Tag**: v0.9.0
- **Date**: 2025-01-15
- **Status**: stable for existing installations, no further development
