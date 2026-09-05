# GIC — AI-Native Rule Engine & Interactive Studio

[![Go](https://img.shields.io/badge/go-1.23%2B-00ADD8.svg)](https://go.dev/)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](./LICENSE)
[![Architecture](https://img.shields.io/badge/architecture-Single%20Binary-6366f1.svg)](#architecture)
[![UI](https://img.shields.io/badge/studio-Interactive%20Playground-10b981.svg)](#interactive-rule-studio)

> **GIC (feelc)** is a next-generation **AI-native business rules engine** (DMN/FEEL) compiled to Go. It bridges human-readable business logic, SMT-powered formal mathematical verification, and runtime AI perception — shipped as a single, high-performance static binary with a built-in modern execution studio.

---

## Key Capabilities

1. **Dual Execution Paradigm (Deterministic + Non-Deterministic Perception)**
   - **100% Deterministic Engine**: Evaluates decision tables and FEEL expressions in microseconds with zero runtime LLM dependency.
   - **Hybrid AI Perception (`infer` nodes)**: Supports live runtime evaluation of unstructured inputs (e.g., fraud memos, clinical triage notes, doctor narratives) resolved dynamically via OpenRouter, Anthropic, or OpenAI endpoints.
2. **Formal Mathematical Verification (SMT Prover)**
   - Before runtime execution, the compiler mathematically **proves completeness** (no uncovered edge cases), **absence of conflicts** (overlapping rules with conflicting outputs), and **redundancy** — generating concrete counterexample witnesses when proofs fail.
3. **Obsidian Dark Studio & Interactive Playground**
   - **⚡ Playground**: Test and stress-test compiled models interactively. Adjust parameters with synchronized dual range sliders and numeric boxes, load 1-click scenario presets (`Min Bounds`, `Max Bounds`, `Midpoint`, `🎲 Randomize`), and view winning rule paths.
   - **Live LLM Inspector**: Real-time inspection of prompts, raw completions, and parsed scores from OpenRouter when evaluating hybrid models.
   - **📝 Rules Studio**: In-browser syntax-highlighted editor with instant SMT formal verification diagnostics and traceability mapping.
   - **🗺️ DRG Graph**: Interactive Decision Requirements Graph with zoom, pan, and live firing path illumination.
   - **💬 AI Authoring**: Conversational rules drafting and automated business document ingestion with bounded repair loops.
4. **Single-Binary Zero-Dependency Architecture**
   - Web frontend, WASM runtime, and HTTP API are embedded directly into the Go binary (`//go:embed web`). No Node.js, Python, or external services required at runtime.
5. **Model Context Protocol (MCP)**
   - Built-in MCP server (`./feelc mcp`) enabling agents (Claude Code, Cursor, AGY) to draft, verify, and execute rules over standard I/O.

---

## Quick Start

### 1. Prerequisites
- **Go 1.23+** installed (`go version`)
- Git

### 2. Clone and Setup

```bash
# Clone the repository
git clone https://github.com/santosh-sankranthi/GIC.git
cd GIC

# Optional: Configure your OpenRouter / LLM credentials
cp .env.example .env
```

Edit `.env` if you wish to use your own OpenRouter key or model:
```env
OPENROUTER_API_KEY=your_openrouter_api_key_here
FEELC_LLM_PROVIDER=openrouter
FEELC_LLM_BASE_URL=https://openrouter.ai/api
FEELC_LLM_MODEL=nvidia/nemotron-3-ultra-550b-a55b:free
```

### 3. Build the Binary

```bash
go build -o feelc ./cmd/feelc
```

### 4. Launch the Server and Studio

```bash
./start-server.sh
```

Or run directly:
```bash
./feelc serve --project sample-project --addr :8080 --ui --allow-edit --watch
```

Open your browser at:
👉 **[http://localhost:8080](http://localhost:8080)**

---

## Interactive Rule Studio

The web interface embedded in `feelc serve --ui` provides 4 dedicated workspaces:

| Mode | Purpose |
| :--- | :--- |
| **⚡ Playground** | Test decisions live. Move dual range sliders, click 1-click scenario chips (`Prime Approval`, `Min Bounds`, `Randomize`), view live latency (`⏱ 2.4ms`), and inspect the Live LLM Runtime card for hybrid AI nodes. |
| **📝 Rules Studio** | Edit `.rules` source files with real-time compilation, SMT proofs, counter-example witness generation, and `@source` traceability. |
| **🗺️ DRG Graph** | Visual topological dependency graph with wheel-zoom, drag-pan, and node inspection. |
| **💬 AI Author** | Chat with your configured LLM to synthesize `.rules` from business specs with deterministic repair loops. |

---

## Writing Rules (`.rules` DSL)

### 1. Deterministic Decision Table Example (`credit.rules`)

```feelc
model "credit" {
  rounding: half_even
}

input credit_score  : number in [300..850]
input annual_income : number >= 0
input monthly_debt  : number >= 0
input age           : number in [0..120]

type Eligibility = context { eligible: boolean, reason: string }

decision dti : number = if annual_income > 0 then monthly_debt / (annual_income / 12) else (if monthly_debt > 0 then 1 else 0)

decision eligibility : Eligibility {
  needs: credit_score, dti, age
  hit: first
  #  credit_score | dti     | age   => eligible | reason
     < 580        | -       | -     => false    | "insufficient score"
     -            | > 0.43  | -     => false    | "debt too high"
     -            | -       | < 18  => false    | "minor"
     [580..680)   | <= 0.43 | >= 18 => true     | "approved with conditions"
     >= 680       | <= 0.43 | >= 18 => true     | "approved"
     default      |         |       => false    | "not covered"
}
```

### 2. Hybrid Deterministic + Live AI Perception Example (`fraud.rules`)

```feelc
model "hybrid_fraud_shield" {
  rounding: half_even
}

input amount : number >= 0
input country_risk_score : number in [0..100]
input velocity_last_24h : number in [0..50]
input transaction_memo : string

# Non-deterministic AI perception node resolved at runtime by OpenRouter
infer memo_risk_score : number in [0..100] {
  needs: transaction_memo
  prompt: "You are an AML risk specialist. Evaluate this transaction memo for money laundering or fraud indicators. Return ONLY a single integer score 0 (safe) to 100 (extreme danger): ${transaction_memo}"
}

# Mathematical risk synthesis
decision composite_risk : number = country_risk_score * 0.35 + velocity_last_24h * 1.5 + memo_risk_score * 0.5

type FraudVerdict = context { action: string, flag_code: string, review_tier: number }

decision verdict : FraudVerdict {
  needs: amount, composite_risk, memo_risk_score
  hit: first
  #  amount   | composite_risk | memo_risk_score => action                 | flag_code      | review_tier
     > 50000  | > 75           | -               => "FREEZE_AND_ESCALATE"  | "AML_CRITICAL" | 1
     -        | > 70           | > 80            => "BLOCK_TRANSACTION"    | "HIGH_AI_RISK" | 1
     > 10000  | > 45           | -               => "MANUAL_REVIEW"        | "VELOCITY_DTI" | 2
     -        | > 50           | -               => "STEP_UP_AUTH"         | "ELEVATED"     | 3
     default  |                |                 => "AUTO_APPROVE"         | "CLEAN"        | 0
}
```

---

## CLI Commands

### Formal Verification (SMT)
Prove completeness and conflict freedom before shipping:
```bash
./feelc verify --rules examples/credit/credit.rules
```

### Evaluate a Decision
Execute a decision directly from terminal:
```bash
./feelc run --rules examples/credit/credit.rules \
  --decision eligibility \
  --input '{"credit_score":720,"annual_income":80000,"monthly_debt":1200,"age":30}'
```

### Serve with UI & Watch Mode
```bash
./feelc serve --project sample-project --addr :8080 --ui --watch
```

### Run as Model Context Protocol (MCP) Server
Expose rule authoring, verification, and evaluation as MCP tools for Cursor, Claude Code, or AGY:
```bash
./feelc mcp
```

---

## REST API Reference

| Method | Path | Description |
| :--- | :--- | :--- |
| `POST` | `/v1/run` | Evaluates a decision given rules and JSON inputs. Returns output, trace, and `llmTrace`. |
| `POST` | `/v1/verify` | Compiles and runs SMT verification, returning blockers, warnings, and counterexamples. |
| `POST` | `/v1/graph` | Generates node/edge topology for Decision Requirements Graph visualization. |
| `GET` | `/v1/ai/status` | Checks server-side LLM connectivity and active provider/model. |
| `POST` | `/v1/chat` | Chat endpoint for authoring `.rules` with an LLM. |
| `POST` | `/v1/ingest` | Multi-round spec-to-rules drafting, verification, and automated repair. |
| `GET` | `/v1/project` | Retrieves multi-module workspace manifest and module health. |

### Execution Example via cURL

```bash
curl -X POST http://localhost:8080/v1/run \
  -H "Content-Type: application/json" \
  -d '{
    "rules": "model \"test\" { rounding: half_even } input x: number decision y: number = x * 2",
    "decision": "y",
    "input": {"x": 21},
    "full": true
  }'
```

---

## Project Structure

```
.
├── cmd/feelc/                  # CLI entry point and subcommands
├── internal/
│   ├── check/                  # SMT solver & formal verification engine
│   ├── compiler/               # .rules DSL lexer, parser, and IR generator
│   ├── dsl/                    # DSL definitions and AST nodes
│   ├── engine/                 # Deterministic execution VM & runtime resolvers
│   ├── genai/                  # OpenRouter, OpenAI, & Anthropic LLM connectors
│   ├── ir/                     # Typed Intermediate Representation
│   ├── project/                # Multi-module project linker & health checker
│   └── service/                # HTTP API server & embedded web assets
│       └── web/                # Studio UI (HTML, CSS, Vanilla JS)
├── examples/                   # Industry sample rule models (credit, tax, triage)
├── sample-project/             # Multi-module workspace demo
├── start-server.sh             # Startup script with OpenRouter support
└── .env.example                # Sample environment configuration
```

---

## License

Apache-2.0 License. See [LICENSE](./LICENSE) for details.
