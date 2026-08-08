# Architecture Overview

## Three-Layer Principle

```mermaid
graph TB
    subgraph "Platform Layer"
        Router[Router]
        SOP[SOPGuardian]
        Hooks[HookPipeline]
        Skills[SkillManager]
    end
    subgraph "Adapter Layer"
        Unified[Unified Executor]
        Claude[claude adapter]
        Codex[codex adapter]
        Gemini[gemini adapter]
        Opencode[opencode adapter]
    end
    subgraph "CLI Layer"
        CLI1[claude CLI]
        CLI2[codex CLI]
        CLI3[gemini CLI]
        CLI4[opencode CLI]
    end
    Router --> Unified
    Unified --> Claude & Codex & Gemini & Opencode
    Claude --> CLI1
    Codex --> CLI2
    Gemini --> CLI3
    Opencode --> CLI4
    Hooks -.-> Unified
    SOP -.-> Unified
    Skills -.-> Unified
```

## A2A Handoff Flow

```mermaid
sequenceDiagram
    participant A as Breed A
    participant Hub as A2AHub
    participant SOP as SOPGuardian
    participant B as Breed B
    A->>Hub: Handoff(thread, fromA, toB)
    Hub->>SOP: CheckA2ADepth(thread)
    SOP-->>Hub: Continue
    Hub->>B: Execute with handoff context
    B-->>Hub: Response
    Hub-->>A: Handoff complete
```

## Data Flow

```mermaid
graph LR
    User[User Input] --> WS[WebSocket Handler]
    WS --> Router[Router]
    Router --> Adapter[CLI Adapter]
    Adapter --> CLI[CLI Process]
    CLI -->|stream-json| Adapter
    Adapter -->|stream| WS
    WS -->|stream| User
    CLI -->|@mention| Hub[A2AHub]
    Hub --> SOP[SOPGuardian]
    SOP --> NextBreed[Next Breed]
    NextBreed --> CLI2[CLI Process 2]
```
