
### Theme Metaphor

- **Literal meaning**: `Sounds Great!` — the terminal feedback every time an Agent perfectly completes collaboration and passes Quality Gates.
- **Canine metaphor**: **Barking / Hearing**. Dogs possess extraordinarily keen hearing (perceiving events / Event-Driven) and distinctive barks (inter-Agent communication / Structured Messaging).
  - Slogan: *"When AI Agents Bark Together, It Sounds Great."*

## Architecture Role Mapping (Go / Eino Implementation Guide)

| Module / Role | Breed Mapping | Personality & Traits | Core Responsibilities | Eino / Go Architecture |
|---|---|---|---|---|
| Orchestrator | Border Collie *(bianmu)* | Extremely intelligent, field-control master, sharp-eyed | Task decomposition, dynamic routing, result synthesis | Based on Eino Graph, serves as main Task Coordinator |
| Safety Guardrail | Chinese Rural Dog *(zhonghuatianyuanquan)* | Loyal, reliable, highly alert, familiar with home terrain | Home defense: Hard Rails safety boundaries, command blocklist, permission auditing | Interceptor + Sandbox isolation validation, absolute loyal guard |
| UI / CLI Presentation | Tibetan Mastiff *(zangao)* | Majestic, imposing, steadfast, gatekeeper | Global watchkeeping & terminal interaction: TUI status rendering, log dashboard, human confirmation for critical ops | TUI (e.g. Bubbletea) interface, providing steady, authoritative interaction & status summary |
| Code Hunter | Xigou (Greyhound) *(xigou)* | Streamlined, lightning-fast, laser-focused | Precision hunting: automated Refactor design, high-difficulty Bug fix code generation | Focused on high-difficulty code optimization, automated security vulnerability "hunting" & repair |
| RAG / Retriever | Golden Retriever *(jinmao)* | Strong retrieval instinct, gentle, dependable | Vector search, context fetching, document association | Integrates Vector DB (Milvus/Qdrant), responsible for Context Engine |
| Log & Bug Tracer | German Shepherd *(demu)* | Alert, black-backed, upright ears, strong execution | Rigorous Panic tracking, StackTrace analysis, Log tracing | Collects Agent execution logs, traces error scenes and pinpoints root cause |
