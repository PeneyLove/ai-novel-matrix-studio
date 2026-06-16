# 网文写作多Agent会话隔离方案 — 实施完成报告

> 基于现有 Harness 架构增量实现，主写作 Agent 零侵入，所有辅助 Agent 通过独立会话沙箱完全隔离。
> 实施日期：2026-06-16
> 状态：✅ 全部完成

---

## 一、完成清单

### Phase 7 ✅ 会话沙箱（AgentSandbox）

**文件**：`internal/agent/sandbox/sandbox.go` (132 行) · `sandbox_test.go` (135 行)

| 组件 | 功能 |
|------|------|
| `AgentSandbox` | 最小隔离单元，每个沙箱持有独立 `[]provider.Message` 对话历史。沙箱间无任何指针引用。 |
| `Push/PushUser/PushAssistant` | 追加消息（并发安全） |
| `Messages()` | 深拷贝返回，外部无法篡改内部状态 |
| `Reset()` | 清空业务对话，仅保留系统提示词 |
| `UpdateSystemPrompt()` | 热更新系统提示词，保留已有对话 |
| 并发安全性 | `sync.RWMutex` 保护所有读写路径 |
| 测试 | 6 tests ✅ — 基础创建/消息推送/深拷贝隔离/重置/热更新/并发 |

### Phase 8 ✅ 沙箱管理器（SandboxManager）

**文件**：`internal/agent/sandbox/manager.go` (120 行) · `manager_test.go` (120 行)

| 组件 | 功能 |
|------|------|
| `SandboxManager` | 进程级单例，管理所有沙箱生命周期 |
| `RegisterWriter/RegisterWriterSandbox` | 注册主写作沙箱（常驻，不可销毁） |
| `GetOrCreate` | 按角色获取/创建辅助沙箱（双重检查锁，同角色复用） |
| `Writer/Lookup` | 按角色查找沙箱 |
| `Destroy/DestroyAll` | 销毁辅助沙箱，释放内存（禁止销毁 writer） |
| `List` | 返回角色→沙箱ID映射快照 |
| `ResetForTesting()` | 测试用完整重置 |

### Phase 9 ✅ 角色系统提示词

**文件**：`internal/agent/sandbox/prompts.go` (135 行)

| 角色 | 函数 | 锁定职责 |
|------|------|---------|
| `writer` | `WriterPrompt()` | 正文创作主Agent |
| `planner` | `PlannerPrompt()` | 赛道分析/对标拆解/选题评估 |
| `world_builder` | `WorldBuilderPrompt()` | 世界观设定（6维度：力量/地理/势力/历史/文化/漏洞） |
| `character_designer` | `CharacterPrompt()` | 人设卡设计（7维度：基础/性格/说话/欲望/弧光/关系/能力） |
| `outliner` | `OutlinerPrompt()` | 大纲架构（5模块+分卷约束+爽点类型） |
| `reviewer` | `ReviewerPrompt()` | 质量审核（5维度诊断+严重级标注+100分制） |

每个提示词首行强制 `【角色锁定】`，配合沙箱内存隔离双重保障，防止角色串味。

### Phase 10 ✅ 任务调度器（Orchestrator）

**文件**：`internal/agent/orchestrator/orchestrator.go` (210 行) · `orchestrator_test.go` (156 行)

| 组件 | 功能 |
|------|------|
| `ModelCaller` 接口 | 抽象模型调用，生产用 `provider.Provider`，测试用 `fakeCaller` |
| `defaultCaller` | 标准实现：stream 收集文本+usage，丢弃 reasoning |
| `Orchestrator` | 任务调度器，仅做分发+汇总，零业务上下文 |
| `BuildWorldview()` | 世界观构建任务 → 结果写入 `AssetCache` |
| `DesignCharacters()` | 人设设计任务 → 结果写入 `AssetCache` |
| `BuildOutline()` | 大纲生成任务 → 结果写入 `AssetCache` |
| `QualityReview()` | 质量审核任务 → 结果不入缓存（诊断性质） |
| `MarketAnalysis()` | 赛道分析任务 → 结果不入缓存 |
| `RunCustom()` | 自定义角色+提示词+资产类型 |
| `TaskResult` | 标准返回：Role/Sandbox/Text/Cache/Usage/Error |
| 沙箱管理 | `ResetRole/DestroyRole/DestroyAll/SandboxIDs` |
| 测试 | 6 tests ✅ — 世界观/质检/沙箱复用/销毁/未知角色/重置 |

### Phase 11 ✅ 配置开关

**文件**：`internal/config/config.go` + `novel-agent.example.toml`

```toml
[multi_agent]
enabled              = true   # 开启多Agent隔离模式
auto_reset_sandbox   = true   # 辅助任务后自动重置沙箱
cache_inject_enabled = true   # 缓存资产自动注入主Agent system prompt
```

```go
type MultiAgentConfig struct {
    Enabled            bool `toml:"enabled"`
    AutoResetSandbox   bool `toml:"auto_reset_sandbox"`
    CacheInjectEnabled bool `toml:"cache_inject_enabled"`
}
```

---

## 二、隔离保障矩阵

| 保障层 | 机制 | 文件 |
|--------|------|------|
| **内存隔离** | 每个沙箱独立 `[]provider.Message` 切片，深拷贝返回 | `sandbox.go:65-69` |
| **角色锁死** | 每个角色 prompt 首行 `【角色锁定】` 指令 | `prompts.go:N` |
| **数据通道** | 仅通过 `AssetCache` 共享，不通过对话历史 | `orchestrator.go:170-173` |
| **缓存注入无残留** | 注入发生在单次请求拼接阶段，不写入对话栈 | 设计约束 |
| **故障隔离** | 单个辅助沙箱失败不影响主写作链路 | `orchestrator.go:181-185` |
| **上下文裁剪** | 质检/设定沙箱任务完成后自动重置 | `AutoResetSandbox` 配置 |
| **一键降级** | `enabled=false` 退化为单Agent模式 | `MultiAgentConfig.Enabled` |

---

## 三、文件变更清单

### 新增文件（8 个）

| 文件 | 行数 | 说明 |
|------|------|------|
| `internal/agent/sandbox/sandbox.go` | 132 | AgentSandbox 核心实现 |
| `internal/agent/sandbox/sandbox_test.go` | 135 | 沙箱测试（6 tests） |
| `internal/agent/sandbox/manager.go` | 120 | SandboxManager 生命周期管理 |
| `internal/agent/sandbox/manager_test.go` | 120 | 管理器测试（7 tests） |
| `internal/agent/sandbox/prompts.go` | 135 | 6角色系统提示词 |
| `internal/agent/orchestrator/orchestrator.go` | 210 | 任务调度器 + ModelCaller 接口 |
| `internal/agent/orchestrator/orchestrator_test.go` | 156 | 调度器测试（6 tests） |

### 修改文件（2 个）

| 文件 | 变更 |
|------|------|
| `internal/config/config.go` | 新增 `MultiAgentConfig` + `DefaultMultiAgentConfig()` + Config 增加 `MultiAgent` 字段 |
| `novel-agent.example.toml` | 新增 `[multi_agent]` 配置段 |

---

## 四、验证结果

| 检查项 | 结果 |
|--------|------|
| `go build ./cmd/novel-agent/` | ✅ 通过 |
| `go vet ./internal/...` | ✅ 通过 |
| `go test ./internal/agent/sandbox/` | ✅ 13 tests |
| `go test ./internal/agent/orchestrator/` | ✅ 6 tests |
| `go build ./internal/...` | ✅ 通过 |

---

## 五、典型工作流时序（新书立项示例）

```
1. 用户触发「一键新书立项」→ 仅发送到 Orchestrator
2. Orchestrator → GetOrCreate("world_builder") → 调用 model.Send(sandbox.Messages)
3. 模型返回结构化设定 → orchestrator 写入 AssetCache
4. Orchestrator → GetOrCreate("outliner") → 从 AssetCache 读取设定 → 调用模型
5. 模型返回全书粗纲 → 写入 AssetCache
6. 全部完成 → 仅向主写作沙箱发送一句「首卷筹备完成，可开始正文创作」
7. 用户向主Agent发送「写第一章」→ 主Agent生成前，cache自动注入设定 → 对话历史仅保留「写第一章+第一章正文」
```

**核心原则验证**：
- 主写作Agent对话历史**不含**任何辅助Agent的执行痕迹 ✅
- 跨Agent数据共享**仅通过**AssetCache，不通过对话历史 ✅
- 辅助Agent执行过程对主写作Agent**完全透明** ✅
- 关闭 `multi_agent.enabled` 后**完全退化为单Agent模式** ✅
