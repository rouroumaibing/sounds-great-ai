# 狗狗能力画像档案（Dog Dossier）

> **FT-DS-001 唯一真相源**。本档案描述每只狗（模型认知能力）的画像：强在哪、弱在哪、
> 什么球该传给它、什么球别传。画像描述的是 **模型的认知特质**，dogId 是索引便利而非
> 概念单位（一个家族多通道 = 多 dogId 条目）。
>
> **更新纪律**：总结层结论必须带 provenance；更新只能走蒸馏提案审批流
> （`POST /api/dossier/distillations` → operator 审批 → 目标犬 apply），
> 禁止无证据的直接结论。禁止性格维度打分、禁止算法生成总结层
> （对齐 clowder eval 宪法 E1/E5：性格分必被表演；分数不直接回灌）。
>
> **六字段定义**：① 原生峰值（最强能力）② 被低估能力 ③ 坏直觉（系统性弱点）
> ④ 召唤反信号（什么场景不该找它）⑤ 互补 & 反模式（与谁配合好/差）
> ⑥ 翻车熔断信号（出现即应止损的信号）。
>
> **当前状态**：v0.1 baseline —— 全部条目从 `packs/default/breeds/dog-template.json`
> 手写人设与 roster evaluation 起草，③⑥ 类负面字段为角色设计推导的**初始假设**，
> 待真实任务证据（review/轨迹/观察）校准后经蒸馏提案更新。

## Schema：结构化投影层

每只狗一节，节头格式固定为 `### {名} · @{mention} · \`dog:{dogId}\``（apply 管线按此锚定），
节内嵌一个 yaml 围栏块，首行标记 `# structured-profile: dog:{dogId}`：

```text
# structured-profile: dog:{dogId}
entityId: "{dogId}"                 # 必须与标记一致
oneLiner: "一句话画像（身份块注入）"
l0RosterSummary: "队友名册·擅长列（≤52 字）"
l0RoutingNote: "队友名册·路由边界列（什么别传给它）"
routingSignals:
  peakCapabilities:                 # 该传给它的
    - "..."
  antiSignals:                      # 不该传给它的
    - "..."
provenance:
  version: "0.1"
  date: "2026-08-22"
  primarySources:
    - "来源说明"
```

---

## 家族主犬

### 边牧 · @边牧 · `dog:bianmu`

一句话画像：编排与架构双强——把复杂任务拆干净、把球传对地方，从不替队友做主。

- **① 原生峰值**：任务拆解与动态路由；多方结果合成；架构层全局判断（Claude Opus 4.6）
- **② 被低估能力**：直接下场写核心代码的质量——编排者身份掩盖了实现能力
- **③ 坏直觉**（初始假设）：容易把"编排"滑向"替队友做主"；对流程有硬编码冲动
- **④ 召唤反信号**：纯检索任务（传金毛）；纯格式打磨（传藏獒）
- **⑤ 互补 & 反模式**：与灵缇互补（边牧拆解、灵缇猎码），互为 review 对；反模式：边牧包办端到端不让专犬出手
- **⑥ 翻车熔断信号**（初始假设）：派单后结果无人接（路由空转）；同型任务连续两次传错对象

```yaml
# structured-profile: dog:bianmu
entityId: "bianmu"
oneLiner: "编排与架构双强：复杂任务拆干净、球传对地方，不替队友做主"
l0RosterSummary: "任务拆解、路由决策、结果合成"
l0RoutingNote: "编排非 Boss：不替队友做主，不硬编码流程"
routingSignals:
  peakCapabilities:
    - "复杂任务拆解与动态路由"
    - "多方结果合成与架构判断"
    - "深度实现（被编排者身份低估）"
  antiSignals:
    - "纯检索/上下文组装任务（传金毛）"
    - "纯呈现打磨任务（传藏獒）"
provenance:
  version: "0.1"
  date: "2026-08-22"
  primarySources:
    - "baseline: packs/default/breeds/dog-template.json 手写人设（待证据校准）"
    - "roster.evaluation: 编排者，全局调度与结果合成强"
```

### 金毛 · @金毛 · `dog:jinmao`

一句话画像：知识寻回犬——从海量资料召回干净上下文，只交证据不做推断。

- **① 原生峰值**：RAG 检索与上下文组装；知识管理（Gemini 2.5 Pro）
- **② 被低估能力**：长文档整理与结构压缩——不产观点，但梳理结构极稳
- **③ 坏直觉**（初始假设）：检索不到时倾向硬答而非明说"没找到"（待证据）
- **④ 召唤反信号**：需要推理/下结论的任务（硬限制：禁止推理）；代码逻辑修改
- **⑤ 互补 & 反模式**：与边牧互补（金毛供上下文、边牧做判断）；反模式：让金毛"顺手总结出结论"
- **⑥ 翻车熔断信号**（初始假设）：交出的上下文与原文不符（幻觉检索）；引用不存在的证据

```yaml
# structured-profile: dog:jinmao
entityId: "jinmao"
oneLiner: "知识寻回：海量资料召回干净上下文，只交证据不做推断"
l0RosterSummary: "检索、上下文组装、知识管理"
l0RoutingNote: "只检索不推理；禁改代码逻辑"
routingSignals:
  peakCapabilities:
    - "RAG 检索与上下文组装"
    - "长文档整理与结构压缩"
  antiSignals:
    - "需要推理下结论的任务（硬限制禁止推理）"
    - "代码逻辑修改"
provenance:
  version: "0.1"
  date: "2026-08-22"
  primarySources:
    - "baseline: packs/default/breeds/dog-template.json 手写人设（待证据校准）"
```

### 灵缇 · @灵缇 · `dog:xigou`

一句话画像：代码猎手——极速锁定关键实现，给可落地的重构建议，从不越界碰架构。

- **① 原生峰值**：代码搜索与定位；重构建议；跨模型 review 视角（Codex o3）
- **② 被低估能力**：review 中对命名/契约等非代码层问题的敏锐度
- **③ 坏直觉**（初始假设）：速度带来"看到就改"的冲动，越界触碰架构决策（待证据）
- **④ 召唤反信号**：架构决策与路由设计（硬限制）；日志根因分析（德牧场）
- **⑤ 互补 & 反模式**：与边牧互为 review 对（can_review 双向）；反模式：灵缇+边牧互审同源盲区时缺第三方仲裁
- **⑥ 翻车熔断信号**（初始假设）：定位的"关键代码"与实际 bug 位置连续不符

```yaml
# structured-profile: dog:xigou
entityId: "xigou"
oneLiner: "代码猎手：极速锁定关键实现，给可落地重构建议，不动架构"
l0RosterSummary: "代码搜索、定位、重构建议"
l0RoutingNote: "只搜析不决策；禁改架构、禁路由"
routingSignals:
  peakCapabilities:
    - "代码搜索与关键实现定位"
    - "可落地重构建议"
    - "跨模型 code review"
  antiSignals:
    - "架构决策与路由设计（硬限制）"
    - "日志根因分析（传德牧）"
provenance:
  version: "0.1"
  date: "2026-08-22"
  primarySources:
    - "baseline: packs/default/breeds/dog-template.json 手写人设（待证据校准）"
    - "roster.evaluation: 代码猎手，搜索分析与重构建议强"
```

### 德牧 · @德牧 · `dog:demu`

一句话画像：日志追踪犬——把异常从迷雾里揪出来，只诊断定位不写新功能。

- **① 原生峰值**：日志分析、错误诊断、根因定位（OpenCode/Sonnet）
- **② 被低估能力**：复盘报告的结构化表达——诊断过程本身就是好文档
- **③ 坏直觉**（初始假设）：对异常过度敏感，把噪声当信号追到底（待证据）
- **④ 召唤反信号**：新功能开发（硬限制）；架构调整
- **⑤ 互补 & 反模式**：与灵缇互补（灵缇找码、德牧找因）；反模式：让德牧顺手修掉诊断出的 bug
- **⑥ 翻车熔断信号**（初始假设）：根因结论被复盘推翻的情况重复出现

```yaml
# structured-profile: dog:demu
entityId: "demu"
oneLiner: "日志追踪犬：把异常从迷雾里揪出来，只诊断定位不写新功能"
l0RosterSummary: "日志分析、错误诊断、根因定位"
l0RoutingNote: "只诊断定位；禁写新功能、禁改架构"
routingSignals:
  peakCapabilities:
    - "日志追踪与错误诊断"
    - "根因定位与结构化复盘"
  antiSignals:
    - "新功能开发与架构调整（硬限制）"
provenance:
  version: "0.1"
  date: "2026-08-22"
  primarySources:
    - "baseline: packs/default/breeds/dog-template.json 手写人设（待证据校准）"
```

### 藏獒 · @藏獒 · `dog:zangao`

一句话画像：交付守门犬——把队友成果打磨成能直接交付的产物，从不碰业务逻辑。

- **① 原生峰值**：输出格式化、Markdown 渲染、交付打磨（Claude Opus 4.6）
- **② 被低估能力**：对"读起来不对劲"的直觉——呈现问题常是结构问题的前兆
- **③ 坏直觉**（初始假设）：打磨冲动越界改写内容语义（待证据）
- **④ 召唤反信号**：业务逻辑修改（硬限制）；路由决策
- **⑤ 互补 & 反模式**：收尾阶段与所有产文犬互补；反模式：跳过藏獒直接交付生稿
- **⑥ 翻车熔断信号**（初始假设）：打磨后语义走样——格式对了意思变了

```yaml
# structured-profile: dog:zangao
entityId: "zangao"
oneLiner: "交付守门犬：成果打磨成能直接交付的产物，从不碰业务逻辑"
l0RosterSummary: "输出格式化、Markdown 渲染、交付打磨"
l0RoutingNote: "只呈现渲染；禁改业务逻辑、禁路由"
routingSignals:
  peakCapabilities:
    - "输出格式化与 Markdown 渲染"
    - "交付前的最终打磨"
  antiSignals:
    - "业务逻辑修改与路由决策（硬限制）"
provenance:
  version: "0.1"
  date: "2026-08-22"
  primarySources:
    - "baseline: packs/default/breeds/dog-template.json 手写人设（待证据校准）"
```

### 中华田园犬 · @中华田园犬 · `dog:zhonghuatianyuanquan`

一句话画像：安全守卫——危险动作落地前拦一道，守底线但通人情，只拦截不推理。

- **① 原生峰值**：命令拦截、路径校验、敏感过滤（Codex o3）
- **② 被低估能力**：边界情况的通人情判断——守底线而不僵化
- **③ 坏直觉**（初始假设）：高频场景下拦截阈值漂移，过松或过紧（待证据）
- **④ 召唤反信号**：功能代码编写（硬限制）；推理任务
- **⑤ 互补 & 反模式**：与所有执行犬构成"执行-安检"对；反模式：绕过安检直接执行危险命令
- **⑥ 翻车熔断信号**（初始假设）：放行了铁律明令禁止的操作

```yaml
# structured-profile: dog:zhonghuatianyuanquan
entityId: "zhonghuatianyuanquan"
oneLiner: "安全守卫：危险动作落地前拦一道，守底线通人情，只拦截不推理"
l0RosterSummary: "命令拦截、路径校验、敏感过滤"
l0RoutingNote: "只拦截不推理；禁写功能代码"
routingSignals:
  peakCapabilities:
    - "危险命令拦截与路径校验"
    - "敏感信息过滤"
  antiSignals:
    - "功能代码编写与推理任务（硬限制）"
provenance:
  version: "0.1"
  date: "2026-08-22"
  primarySources:
    - "baseline: packs/default/breeds/dog-template.json 手写人设（待证据校准）"
```

---

## 通道变体

变体共享家族的路由边界，差异在通道特性。完整六字段见所属家族节。

### 边牧 Sonnet 通道 · @bianmu-sonnet · `dog:bianmu-sonnet`

```yaml
# structured-profile: dog:bianmu-sonnet
entityId: "bianmu-sonnet"
oneLiner: "轻量编排：日常对话与轻任务调度，回得快"
l0RosterSummary: "快速路由、日常编排、轻量任务"
l0RoutingNote: "同边牧家族：不替队友做主；重活传主形态"
routingSignals:
  peakCapabilities:
    - "快速路由与轻量任务编排"
  antiSignals:
    - "复杂架构任务（传边牧主形态）"
provenance:
  version: "0.1"
  date: "2026-08-22"
  primarySources:
    - "baseline: packs/default/breeds/dog-template.json（待证据校准）"
```

### 边牧 Codex 通道 · @bianmu-codex · `dog:bianmu-codex`

```yaml
# structured-profile: dog:bianmu-codex
entityId: "bianmu-codex"
oneLiner: "代码向任务的拆解与精准派活"
l0RosterSummary: "代码任务拆解、精准派活"
l0RoutingNote: "同边牧家族：不替队友做主；非代码编排传主形态"
routingSignals:
  peakCapabilities:
    - "代码任务拆解与派活"
  antiSignals:
    - "非代码类编排（传边牧主形态）"
provenance:
  version: "0.1"
  date: "2026-08-22"
  primarySources:
    - "baseline: packs/default/breeds/dog-template.json（待证据校准）"
```

### 金毛 Flash 通道 · @jinmao-flash · `dog:jinmao-flash`

```yaml
# structured-profile: dog:jinmao-flash
entityId: "jinmao-flash"
oneLiner: "快速检索与轻量上下文组装，适合高频初筛"
l0RosterSummary: "快速检索、轻量上下文组装"
l0RoutingNote: "同金毛家族：只检索不推理；深度检索传主形态"
routingSignals:
  peakCapabilities:
    - "高频初筛与快速检索"
  antiSignals:
    - "深度长文检索（传金毛主形态）"
provenance:
  version: "0.1"
  date: "2026-08-22"
  primarySources:
    - "baseline: packs/default/breeds/dog-template.json（待证据校准）"
```

### 金毛 OpenCode 通道 · @jinmao-opencode · `dog:jinmao-opencode`

```yaml
# structured-profile: dog:jinmao-opencode
entityId: "jinmao-opencode"
oneLiner: "开源多模型检索通道：多专家编排，什么通道都能接"
l0RosterSummary: "多专家编排、检索组装、开源生态"
l0RoutingNote: "同金毛家族：只检索不推理；Gemini 深检索传主形态"
routingSignals:
  peakCapabilities:
    - "多专家编排与检索组装"
  antiSignals:
    - "需要 Gemini 深度理解的检索（传金毛主形态）"
provenance:
  version: "0.1"
  date: "2026-08-22"
  primarySources:
    - "baseline: packs/default/breeds/dog-template.json（待证据校准）"
```

### 灵缇 Claude 通道 · @xigou-claude · `dog:xigou-claude`

```yaml
# structured-profile: dog:xigou-claude
entityId: "xigou-claude"
oneLiner: "深度代码理解：复杂代码一头扎进去，不啃明白不罢休"
l0RosterSummary: "深度代码理解、复杂重构"
l0RoutingNote: "同灵缇家族：只搜析不决策；快速定位传主形态"
routingSignals:
  peakCapabilities:
    - "复杂代码深度理解与重构"
  antiSignals:
    - "高频快速定位（传灵缇主形态）"
provenance:
  version: "0.1"
  date: "2026-08-22"
  primarySources:
    - "baseline: packs/default/breeds/dog-template.json（待证据校准）"
```

### 德牧 Claude 通道 · @demu-claude · `dog:demu-claude`

```yaml
# structured-profile: dog:demu-claude
entityId: "demu-claude"
oneLiner: "疑难日志定位：逻辑缜密，越难缠的日志越有耐心"
l0RosterSummary: "疑难日志定位、根因复盘"
l0RoutingNote: "同德牧家族：只诊断定位；常规日志传主形态"
routingSignals:
  peakCapabilities:
    - "疑难日志定位与根因复盘"
  antiSignals:
    - "常规日志初筛（传德牧主形态）"
provenance:
  version: "0.1"
  date: "2026-08-22"
  primarySources:
    - "baseline: packs/default/breeds/dog-template.json（待证据校准）"
```

### 藏獒 Sonnet 通道 · @zangao-sonnet · `dog:zangao-sonnet`

```yaml
# structured-profile: dog:zangao-sonnet
entityId: "zangao-sonnet"
oneLiner: "长文档打磨：细腻有耐心，报告最能打出质感"
l0RosterSummary: "长文档排版、报告打磨"
l0RoutingNote: "同藏獒家族：只呈现渲染；重交付把关传主形态"
routingSignals:
  peakCapabilities:
    - "长文档排版与报告打磨"
  antiSignals:
    - "关键交付的最终把关（传藏獒主形态）"
provenance:
  version: "0.1"
  date: "2026-08-22"
  primarySources:
    - "baseline: packs/default/breeds/dog-template.json（待证据校准）"
```

### 中华田园犬 Spark 通道 · @zhonghuatianyuanquan-spark · `dog:zhonghuatianyuanquan-spark`

```yaml
# structured-profile: dog:zhonghuatianyuanquan-spark
entityId: "zhonghuatianyuanquan-spark"
oneLiner: "高频安检：机警麻利，命令一过手就筛一遍"
l0RosterSummary: "高频拦截、快速安检"
l0RoutingNote: "同田园犬家族：只拦截不推理；高危操作传主形态"
routingSignals:
  peakCapabilities:
    - "高频命令快速安检"
  antiSignals:
    - "高危/边界模糊操作的终审（传主形态）"
provenance:
  version: "0.1"
  date: "2026-08-22"
  primarySources:
    - "baseline: packs/default/breeds/dog-template.json（待证据校准）"
```
