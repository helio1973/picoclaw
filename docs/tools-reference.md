# Tools Reference

Complete overview of all native tools available to PicoClaw agents.

## Filesystem Tools

Always-on tools registered for every agent. Respect workspace restriction when `restrict_to_workspace: true`.

| Tool | Description | Parameters |
|------|-------------|------------|
| `read_file` | Read the contents of a file | `path` (required) |
| `write_file` | Write content to a file (creates parent dirs) | `path` (required), `content` (required) |
| `edit_file` | Replace exact text in a file (must match uniquely) | `path` (required), `old_text` (required), `new_text` (required) |
| `append_file` | Append content to end of file (creates if missing) | `path` (required), `content` (required) |
| `list_dir` | List files and directories in a path | `path` (optional, default: `.`) |
| `glob` | Find files matching a glob pattern | `pattern` (required, supports `**`), `path` (optional) |
| `grep` | Search file contents using regex | `pattern` (required, regex), `path` (optional), `glob` (optional, file filter), `max_results` (optional, default: 100) |

### Workspace Sandbox

When `restrict_to_workspace` is enabled (default), all filesystem tools use `os.Root` to enforce strict sandboxing:

- Paths outside the workspace are rejected
- Symlinks resolving outside the workspace are blocked
- Relative paths are resolved against the workspace root

### Glob Patterns

The `glob` tool supports recursive matching via `**`:

- `*.go` — Go files in the root directory
- `**/*.go` — Go files in any subdirectory
- `src/**/*.ts` — TypeScript files under `src/`
- `**/test_*.py` — Python test files anywhere

Results are capped at 1000 files.

### Grep Output

The `grep` tool returns matches in the format `filepath:line_number:content`:

```
src/main.go:42:func main() {
src/utils.go:15:func helper() string {
```

Binary files are automatically skipped. Results are capped at `max_results` (default: 100).

## Shell Tool

| Tool | Description | Parameters |
|------|-------------|------------|
| `exec` | Execute a shell command | `command` (required) |

Configurable via `tools.exec` in config. See [Tools Configuration](tools_configuration.md#exec-tool) for deny patterns.

## Web Tools

Registered in the shared tool loop. Web search requires at least one search provider enabled.

| Tool | Description | Parameters |
|------|-------------|------------|
| `web_search` | Search the web for current information | `query` (required) |
| `web_fetch` | Fetch a URL and extract readable content | `url` (required) |

Configurable via `tools.web` in config. See [Tools Configuration](tools_configuration.md#web-tools).

## Sidecar Tools (Docker)

Conditionally registered — only available when `enabled: true` in config. Require companion Docker services.

| Tool | Description | Parameters | Service |
|------|-------------|------------|---------|
| `deep_scrape` | Scrape JS-heavy pages via headless browser | `url` (required), `max_depth` (optional), `max_pages` (optional) | Crawl4AI |
| `youtube_transcript` | Extract captions from YouTube videos | `url` or `video_id` (one required), `language` (optional) | YouTube Transcript API |
| `transcribe_audio` | Transcribe audio files to text | `file_path` (required), `language` (optional), `output` (optional: txt/json/srt/vtt) | Whisper ASR |

Configurable via `tools.sidecars` in config. See [Tools Configuration](tools_configuration.md#sidecar-tools) and [Docker Installation](docker-installation.md).

## Memory Tools

Always-on tools for persistent note storage in the memory vault.

| Tool | Description | Parameters |
|------|-------------|------------|
| `memory_save` | Save a structured note with frontmatter metadata | `title` (required), `content` (required), `tags` (optional), `aliases` (optional) |
| `memory_search` | Search the vault by tags, title, or text | `query` (required) |
| `memory_recall` | Read full note content by path or topic | `path` (required) |

## Git Tools

Native git tools for safe version control operations. Registered per-agent when `tools.git.enabled` is `true` (default).

| Tool | Description | Parameters |
|------|-------------|------------|
| `git_status` | Show working tree status | — |
| `git_diff` | Show changes | `staged` (bool), `file` (optional) |
| `git_log` | Show commit history | `max_count` (default: 10), `oneline` (bool), `file` (optional) |
| `git_show` | Show commit details | `ref` (default: HEAD) |
| `git_branch` | List or create branches | `name` (optional, create), `list` (bool) |
| `git_commit` | Create a commit | `message` (required), `files` (optional, auto-stage) |
| `git_add` | Stage files | `files` (required) |
| `git_reset` | Unstage files | `files` (optional, omit to unstage all) |
| `git_checkout` | Switch branch or restore files | `ref` (required) |
| `git_pull` | Pull changes from remote | `remote` (default: origin), `branch` (optional) |
| `git_merge` | Merge a branch into the current branch | `branch` (required) |
| `git_stash` | Stash changes (push, pop, list) | `action` (push/pop/list, default: push), `message` (optional, for push) |
| `git_push` | Push to remote | `remote` (default: origin), `branch` (optional) |

Configurable via `tools.git` in config:

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

`git_push` is disabled by default. Set `allow_push: true` to enable. Destructive operations (`force push`, `reset --hard`, `clean -f`) are not available.

## Scheduling Tool

| Tool | Description | Parameters |
|------|-------------|------------|
| `cron` | Schedule reminders, tasks, or commands | `message` (required), `at_seconds` (one-time), `every_seconds` (recurring), `cron_expr` (cron schedule), `command` (shell command) |

Configurable via `tools.cron` in config. See [Tools Configuration](tools_configuration.md#cron-tool).

## Agent Tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `spawn` | Spawn a subagent in the background (async) | `task` (required), `agent` (optional) |
| `subagent` | Execute a subagent synchronously | `task` (required), `agent` (optional) |

## Skill Management Tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `find_skills` | Search skill registries for installable skills | `query` (required) |
| `install_skill` | Install a skill from a registry by slug | `slug` (required) |

Configurable via `tools.skills` in config. See [Tools Configuration](tools_configuration.md#skills-tool).

## Communication Tool

| Tool | Description | Parameters |
|------|-------------|------------|
| `message` | Send a message to the user on a chat channel | `content` (required) |

## Hardware Tools (Linux only)

Available on Linux for IoT/embedded use cases. Return errors on other platforms.

| Tool | Description | Parameters |
|------|-------------|------------|
| `i2c` | Interact with I2C bus devices | `action` (required: detect/scan/read/write), `bus` (optional), `address` (optional), `data` (optional) |
| `spi` | Interact with SPI bus devices | `action` (required: list/transfer/read), `device` (optional), `data` (optional) |

## Tool Registration Summary

| Category | Registration | Config required |
|----------|-------------|-----------------|
| Filesystem (`read_file`, `write_file`, `edit_file`, `append_file`, `list_dir`, `glob`, `grep`) | Per-agent (`instance.go`) | No |
| Memory (`memory_save`, `memory_search`, `memory_recall`) | Per-agent (`instance.go`) | No |
| Git (`git_status`, `git_diff`, `git_log`, `git_show`, `git_branch`, `git_commit`, `git_add`, `git_reset`, `git_checkout`, `git_push`) | Per-agent (`instance.go`) | Optional (`allow_push`) |
| Shell (`exec`) | Per-agent (`instance.go`) | Optional deny patterns |
| Web (`web_search`, `web_fetch`) | Shared (`loop.go`) | At least one search provider |
| Sidecars (`deep_scrape`, `youtube_transcript`, `transcribe_audio`) | Shared (`loop.go`) | `enabled: true` + Docker service |
| Scheduling (`cron`) | Shared (`loop.go`) | Optional timeout |
| Agent (`spawn`, `subagent`) | Shared (`loop.go`) | No |
| Skills (`find_skills`, `install_skill`) | Shared (`loop.go`) | Registry config |
| Communication (`message`) | Shared (`loop.go`) | No |
| Hardware (`i2c`, `spi`) | Shared (`loop.go`) | Linux only |

## CLI Modes

PicoClaw provides three ways to interact with the agent:

### Agent Mode (basic)

```bash
picoclaw agent -m "Hello"          # One-shot query
picoclaw agent                      # Interactive REPL (readline)
picoclaw agent -s my-session        # Custom session
picoclaw agent --model claude-sonnet-4.6  # Override model
```

### TUI Mode (rich terminal UI)

```bash
picoclaw tui                        # Full-screen Bubble Tea UI
picoclaw chat                       # Alias for tui
picoclaw tui -s my-session          # Custom session
picoclaw tui --model claude-sonnet-4.6    # Override model
```

Features: markdown rendering (Glamour), scrollable chat history, multi-line input, thinking spinner. Uses the same `AgentLoop.ProcessDirect()` as agent mode.

### Gateway Mode (long-running bot)

```bash
picoclaw gateway                    # Start with all enabled channels
```

Connects to external platforms (Telegram, Discord, Slack, etc.) via the channel system. See `config.json` for channel configuration.

## Console Channel

The console channel makes the gateway interactive from the terminal. When enabled, the gateway reads stdin and writes responses to stdout — like agent mode, but integrated into the full gateway with all services (cron, heartbeat, channels).

```json
{
  "channels": {
    "console": {
      "enabled": true
    }
  }
}
```

Unlike the internal `cli` channel used by agent mode, `console` is a real external channel that participates in outbound message dispatch.
