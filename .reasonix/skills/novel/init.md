---
name: novel-init
description: 初始化小说项目目录结构 — 创建 .novelAgent/ 元数据目录、章节目录、大纲/人设/伏笔台账模板
runAs: inline
---

# 小说项目初始化 Skill

你负责为新小说项目创建标准目录结构。

## 标准小说项目结构

```
我的小说/
├── .novelAgent/                  # 元数据（AI 状态 + 版本历史）
│   ├── state.json                # 创作状态（当前类型/阶段/进度）
│   ├── config.yaml               # 项目配置（模型绑定/全局规则）
│   ├── outline/                  # 大纲版本
│   │   └── v1.md
│   ├── characters/               # 人物谱系
│   │   └── characters.yaml
│   ├── hooks/                    # 伏笔台账
│   │   └── ledger.yaml
│   └── prompts_history/          # 提示词版本快照
├── outlines/                     # 人类可读的大纲文件
│   └── main_outline.txt
├── characters/                   # 人类可读的人物设定
│   ├── protagonist.txt
│   └── supporting_cast.txt
├── chapters/                     # 章节目录
│   ├── 第1章/
│   │   └── chapter.txt
│   ├── 第2章/
│   │   └── chapter.txt
│   └── ...
└── README.md                     # 小说简介
```

## 执行步骤

1. 用 `bash mkdir` 创建以上目录结构
2. 用 `write_file` 创建初始模板文件：
   - `.novelAgent/state.json`: `{"genre":"","phase":"init","chapter":0,"total_chapters":0}`
   - `.novelAgent/config.yaml`: 基本配置
   - `README.md`: 小说简介模板
3. 告知用户项目已初始化，可以开始创作

## 文件格式

所有创作文件使用纯文本 `.txt` 格式，UTF-8 编码。元数据文件使用 JSON/YAML 格式。
章节内容文件 `chapter.txt` 使用 Markdown 格式以便排版和阅读。

## 注意

- 所有文件使用 UTF-8 编码
- 章节按数字编号，使用中文 "第N章" 命名
- `.novelAgent/` 下是机器可读的结构化数据
- `chapters/` `outlines/` `characters/` 下是人类可读的文本
