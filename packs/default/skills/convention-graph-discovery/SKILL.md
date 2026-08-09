---
name: convention-graph-discovery
description: >
  约定图发现方法论：进一个 repo 先识别 repo-specific conventions，再定义 domain/extractor。
  Use when: 进入陌生 repo、要画约定图、要找"改 X 影响谁"。
  Not for: 普通符号跳转/LSP、文档索引检索。
  Output: domain 定义 + extractor 计划 + gap/freshness 报告。
triggers:
  - "约定图"
  - "convention graph"
  - "代码图"
---

# Convention Graph Discovery（约定图发现）

## 流程

### 1. 识别约定
- 命名约定（文件/目录/函数）
- 结构约定（层/包/模块划分）
- 行为约定（错误处理/日志/配置）

### 2. 定义 Domain
- 节点类型（file/function/type/module）
- 边类型（calls/imports/depends/implements）

### 3. Extractor 计划
- 静态分析提取节点和边
- 增量更新策略
- 验证提取正确性

### 4. 报告
- gap：哪些约定没被覆盖
- freshness：图是否与代码同步
- provenance：每个节点的来源
