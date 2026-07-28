<br />

### 项目名称绝妙的双关隐喻（Theme Metaphor）

- **字面含义**：`Sounds Great!`（这听起来太棒了！）—— 每次 Agent 完美完成协同、通过 Quality Gates 时的终端反馈。
- **犬系隐喻**：**Barking / Hearing**（听觉与吠声）。狗狗拥有极其敏锐的听觉（感知事件/Event-Driven）和独特的吠声（Agent 间通信/Structured Messaging）。
  - Slogan 可以叫：*"When AI Agents Bark Together, It Sounds Great."*

## 架构角色完整映射表（Go / Eino 实现指南）

| 模块 / 角色 | 犬种映射 | 形象与性格特征 | 核心职责 (Responsibilities) | Eino / Go 架构落地方案 |
|---|---|---|---|---|
| Orchestrator | 边牧 *(bianmu)* | 极高智商、控场大师、眼神敏锐 | 任务拆解、DAG 工作流调度、状态机控制 | 基于 Eino Graph，作为主控 Task Coordinator |
| Safety Guardrail | 中华田园犬 *(zhonghuatianyuanquan)* | 忠诚可靠、警惕性高、熟悉家园环境 | 看家护院：Hard Rails 安全边界、命令黑名单、权限代码审计 | 拦截器（Interceptor）与沙盒（Sandbox）隔离校验，绝对忠诚 guard |
| UI / CLI Presentation | 藏獒 *(zangao)* | 体型雄浑、威严沉稳、一夫当关 | 全局看守与终端交互：TUI 状态框渲染、日志大盘展示、关键操作的人类确认 | 结合 TUI（如 Bubbletea）界面展示，提供沉稳、权威的交互与状态汇总 |
| Code Hunter | 细狗 *(xigou)* | 身形流线、极速迅猛、目标明确 | 精准打猎：自动化 Refactor 方案设计、高难度 Bug 修复代码生成 | 专注于高难度代码优化、安全漏洞自动化"猎捕"修复 |
| RAG / Retriever | 金毛 *(jinmao)* | 寻回本能强、温和靠谱 | 向量检索、上下文叼取（Fetch）、文档关联 | 集成 Vector DB (Milvus/Qdrant)，负责 Context Engine |
| Log & Bug Tracer | 德牧 *(demu)* | 警觉敏锐、黑背立耳、执行力强 | 严密追查 Panic、StackTrace 分析、Log 溯源 | 收集 Agent 执行日志，追查错误现场并定位根因（Root Cause） |
