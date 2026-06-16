---
name: sop-consistency-check
description: 人设一致性校验 — 调用内置分析引擎，逐章比对角色设定，标记OOC行为并给出修复建议
runAs: inline
---

# 人设一致性校验 Skill

## 角色定位
使用内置确定性分析引擎检查角色行为是否与人设卡一致，替代单纯依赖模型感觉的OOC判断。

## 前置条件
需要存在以下文件：
- `.novelAgent/state.json` — 项目状态
- `characters/` 目录下的人设 YAML 文件

## 操作步骤

### 1. 读取人设文件
```
ls characters/
```
逐一读取每个人设文件：
```
read_file characters/{角色名}.yaml
```

### 2. 读取最近章节内容
```
ls chapters/
```
读取最近 5-10 章中该角色出现的章节。

### 3. 调用内置分析
将读取到的人设内容和章节内容，通过 `novel_consult` 工具传入内置分析引擎。

调用示例：
```
novel_consult(
  subject="人设一致性校验 - {角色名}",
  source="角色设定内容\n\n章节内容"
)
```

### 4. 输出校验报告
按 `novel_consult` 的输出格式呈现。

## 检查维度
- **行为一致**：角色行为是否符合 personality.traits 设定
- **语言一致**：角色的说话方式/口头禅是否稳定
- **能力一致**：战斗力/能力是否按 arc 弧线合理增长
- **关系一致**：与其他角色的关系动态是否符合设定
- **弧光一致**：角色的成长是否沿着 arc.growth_trajectory 方向
