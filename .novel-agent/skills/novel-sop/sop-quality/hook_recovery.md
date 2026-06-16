---
name: sop-hook-recovery
description: 伏笔回收校验 — 扫描已埋设伏笔，标记待回收节点和逾期伏笔
runAs: inline
---

# 伏笔回收校验 Skill

## 角色定位
扫描 `.novel-agent/hooks/ledger.yaml` 中的伏笔台账，结合大纲和当前章节进度，检查伏笔回收健康度。

## 前置条件
- `read_file .novel-agent/state.json`
- `read_file .novel-agent/hooks/ledger.yaml`
- `read_file outlines/main_outline.txt`

## 操作步骤

### 1. 读取台账和大纲
```
read_file .novel-agent/hooks/ledger.yaml
read_file outlines/main_outline.txt
```

### 2. 调用内置伏笔分析
将伏笔台账内容作为 `source` 传入：
```
novel_consult(
  subject="伏笔回收校验",
  source="<hooks_ledger_content>"
)
```

### 3. 结合大纲检查逾期伏笔
对于标记了 `expected_recovery` 的伏笔，对比当前进度：
- 已过预定回收章仍未回收 → 标记「逾期」
- 预定回收章在前 20 章范围内 → 标记「即将到期」
- 预计回收章在后 20 章之外 → 标记「尚早」

### 4. 输出校验报告

```
═══ 伏笔回收校验报告 ═══
当前进度：第{N}章

总伏笔数：{N}
已回收：{N}（{X}%）
待回收：{N}
  ├─ 逾期：{N}条 ⚠️
  ├─ 即将到期：{N}条
  └─ 尚早：{N}条

逾期伏笔清单：
1. {伏笔名}（预定第{X}章）→ 已超{N}章
   💡 建议：在下{N}章内安排回收

即将到期伏笔：
...

建议新增伏笔：
- 连续 {N} 章未埋伏笔，建议近期新增 1-2 条伏笔
'''
