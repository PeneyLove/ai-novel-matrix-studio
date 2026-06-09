---
name: novel-volume-plan
description: 分卷规划 — 将大纲按卷拆分细化，规划每卷的起承转合、爽点分布、伏笔回收时间表
runAs: inline
---

# 分卷规划

## 角色定位
将已定稿大纲细化为可执行的分卷规划，每卷有独立的起承转合和爽点节奏。

## 操作步骤
1. `read_file outlines/main_outline.txt` 获取大纲
2. `read_file .novel-agent/hooks/ledger.yaml` 获取伏笔台账
3. 逐卷细化 → 用户确认 → `write_file outlines/volume_plan.yaml`

## 分卷规划模板

```yaml
novel_title: 小说名
total_volumes: 8
current_volume: 1

volumes:
  - id: 1
    title: 卷名
    chapters: "1-40"
    status: writing  # planned/writing/completed
    # 起承转合
    opening: 开篇事件（前3章完成）
    development: 发展（第4-30章的主要事件链）
    twist: 转折（第31-35章的反转/冲突升级）
    climax: 高潮（第36-40章的爆发）
    # 爽点布局（必须≥3个核心爽点标注章号）
    high_points:
      - chapter: 10
        type: 首次越级杀敌
        expected_impact: 高
      - chapter: 25
        type: 获得上古传承
      - chapter: 40
        type: 突破金丹+扬名
    # 本卷回收伏笔
    resolve_hooks: [hook-001, hook-003, hook-005]
    # 本卷新埋伏笔
    plant_hooks: [hook-011, hook-012, hook-013]
    # 卷末钩子（引出下卷）
    volume_hook: 卷末悬念描述
```

## 各卷衔接检查
- 每卷结尾钩子必须自然引导下卷
- 卷间时间跳跃有明确说明
- 力量体系升级在卷末完成（不做半截升级）
- 每卷结尾主角状态必须与下卷开篇一致
