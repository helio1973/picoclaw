<div align="center">

  <img src="assets/millibee-logo.png" alt="MilliBee" width="200">

  <h1>MilliBee</h1>
  <p><i>She ships.</i></p>

  <h3>Lean AI Assistant &middot; 13 Git Tools &middot; Streaming Output &middot; Runs Anywhere</h3>

  <p>
    <img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go&logoColor=white" alt="Go">
    <img src="https://img.shields.io/badge/Docker-ready-2496ED?style=flat&logo=docker&logoColor=white" alt="Docker">
    <img src="https://img.shields.io/badge/RAM-<10MB-ff69b4" alt="Memory">
    <img src="https://img.shields.io/badge/license-MIT-green" alt="License">
  </p>

</div>

---

**MilliBee** is a lean, dockerized AI assistant built in Go. She's small, fast, and doesn't mess around.

Fork of [PicoClaw](https://github.com/sipeed/picoclaw) — stripped of the hardware marketing, sharpened into a practical coding companion with native git integration and streaming responses.

## What She Does

- **13 native git tools** — status, diff, log, show, branch, commit, add, reset, checkout, pull, merge, stash, push. No shell injection, configurable push policy.
- **Streaming output** — tokens appear as the LLM thinks. Anthropic and OpenAI-compatible providers supported.
- **< 10MB RAM** — single Go binary, boots in under a second.
- **Multi-channel** — Telegram, Discord, QQ, DingTalk, LINE, WeCom, or just your terminal.
- **Multi-provider** — OpenAI, Anthropic, DeepSeek, Zhipu, Groq, Ollama, OpenRouter, and 15+ more. Zero-code provider addition via `model_list` config.
- **Memory vault** — persistent notes the agent can save, search, and recall across sessions.
- **Tool sandbox** — workspace-restricted file ops, exec safety guards, configurable security policies.

## Quick Start

### Docker (recommended)

```bash
git clone https://github.com/helio1973/picoclaw.git milliclaw
cd milliclaw

cp config/config.example.json config/config.json
# Edit config.json — set your API key

docker compose --profile gateway up -d
```

One-shot mode:

```bash
docker compose run --rm picoclaw-agent -m "Explain this repo's architecture"
```

### From Source

```bash
git clone https://github.com/helio1973/picoclaw.git milliclaw
cd milliclaw
make build

# Alias it
alias milli='./build/picoclaw'

milli onboard
milli agent -m "Hello"
```

### Binary

Download from [releases](https://github.com/helio1973/picoclaw/releases), rename to `milli`, done.

## Configuration

Config lives at `~/.picoclaw/config.json`.

### Minimal Config

```json
{
  "model_list": [
    {
      "model_name": "claude-sonnet-4.6",
      "model": "anthropic/claude-sonnet-4.6",
      "api_key": "sk-ant-your-key"
    }
  ],
  "agents": {
    "defaults": {
      "model": "claude-sonnet-4.6",
      "workspace": "~/.picoclaw/workspace"
    }
  }
}
```

### Git Tools Config

Git tools are enabled by default. Push is disabled by default (opt-in for safety):

```json
{
  "tools": {
    "git": {
      "enabled": true,
      "allow_push": false
    }
  }
}
```

Available git tools:

| Tool | Description | Key Parameters |
|------|-------------|----------------|
| `git_status` | Working tree status | — |
| `git_diff` | Show changes | `staged`, `file` |
| `git_log` | Commit history | `max_count`, `oneline`, `file` |
| `git_show` | Commit details | `ref` |
| `git_branch` | List/create branches | `name`, `list` |
| `git_commit` | Create commit | `message`, `files` |
| `git_add` | Stage files | `files` |
| `git_reset` | Unstage files | `files` |
| `git_checkout` | Switch branch/restore | `ref` |
| `git_pull` | Pull from remote | `remote`, `branch` |
| `git_merge` | Merge branch | `branch` |
| `git_stash` | Stash changes | `action` (push/pop/list), `message` |
| `git_push` | Push to remote | `remote`, `branch` |

### Streaming

Streaming works automatically when your provider supports it. Anthropic and OpenAI-compatible providers stream out of the box. The TUI shows tokens as they arrive; non-streaming providers fall back gracefully.

### Providers

| Vendor | Prefix | Protocol |
|--------|--------|----------|
| OpenAI | `openai/` | OpenAI |
| Anthropic | `anthropic/` | Anthropic |
| DeepSeek | `deepseek/` | OpenAI |
| Zhipu | `zhipu/` | OpenAI |
| Groq | `groq/` | OpenAI |
| Ollama | `ollama/` | OpenAI |
| OpenRouter | `openrouter/` | OpenAI |
| Cerebras | `cerebras/` | OpenAI |
| Qwen | `qwen/` | OpenAI |
| Gemini | `gemini/` | OpenAI |
| Moonshot | `moonshot/` | OpenAI |
| NVIDIA | `nvidia/` | OpenAI |
| VLLM | `vllm/` | OpenAI |

Use `vendor/model` format in `model_list`. Custom endpoints via `api_base`.

### Chat Channels

<details>
<summary><b>Telegram</b> (easiest)</summary>

1. Talk to `@BotFather`, create bot, copy token
2. Add to config:

```json
{
  "channels": {
    "telegram": {
      "enabled": true,
      "token": "YOUR_BOT_TOKEN",
      "allow_from": ["YOUR_USER_ID"]
    }
  }
}
```

3. `milli gateway`

</details>

<details>
<summary><b>Discord</b></summary>

1. Create app at https://discord.com/developers/applications
2. Enable MESSAGE CONTENT INTENT
3. Configure:

```json
{
  "channels": {
    "discord": {
      "enabled": true,
      "token": "YOUR_BOT_TOKEN",
      "allow_from": ["YOUR_USER_ID"],
      "mention_only": false
    }
  }
}
```

4. Invite bot (scopes: `bot`, permissions: Send Messages + Read History)
5. `milli gateway`

</details>

<details>
<summary><b>Console</b> (terminal, no setup)</summary>

```bash
milli tui
```

Full Bubble Tea TUI with streaming, markdown rendering, and a spinner.

</details>

Other channels: QQ, DingTalk, LINE, WeCom — see [upstream docs](https://github.com/sipeed/picoclaw#-chat-apps).

## Architecture

```
milli gateway          milli agent -m "..."       milli tui
     │                        │                       │
     ▼                        ▼                       ▼
 MessageBus ──► AgentLoop ◄── ProcessDirect    ProcessDirectStreaming
                    │                                  │
                    ▼                                  ▼
              ToolRegistry                    StreamingLLMProvider
              (13 git tools                   (ChatStream + onChunk)
               + file ops
               + exec
               + memory
               + web search)
```

### Key Interfaces

```go
// Standard provider
type LLMProvider interface {
    Chat(ctx, messages, tools, model, options) (*LLMResponse, error)
}

// Streaming provider (optional, non-breaking)
type StreamingLLMProvider interface {
    LLMProvider
    ChatStream(ctx, messages, tools, model, options, onChunk) (*LLMResponse, error)
}

// Tool interface
type Tool interface {
    Name() string
    Description() string
    Parameters() map[string]any
    Execute(ctx, args) *ToolResult
}
```

## Security

- **Workspace sandbox**: file tools restricted to workspace by default
- **Exec guards**: blocks `rm -rf`, `format`, `dd`, fork bombs, etc.
- **Git push opt-in**: `allow_push: false` by default
- **No shell interpolation**: git tools use `exec.Command("git", ...)` directly
- **Per-tool policies**: enable/disable, rate limits, max arg size

## CLI

| Command | Description |
|---------|-------------|
| `milli onboard` | Initialize config & workspace |
| `milli agent -m "..."` | One-shot chat |
| `milli agent` | Interactive CLI |
| `milli tui` | Bubble Tea TUI with streaming |
| `milli gateway` | Start multi-channel gateway |
| `milli status` | Show status |
| `milli cron list` | List scheduled jobs |

## Workspace Layout

```
~/.picoclaw/workspace/
├── sessions/          # Conversation history
├── memory/            # Persistent memory vault
├── state/             # Runtime state
├── cron/              # Scheduled jobs
├── skills/            # Custom skills
├── AGENTS.md          # Agent behavior
├── IDENTITY.md        # Agent identity
└── USER.md            # User preferences
```

## License

MIT

---

<div align="center">
  <i>She's small. She's fast. She ships.</i>
</div>
