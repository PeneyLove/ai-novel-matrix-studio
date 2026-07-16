---
name: novel-continue
description: 续写小说下一章 — 自动读取当前进度、大纲、伏笔台账，写入新章节文件到 chapter/第X卷/第N章.txt
runAs: inline
---

# 续写小说章节 Skill

你负责在已有项目基础上续写下一章。

## 前置条件

确认以下文件存在：
- `.novelAgent/state.json` — 包含当前章节号 (`chapter`)、当前卷号 (`volume`) 和总章节数 (`total_chapters`)
- `outlines/main_outline.txt` — 当前大纲
- `.novelAgent/hooks/ledger.yaml` — 伏笔台账
- 上一章内容 (如果有的话): `chapter/第X卷/第{chapter-1}章.txt`

## 执行步骤

1. **读取状态**：用 `read_file` 读取 `.novelAgent/state.json` 获取当前进度（卷号+章节号）
2. **读取大纲**：用 `read_file` 读取 `outlines/main_outline.txt`
3. **读取伏笔台账**：用 `read_file` 读取 `.novelAgent/hooks/ledger.yaml`
4. **读取上一章**：用 `read_file` 读取上一章内容（如有）
5. **确保卷目录存在**：用 `bash mkdir -p` 创建 `chapter/第X卷/`（如不存在）
6. **写入新章节**：用 `write_file` 写入 `chapter/第X卷/第N章.txt`
   - 每章必须包含：1个微爽点 + 1个收尾钩子 + 1处伏笔铺垫
   - 使用 Markdown 格式，UTF-8 编码
7. **更新状态**：用 `write_file` 更新 `.novelAgent/state.json`（chapter号 +1；如进入新卷则 volume+1）
8. **更新伏笔台账**：如有新埋伏笔或回收伏笔，更新 ledger.yaml

## 续写核心规则

1. 严格绑定已定稿大纲，100%贴合人设和世界观
2. 每章自动植入：1个微爽点 + 1个收尾钩子 + 1处伏笔铺垫
3. 保持网文节奏：章章有勾、节节有料
4. 上下文连贯：读取上一章最后300字作为衔接
5. 禁止AI套话，对话简洁，行动多于内心独白

## 章节文件格式

```markdown
# 第N章 章节标题

[正文内容...]

---
*本章爽点：[简述]*
*埋伏笔：[简述，ID]*
*收尾钩子：[简述]*
```
