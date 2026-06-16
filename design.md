抱歉，上一版误输出了 TypeScript 实现，以下为**纯 Go 语言版本**的多Agent会话隔离方案，完全对齐你现有 `internal/agent`、`internal/cache` 的项目结构，可直接保存为 Markdown 设计文档使用。

# 网文写作多Agent会话隔离方案设计（Go版 · 单用户本地工具）
> 基于现有 Harness 架构增量实现，主写作 Agent 零侵入，所有辅助 Agent 通过独立会话沙箱完全隔离，等价于独立对话窗口，互不干扰
> 隔离形态：内存级会话沙箱，单进程内实现，无额外服务依赖
> 适配场景：单用户本地 Agent 工具包

---

## 一、方案概述
### 1.1 设计目标
在不改动现有主写作 Agent 核心逻辑的前提下，新增策划、设定、大纲、质检等辅助角色 Agent，实现：
- 每个 Agent 拥有独立的对话上下文，完全隔离，不会出现角色串线、上下文污染
- 跨 Agent 数据仅通过三级缓存引擎共享，绝不通过对话历史传递
- 辅助 Agent 的执行过程对主写作 Agent 透明，无感知、无侵入
- 轻量化实现，兼容单用户本地使用场景，支持开关一键降级为单 Agent 模式

### 1.2 核心设计原则
1. **上下文完全隔离**：每个 Agent 持有独立会话实例，对话历史内存地址分离，沙箱间不可互相读写
2. **数据共享只走缓存**：设定、大纲、人设等资产仅通过结构化缓存传递，不塞入对话栈
3. **主Agent零侵入**：原有写作 Agent 代码、调用链路、对话逻辑完全保留，不做任何修改
4. **调度层无业务上下文**：调度器仅负责任务分发与结果汇总，不积累业务对话历史

---

## 二、整体隔离架构
### 2.1 架构分层
```
┌─────────────────────────────────────────────────────────┐
│                    用户调用入口                          │
│  写作指令 → 主写作沙箱；辅助任务 → 调度器分发            │
└───────────┬───────────────────────────┬─────────────────┘
            │                           │
            ▼                           ▼
┌─────────────────────┐     ┌─────────────────────────────┐
│  主写作Agent沙箱     │     │         任务调度器          │
│  独立会话历史        │     │  仅分发指令、汇总结果        │
│  独立系统Prompt      │     │  不持有业务上下文            │
│  （原有逻辑零修改）  │     └───────────┬─────────────────┘
└───────────┬─────────┘                 │
            │                           │ 调度独立沙箱执行任务
            │  只读注入资产              ▼
            │                ┌─────────────────────────┐
            │                │  辅助Agent沙箱池        │
            │                │  · 策划Agent沙箱        │
            │                │  · 设定Agent沙箱        │
            │                │  · 大纲Agent沙箱        │
            │                │  · 质检Agent沙箱        │
            │                │  每个沙箱独立对话历史    │
            │                └───────────┬─────────────┘
            │                            │
            │  结构化读写资产              │
            └─────────────┬──────────────┘
                          ▼
            ┌─────────────────────────────┐
            │     三级缓存引擎（共享）     │
            │  全局资产/语义片段/剧情摘要  │
            │  唯一跨Agent数据通道        │
            └─────────────────────────────┘
```

### 2.2 核心隔离逻辑
1. 主写作 Agent 与所有辅助 Agent 分别持有独立的会话沙箱实例，对话历史切片完全独立，无指针引用
2. 辅助 Agent 的执行过程、推导逻辑、对话内容，全程不会写入主写作 Agent 的对话栈
3. 跨 Agent 数据同步仅通过缓存引擎完成：辅助 Agent 生成结果 → 写入缓存 → 主 Agent 生成时自动读取注入
4. 缓存注入为**单次临时拼接**，仅作用于当次模型请求，不会持久化到对话历史中

---

## 三、核心模块 Go 实现
### 3.1 会话沙箱（AgentSandbox）
最小隔离单元，每个 Agent 对应一个沙箱实例，等价于一个独立对话窗口，并发安全。

```go
// internal/agent/sandbox/sandbox.go
package sandbox

import "sync"

// ChatMessage 对话消息结构
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AgentSandbox Agent会话沙箱
// 完全隔离的对话上下文，沙箱间无任何数据共享
type AgentSandbox struct {
	sandboxID    string
	roleType     string
	systemPrompt string
	messageStack []ChatMessage
	mu           sync.RWMutex
}

// NewAgentSandbox 创建新的独立沙箱
func NewAgentSandbox(roleType string, systemPrompt string) *AgentSandbox {
	return &AgentSandbox{
		sandboxID:    "sandbox_" + roleType + "_" + randomString(8),
		roleType:     roleType,
		systemPrompt: systemPrompt,
		messageStack: []ChatMessage{
			{Role: "system", Content: systemPrompt},
		},
	}
}

// PushUserMessage 追加用户指令，仅写入当前沙箱
func (s *AgentSandbox) PushUserMessage(content string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messageStack = append(s.messageStack, ChatMessage{
		Role:    "user",
		Content: content,
	})
}

// PushAssistantMessage 追加模型回复，仅写入当前沙箱
func (s *AgentSandbox) PushAssistantMessage(content string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messageStack = append(s.messageStack, ChatMessage{
		Role:    "assistant",
		Content: content,
	})
}

// GetContext 获取当前沙箱完整上下文（深拷贝，防止外部篡改）
func (s *AgentSandbox) GetContext() []ChatMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make([]ChatMessage, len(s.messageStack))
	copy(res, s.messageStack)
	return res
}

// Reset 重置沙箱，仅保留系统提示词，清空所有业务对话
func (s *AgentSandbox) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messageStack = []ChatMessage{
		{Role: "system", Content: s.systemPrompt},
	}
}

// GetRole 获取沙箱角色标识
func (s *AgentSandbox) GetRole() string {
	return s.roleType
}

// GetID 获取沙箱唯一ID
func (s *AgentSandbox) GetID() string {
	return s.sandboxID
}
```

### 3.2 沙箱管理器（SandboxManager）
单例管理所有沙箱生命周期，主写作沙箱常驻，辅助沙箱按需创建/销毁。

```go
// internal/agent/sandbox/manager.go
package sandbox

import "sync"

// SandboxManager 沙箱管理器
// 统一维护所有Agent沙箱的生命周期
type SandboxManager struct {
	sandboxMap map[string]*AgentSandbox
	mu         sync.RWMutex
}

var instance *SandboxManager
var once sync.Once

// GetSandboxManager 获取单例
func GetSandboxManager() *SandboxManager {
	once.Do(func() {
		instance = &SandboxManager{
			sandboxMap: make(map[string]*AgentSandbox),
		}
	})
	return instance
}

// RegisterWriterSandbox 注册主写作Agent沙箱（常驻）
// 原有主Agent直接挂载，无侵入改造
func (m *SandboxManager) RegisterWriterSandbox(systemPrompt string) *AgentSandbox {
	m.mu.Lock()
	defer m.mu.Unlock()
	sandbox := NewAgentSandbox("writer", systemPrompt)
	m.sandboxMap["writer"] = sandbox
	return sandbox
}

// GetOrCreateAuxSandbox 获取/创建辅助Agent沙箱
// 同角色复用沙箱，支持重置复用
func (m *SandboxManager) GetOrCreateAuxSandbox(role string, systemPrompt string) *AgentSandbox {
	m.mu.RLock()
	sandbox, ok := m.sandboxMap[role]
	m.mu.RUnlock()
	if ok {
		return sandbox
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	// 双重检查
	if sandbox, ok := m.sandboxMap[role]; ok {
		return sandbox
	}
	sandbox = NewAgentSandbox(role, systemPrompt)
	m.sandboxMap[role] = sandbox
	return sandbox
}

// DestroySandbox 销毁指定辅助沙箱，释放内存
// 禁止销毁主写作沙箱
func (m *SandboxManager) DestroySandbox(role string) {
	if role == "writer" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sandboxMap, role)
}

// GetWriterSandbox 获取主写作沙箱
func (m *SandboxManager) GetWriterSandbox() *AgentSandbox {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sandboxMap["writer"]
}
```

### 3.3 任务调度器（Orchestrator）
负责任务分发与结果汇总，全程不持有业务上下文，不接触主写作沙箱的对话历史。

```go
// internal/agent/orchestrator.go
package agent

import (
	"your_project/internal/agent/sandbox"
	"your_project/internal/cache/novel"
)

// Orchestrator 任务调度器
// 仅做指令下发与结果回写缓存，不积累业务对话
type Orchestrator struct {
	sandboxMgr *sandbox.SandboxManager
	cache      *novel.Cache
}

func NewOrchestrator(cache *novel.Cache) *Orchestrator {
	return &Orchestrator{
		sandboxMgr: sandbox.GetSandboxManager(),
		cache:      cache,
	}
}

// BuildWorldview 执行世界观构建辅助任务
// 全程不触碰主写作沙箱的对话栈
func (o *Orchestrator) BuildWorldview(projectID string, params map[string]interface{}) error {
	// 1. 获取独立的设定Agent沙箱
	sandbox := o.sandboxMgr.GetOrCreateAuxSandbox(
		"world_builder",
		SystemPrompts.WorldBuilder,
	)

	// 2. 向独立沙箱下发指令
	prompt := o.buildWorldviewPrompt(params)
	sandbox.PushUserMessage(prompt)

	// 3. 调用模型生成（仅使用当前沙箱上下文）
	result, err := o.callModel(sandbox.GetContext())
	if err != nil {
		return err
	}
	sandbox.PushAssistantMessage(result)

	// 4. 结构化结果写入缓存，作为唯一数据共享通道
	structured := o.parseStructuredResult(result)
	return o.cache.Asset.Set(projectID, "worldview", structured)
}

// 其他辅助任务同理：对标拆解、大纲生成、质量检测等
```

### 3.4 缓存共享通道
三级缓存引擎作为唯一跨 Agent 数据通道，所有资产结构化读写，禁止通过对话上下文传递。
- **写入方**：辅助 Agent 生成结果后，结构化写入对应缓存分区
- **读取方**：主写作 Agent 调用模型前，缓存引擎临时从对应分区读取资产，拼接到当次请求的 System Prompt 中
- **核心约束**：缓存注入为单次临时行为，**不会写入主 Agent 的对话历史栈**

---

## 四、主写作 Agent 零侵入保障
### 4.1 存量逻辑零修改
- 主写作 Agent 的核心代码、对话历史、Skill 调用逻辑完全保留，无需改造
- 仅在模型请求的前置阶段，新增「缓存资产注入」的中间件逻辑，属于请求级拼接，不修改会话本身
- 关闭多 Agent 模式后，完全退化为原有单 Agent 逻辑，无任何副作用

### 4.2 资产注入无残留
缓存资产的注入发生在单次模型请求的拼接阶段：
1. 模型调用前：从缓存读取设定 → 拼接到 System Prompt 末尾 → 发送给模型
2. 模型调用后：主 Agent 的对话历史仅追加「用户指令+模型回复」，不保留注入的设定内容
3. 效果等价于：写作者每次动笔前临时翻一下设定手册，写完就合上，不会把设定手册抄进对话记录里

### 4.3 辅助 Agent 透明化
- 主写作 Agent 的感知范围内，不存在其他 Agent
- 辅助任务的执行过程、报错、重试全部在调度层闭环，不会向主 Agent 的对话栈输出任何中间过程
- 仅当辅助任务全部完成后，可选择向主 Agent 发送一句极简通知（如「设定已更新」），无冗余信息

---

## 五、隔离可靠性保障机制
### 5.1 角色身份锚定
每个沙箱的系统 Prompt 首行强制加入角色锁死指令，配合沙箱隔离双重保障，防止角色串味：
```
【角色锁定】你是专业的网文世界观设定师，仅负责输出结构化世界观设定。
绝对禁止参与正文写作、剧情推演、人设修改等非设定类工作。
所有输出必须严格遵循YAML格式，无多余解释性话术。
```

### 5.2 上下文独立裁剪
每个沙箱独立维护上下文窗口与裁剪策略：
- 主写作沙箱：保留最近 N 章创作对话+剧情摘要，保障创作连贯性
- 质检沙箱：每次仅传入单章内容+设定资产，用完即重置，不积累历史对话
- 设定沙箱：设定完成后即可重置释放，避免上下文膨胀

### 5.3 故障单点隔离
每个辅助沙箱的调用独立做错误处理：
- 单个辅助 Agent 执行失败 → 独立重试/降级，不影响主写作链路
- 沙箱上下文污染 → 直接销毁重建，不波及其他沙箱
- 调度器异常 → 自动降级为单 Agent 模式，主写作功能完全可用

---

## 六、典型工作流时序（新书立项示例）
1. 用户触发「一键新书立项」指令，指令仅发送到调度器
2. 调度器创建「策划 Agent 沙箱」，调用对标拆解 Skill，生成结构化赛道报告 → 写入缓存
3. 调度器创建「设定 Agent 沙箱」，从缓存读取对标报告，生成世界观/人设/金手指 → 写入全局资产缓存
4. 调度器创建「大纲 Agent 沙箱」，从缓存读取设定资产，生成全书粗纲+首卷细纲 → 写入缓存
5. 全部任务完成后，调度器仅向主写作沙箱发送一句通知：「首卷筹备完成，可开始正文创作」
6. 用户向主写作 Agent 发送「写第一章」指令，主 Agent 生成前，缓存引擎自动从缓存读取设定，临时注入请求
7. 主 Agent 对话历史仅保留「写第一章 + 第一章正文」，无任何辅助 Agent 的执行痕迹

---

## 七、文件结构变更
```
your_project/
├── internal/
│   ├── agent/
│   │   ├── agent.go              # 原有主写作Agent，无修改
│   │   ├── orchestrator.go       # 新增：任务调度器
│   │   └── sandbox/
│   │       ├── sandbox.go        # 新增：会话沙箱核心实现
│   │       ├── manager.go        # 新增：沙箱生命周期管理
│   │       └── prompts.go        # 新增：各角色专属系统提示词
│   ├── cache/
│   │   └── novel/                # 原有三级缓存，全量复用
│   └── config/
│       └── config.go             # 新增：多Agent开关配置
└── novel-agent.example.toml      # 新增：多Agent配置项
```

---

## 八、配置开关
支持通过配置项一键启停多 Agent 隔离模式，兼容单 Agent 使用场景：

```toml
# novel-agent.example.toml
[multi_agent]
# 是否启用多Agent模式，关闭后退化为单Agent
enabled = true
# 辅助任务完成后自动重置沙箱，释放内存
auto_reset_sandbox = true
# 启用缓存资产自动注入
cache_inject_enabled = true
```

对应 Go 配置结构体：
```go
// internal/config/config.go
type MultiAgentConfig struct {
	Enabled           bool `toml:"enabled"`
	AutoResetSandbox  bool `toml:"auto_reset_sandbox"`
	CacheInjectEnabled bool `toml:"cache_inject_enabled"`
}
```