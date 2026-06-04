# AI Novel Agent · v1.x

> Go single-binary harness with pluggable skills.  
> `npm install -g ai-novel-agent` → `novel-agent init` → `novel-agent run`

## Branches

| Branch | Version | Status |
|--------|---------|--------|
| **main** (v1.x) | 1.0.0-alpha | Active development |
| [v0.x](https://gitee.com/penney-101/ai-novel-matrix-studio/tree/v0.x/) | 0.9.0 | Frozen — Python/FastAPI/Celery/PyQt6 |

> If you're looking for the legacy Python system, switch to the `v0.x` branch.

## Quick Start

```bash
# Install
npm install -g ai-novel-agent

# Initialize a project directory
novel-agent init
cd my-novel-project/

# Run a skill
novel-agent run --skill female_rebirth --stage topic_generation \
  --input '{"trend_data":"重生虐渣文持续霸榜"}'

# Full pipeline
novel-agent pipeline --skill female_rebirth \
  --trend-data "近期热榜：重生、穿越、虐渣"

# Export output
novel-agent export --task-id <id> --format docx
```

## Architecture

```
novel-agent (Go single binary)
  ├── Harness —— skill lifecycle, model router, pipeline orchestrator
  ├── Skill —— YAML-defined pluggable agent (prompt templates + model bindings)
  └── Storage —— .novelAgent/ local directory (no external DB needed)
```

## .novelAgent/ Directory

```
.novelAgent/
├── config.yaml          # Global config (API keys, defaults)
├── skills/              # Installed skill definitions
│   ├── female_rebirth/
│   │   └── skill.yaml
│   ├── male_power/
│   ├── suspense/
│   └── romance/
├── corpus/              # Local corpus cache (optional)
├── outputs/             # Generated content by task_id
└── traces/              # Copyright trace records (.jsonl)
```

## Built-in Skills

| Skill | Genre | Supported Stages |
|-------|-------|-----------------|
| `female_rebirth` | Women's fiction — rebirth/revenge | topic, outline, content, polish |
| `male_power` | Men's fiction — urban powers | topic, outline, content, polish |
| `suspense` | Mystery/thriller short stories | topic, outline, content, polish |
| `romance` | Sweet romance | topic, outline, content, polish |

## Model Routing

Each skill YAML declares which AI model handles each stage:

| Stage | Default Model | Fallback |
|-------|--------------|----------|
| topic_generation | MiniMax | Qwen |
| outline_generation | Doubao | Qwen |
| content_generation | Qwen | — |
| polish | DeepSeek | Qwen |

## Security

- API keys encrypted at rest (AES-256-GCM, derived from machine fingerprint)
- Skills sandboxed to `.novelAgent/` — no filesystem escape
- Model endpoints whitelisted in `config.yaml`

## Development

```bash
# Build
go build -o novel-agent ./cmd/novel-agent/

# Run tests
go test ./internal/... -v -cover

# Cross-compile
make build    # outputs binaries in /dist/
```

## License

MIT
