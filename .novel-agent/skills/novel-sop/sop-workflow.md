---
name: sop-workflow
description: SOP 全流程导航 — 按网文创作标准化流程，指导用户从立项到完本的每一步，自动编排 Skill 调用
runAs: inline
---

# SOP 全流程导航 Skill

## 角色定位
网文创作的标准化流程（SOP）导航器。根据用户当前所处的阶段，推荐下一步操作和对应的 Skill。

## SOP 七阶段总览
```
前期筹备 → 核心设定 → 大纲搭建 → 开篇打磨 → 正文创作 → 质量校验 → 发布运营
```

## 各阶段关联 Skill

### 1. 前期筹备
- `/sop-benchmark-analysis` — 对标作品拆解（如需做赛道分析）
- 目标：确定题材、卖点、核心创意

### 2. 核心设定
- `/novel-{genre}-genre_init` — 类型初始化（现有 Genre Skill）
- `/novel-worldbuilding` — 世界观构建
- `/novel-characters` — 人设卡创建
- 所有设定自动注入写作资产缓存，后续生成无需重复输入

### 3. 大纲搭建
- `/novel-{genre}-outline` — 分卷大纲生成（现有 Genre Skill）
- 大纲定稿后自动写入 `outlines/main_outline.txt`

### 4. 开篇打磨
- `/novel-{genre}-writing` — 首章生成（现有 Genre Skill）
- `/novel-consult outline` — 大纲完整性审核
- 黄金三章特别重要，建议反复打磨

### 5. 正文创作
- `/novel-continue` — 日常续写（现有 Skill）
- `/sop-plot-divergence` — 卡文推演（遇到剧情瓶颈时调用）

### 6. 质量校验
- `/novel-consult full` — 完整创作咨询（大纲+人设+伏笔+风格）
- `/sop-consistency-check` — 人设一致性校验
- `/sop-hook-recovery` — 伏笔回收校验
- 建议每 10 章做一次完整咨询

### 7. 发布运营
- 书名/简介生成（规划中）
- 完本复盘（规划中）

## 使用方式

### 查询当前阶段
`read_file .novel-agent/state.json`

### 获取引导
直接输入 `/sop-workflow` 查看本页导航。
