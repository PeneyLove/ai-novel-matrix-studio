---
name: novel-characters
description: 人物谱系管理 — 创建/查看/修改角色设定，检查人物一致性
runAs: inline
---

# 人物谱系管理

## 角色定位
管理小说所有角色的设定文件，确保人物行为一致、有辨识度、有成长弧光。

## 操作步骤
1. `read_file characters/` 目录查看现有人物
2. 根据用户指令创建/修改/删除角色
3. 每个角色存入 `characters/{角色名}.yaml`
4. 同步更新主角汇总文件 `characters/protagonist.txt`

## 角色设定模板（每个角色一个 YAML 文件）

```yaml
name: 角色全名
alias: [别名/称号]
role: 主角 / 主要配角 / 次要配角 / 反派 / 路人
genre_binding: xuanhuan

# 基础信息
basic:
  age: 0
  gender: 男/女
  appearance: 外貌特征（≥3个辨识点）
  background: 身世背景（500字内）

# 性格
personality:
  traits: [性格标签1, 标签2, 标签3]  # 至少3个
  speech_style: 说话方式/口头禅
  habits: [标志性动作/习惯]
  fears: 内心恐惧
  desires: 核心欲望

# 能力（玄幻/科幻）
abilities:
  - name: 能力名
    level: 当前等级
    description: 能力描述
    limitations: 限制/代价

# 关系网
relationships:
  - target: 角色名
    type: 盟友/敌人/暧昧/师徒/亲属
    dynamic: 关系动态描述

# 弧光
arc:
  current_phase: 当前阶段
  growth_trajectory: 成长方向
  key_moments: [关键转折点列表]
```

## 诊断检查项
- **辨识度**：每个角色≥3个独有特征（外貌/口头禅/习惯/思维方式）
- **功能定位**：每个配角有独立的剧情功能而非纯工具人
- **关系网**：每个角色关联≥2个其他角色
- **弧光**：主要角色10章内需要有可感知的变化
- **OOC检查**：最近行为是否符合 personality 设定
