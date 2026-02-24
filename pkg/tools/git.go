package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// gitBase provides shared git command execution for all git tools.
type gitBase struct {
	workspace string
}

func (g *gitBase) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.workspace
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// --- git_status ---

// GitStatusTool shows the working tree status.
type GitStatusTool struct{ gitBase }

// NewGitStatusTool creates a new git_status tool.
func NewGitStatusTool(workspace string) *GitStatusTool {
	return &GitStatusTool{gitBase{workspace: workspace}}
}

func (t *GitStatusTool) Name() string        { return "git_status" }
func (t *GitStatusTool) Description() string { return "Show the working tree status (staged, unstaged, untracked files)" }
func (t *GitStatusTool) Parameters() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (t *GitStatusTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	out, err := t.run(ctx, "status")
	if err != nil {
		return ErrorResult(fmt.Sprintf("git status failed: %s", out))
	}
	return NewToolResult(out)
}

// --- git_diff ---

// GitDiffTool shows changes in the working tree or staging area.
type GitDiffTool struct{ gitBase }

// NewGitDiffTool creates a new git_diff tool.
func NewGitDiffTool(workspace string) *GitDiffTool {
	return &GitDiffTool{gitBase{workspace: workspace}}
}

func (t *GitDiffTool) Name() string        { return "git_diff" }
func (t *GitDiffTool) Description() string { return "Show changes between commits, staging area, and working tree" }
func (t *GitDiffTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"staged": map[string]any{
				"type":        "boolean",
				"description": "Show staged changes (--cached) instead of unstaged",
			},
			"file": map[string]any{
				"type":        "string",
				"description": "Limit diff to a specific file path",
			},
		},
	}
}

func (t *GitDiffTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	gitArgs := []string{"diff"}

	if staged, ok := args["staged"].(bool); ok && staged {
		gitArgs = append(gitArgs, "--cached")
	}

	if file, ok := args["file"].(string); ok && file != "" {
		gitArgs = append(gitArgs, "--", file)
	}

	out, err := t.run(ctx, gitArgs...)
	if err != nil {
		return ErrorResult(fmt.Sprintf("git diff failed: %s", out))
	}
	if out == "" {
		return NewToolResult("No differences found.")
	}
	return NewToolResult(out)
}

// --- git_log ---

// GitLogTool shows the commit history.
type GitLogTool struct{ gitBase }

// NewGitLogTool creates a new git_log tool.
func NewGitLogTool(workspace string) *GitLogTool {
	return &GitLogTool{gitBase{workspace: workspace}}
}

func (t *GitLogTool) Name() string        { return "git_log" }
func (t *GitLogTool) Description() string { return "Show the commit history" }
func (t *GitLogTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"max_count": map[string]any{
				"type":        "number",
				"description": "Maximum number of commits to show (default: 10)",
			},
			"oneline": map[string]any{
				"type":        "boolean",
				"description": "Use compact one-line format",
			},
			"file": map[string]any{
				"type":        "string",
				"description": "Limit log to commits that modified this file",
			},
		},
	}
}

func (t *GitLogTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	maxCount := 10
	if mc, ok := args["max_count"].(float64); ok && mc > 0 {
		maxCount = int(mc)
	}

	gitArgs := []string{"log", "-n", strconv.Itoa(maxCount)}

	if oneline, ok := args["oneline"].(bool); ok && oneline {
		gitArgs = append(gitArgs, "--oneline")
	}

	if file, ok := args["file"].(string); ok && file != "" {
		gitArgs = append(gitArgs, "--", file)
	}

	out, err := t.run(ctx, gitArgs...)
	if err != nil {
		return ErrorResult(fmt.Sprintf("git log failed: %s", out))
	}
	if out == "" {
		return NewToolResult("No commits found.")
	}
	return NewToolResult(out)
}

// --- git_show ---

// GitShowTool shows details of a specific commit.
type GitShowTool struct{ gitBase }

// NewGitShowTool creates a new git_show tool.
func NewGitShowTool(workspace string) *GitShowTool {
	return &GitShowTool{gitBase{workspace: workspace}}
}

func (t *GitShowTool) Name() string        { return "git_show" }
func (t *GitShowTool) Description() string { return "Show commit details including diff" }
func (t *GitShowTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"ref": map[string]any{
				"type":        "string",
				"description": "Commit reference to show (default: HEAD)",
			},
		},
	}
}

func (t *GitShowTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	ref := "HEAD"
	if r, ok := args["ref"].(string); ok && r != "" {
		ref = r
	}

	out, err := t.run(ctx, "show", ref)
	if err != nil {
		return ErrorResult(fmt.Sprintf("git show failed: %s", out))
	}
	return NewToolResult(out)
}

// --- git_branch ---

// GitBranchTool lists or creates branches.
type GitBranchTool struct{ gitBase }

// NewGitBranchTool creates a new git_branch tool.
func NewGitBranchTool(workspace string) *GitBranchTool {
	return &GitBranchTool{gitBase{workspace: workspace}}
}

func (t *GitBranchTool) Name() string        { return "git_branch" }
func (t *GitBranchTool) Description() string { return "List or create branches" }
func (t *GitBranchTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "Name of the branch to create (omit to list branches)",
			},
			"list": map[string]any{
				"type":        "boolean",
				"description": "List all branches",
			},
		},
	}
}

func (t *GitBranchTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	name, hasName := args["name"].(string)
	if hasName && name != "" {
		out, err := t.run(ctx, "branch", name)
		if err != nil {
			return ErrorResult(fmt.Sprintf("git branch create failed: %s", out))
		}
		return NewToolResult(fmt.Sprintf("Branch '%s' created.", name))
	}

	// Default: list branches
	out, err := t.run(ctx, "branch")
	if err != nil {
		return ErrorResult(fmt.Sprintf("git branch failed: %s", out))
	}
	return NewToolResult(out)
}

// --- git_commit ---

// GitCommitTool creates a commit.
type GitCommitTool struct{ gitBase }

// NewGitCommitTool creates a new git_commit tool.
func NewGitCommitTool(workspace string) *GitCommitTool {
	return &GitCommitTool{gitBase{workspace: workspace}}
}

func (t *GitCommitTool) Name() string        { return "git_commit" }
func (t *GitCommitTool) Description() string { return "Create a git commit with a message" }
func (t *GitCommitTool) Parameters() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"message"},
		"properties": map[string]any{
			"message": map[string]any{
				"type":        "string",
				"description": "Commit message",
			},
			"files": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Files to stage before committing (optional)",
			},
		},
	}
}

func (t *GitCommitTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	message, ok := args["message"].(string)
	if !ok || message == "" {
		return ErrorResult("'message' parameter is required")
	}

	// Stage files if provided
	if files, ok := args["files"].([]any); ok && len(files) > 0 {
		addArgs := []string{"add"}
		for _, f := range files {
			if s, ok := f.(string); ok {
				addArgs = append(addArgs, s)
			}
		}
		out, err := t.run(ctx, addArgs...)
		if err != nil {
			return ErrorResult(fmt.Sprintf("git add failed: %s", out))
		}
	}

	out, err := t.run(ctx, "commit", "-m", message)
	if err != nil {
		return ErrorResult(fmt.Sprintf("git commit failed: %s", out))
	}
	return NewToolResult(out)
}

// --- git_add ---

// GitAddTool stages files for commit.
type GitAddTool struct{ gitBase }

// NewGitAddTool creates a new git_add tool.
func NewGitAddTool(workspace string) *GitAddTool {
	return &GitAddTool{gitBase{workspace: workspace}}
}

func (t *GitAddTool) Name() string        { return "git_add" }
func (t *GitAddTool) Description() string { return "Stage files for commit" }
func (t *GitAddTool) Parameters() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"files"},
		"properties": map[string]any{
			"files": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "List of file paths to stage",
			},
		},
	}
}

func (t *GitAddTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	files, ok := args["files"].([]any)
	if !ok || len(files) == 0 {
		return ErrorResult("'files' parameter is required and must be a non-empty list")
	}

	addArgs := []string{"add"}
	for _, f := range files {
		if s, ok := f.(string); ok {
			addArgs = append(addArgs, s)
		}
	}

	out, err := t.run(ctx, addArgs...)
	if err != nil {
		return ErrorResult(fmt.Sprintf("git add failed: %s", out))
	}
	return NewToolResult("Files staged successfully.")
}

// --- git_reset ---

// GitResetTool unstages files from the staging area.
type GitResetTool struct{ gitBase }

// NewGitResetTool creates a new git_reset tool.
func NewGitResetTool(workspace string) *GitResetTool {
	return &GitResetTool{gitBase{workspace: workspace}}
}

func (t *GitResetTool) Name() string        { return "git_reset" }
func (t *GitResetTool) Description() string { return "Unstage files from the staging area" }
func (t *GitResetTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"files": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Files to unstage (omit to unstage all)",
			},
		},
	}
}

func (t *GitResetTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	gitArgs := []string{"reset", "HEAD"}

	if files, ok := args["files"].([]any); ok && len(files) > 0 {
		gitArgs = append(gitArgs, "--")
		for _, f := range files {
			if s, ok := f.(string); ok {
				gitArgs = append(gitArgs, s)
			}
		}
	}

	out, err := t.run(ctx, gitArgs...)
	if err != nil {
		return ErrorResult(fmt.Sprintf("git reset failed: %s", out))
	}
	return NewToolResult("Files unstaged successfully.")
}

// --- git_checkout ---

// GitCheckoutTool switches branches or restores files.
type GitCheckoutTool struct{ gitBase }

// NewGitCheckoutTool creates a new git_checkout tool.
func NewGitCheckoutTool(workspace string) *GitCheckoutTool {
	return &GitCheckoutTool{gitBase{workspace: workspace}}
}

func (t *GitCheckoutTool) Name() string        { return "git_checkout" }
func (t *GitCheckoutTool) Description() string { return "Switch branches or restore working tree files" }
func (t *GitCheckoutTool) Parameters() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"ref"},
		"properties": map[string]any{
			"ref": map[string]any{
				"type":        "string",
				"description": "Branch name or commit reference to check out",
			},
		},
	}
}

func (t *GitCheckoutTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	ref, ok := args["ref"].(string)
	if !ok || ref == "" {
		return ErrorResult("'ref' parameter is required")
	}

	out, err := t.run(ctx, "checkout", ref)
	if err != nil {
		return ErrorResult(fmt.Sprintf("git checkout failed: %s", out))
	}
	return NewToolResult(fmt.Sprintf("Switched to '%s'.", ref))
}

// --- git_pull ---

// GitPullTool pulls changes from a remote repository.
type GitPullTool struct{ gitBase }

// NewGitPullTool creates a new git_pull tool.
func NewGitPullTool(workspace string) *GitPullTool {
	return &GitPullTool{gitBase{workspace: workspace}}
}

func (t *GitPullTool) Name() string        { return "git_pull" }
func (t *GitPullTool) Description() string { return "Pull changes from a remote repository" }
func (t *GitPullTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"remote": map[string]any{
				"type":        "string",
				"description": "Remote name (default: origin)",
			},
			"branch": map[string]any{
				"type":        "string",
				"description": "Branch to pull (default: current branch)",
			},
		},
	}
}

func (t *GitPullTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	remote := "origin"
	if r, ok := args["remote"].(string); ok && r != "" {
		remote = r
	}

	gitArgs := []string{"pull", remote}
	if branch, ok := args["branch"].(string); ok && branch != "" {
		gitArgs = append(gitArgs, branch)
	}

	out, err := t.run(ctx, gitArgs...)
	if err != nil {
		return ErrorResult(fmt.Sprintf("git pull failed: %s", out))
	}
	if out == "" {
		return NewToolResult("Already up to date.")
	}
	return NewToolResult(out)
}

// --- git_merge ---

// GitMergeTool merges a branch into the current branch.
type GitMergeTool struct{ gitBase }

// NewGitMergeTool creates a new git_merge tool.
func NewGitMergeTool(workspace string) *GitMergeTool {
	return &GitMergeTool{gitBase{workspace: workspace}}
}

func (t *GitMergeTool) Name() string        { return "git_merge" }
func (t *GitMergeTool) Description() string { return "Merge a branch into the current branch" }
func (t *GitMergeTool) Parameters() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"branch"},
		"properties": map[string]any{
			"branch": map[string]any{
				"type":        "string",
				"description": "Branch name to merge into the current branch",
			},
		},
	}
}

func (t *GitMergeTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	branch, ok := args["branch"].(string)
	if !ok || branch == "" {
		return ErrorResult("'branch' parameter is required")
	}

	out, err := t.run(ctx, "merge", branch)
	if err != nil {
		return ErrorResult(fmt.Sprintf("git merge failed: %s", out))
	}
	return NewToolResult(out)
}

// --- git_stash ---

// GitStashTool manages the git stash (push, pop, list).
type GitStashTool struct{ gitBase }

// NewGitStashTool creates a new git_stash tool.
func NewGitStashTool(workspace string) *GitStashTool {
	return &GitStashTool{gitBase{workspace: workspace}}
}

func (t *GitStashTool) Name() string        { return "git_stash" }
func (t *GitStashTool) Description() string { return "Stash changes in the working directory (push, pop, or list)" }
func (t *GitStashTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": "Stash action: push (default), pop, or list",
				"enum":        []string{"push", "pop", "list"},
			},
			"message": map[string]any{
				"type":        "string",
				"description": "Message for the stash entry (only for push)",
			},
		},
	}
}

func (t *GitStashTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	action := "push"
	if a, ok := args["action"].(string); ok && a != "" {
		action = a
	}

	switch action {
	case "push":
		gitArgs := []string{"stash", "push"}
		if msg, ok := args["message"].(string); ok && msg != "" {
			gitArgs = append(gitArgs, "-m", msg)
		}
		out, err := t.run(ctx, gitArgs...)
		if err != nil {
			return ErrorResult(fmt.Sprintf("git stash push failed: %s", out))
		}
		return NewToolResult(out)

	case "pop":
		out, err := t.run(ctx, "stash", "pop")
		if err != nil {
			return ErrorResult(fmt.Sprintf("git stash pop failed: %s", out))
		}
		return NewToolResult(out)

	case "list":
		out, err := t.run(ctx, "stash", "list")
		if err != nil {
			return ErrorResult(fmt.Sprintf("git stash list failed: %s", out))
		}
		if out == "" {
			return NewToolResult("No stashes found.")
		}
		return NewToolResult(out)

	default:
		return ErrorResult(fmt.Sprintf("invalid action '%s': use push, pop, or list", action))
	}
}

// --- git_push ---

// GitPushTool pushes commits to a remote repository.
type GitPushTool struct {
	gitBase
	allowPush bool
}

// NewGitPushTool creates a new git_push tool.
func NewGitPushTool(workspace string, allowPush bool) *GitPushTool {
	return &GitPushTool{gitBase: gitBase{workspace: workspace}, allowPush: allowPush}
}

func (t *GitPushTool) Name() string        { return "git_push" }
func (t *GitPushTool) Description() string { return "Push commits to a remote repository" }
func (t *GitPushTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"remote": map[string]any{
				"type":        "string",
				"description": "Remote name (default: origin)",
			},
			"branch": map[string]any{
				"type":        "string",
				"description": "Branch to push (default: current branch)",
			},
		},
	}
}

func (t *GitPushTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	if !t.allowPush {
		return ErrorResult("Push is disabled by configuration. Set tools.git.allow_push=true to enable.")
	}

	remote := "origin"
	if r, ok := args["remote"].(string); ok && r != "" {
		remote = r
	}

	gitArgs := []string{"push", remote}
	if branch, ok := args["branch"].(string); ok && branch != "" {
		gitArgs = append(gitArgs, branch)
	}

	out, err := t.run(ctx, gitArgs...)
	if err != nil {
		return ErrorResult(fmt.Sprintf("git push failed: %s", out))
	}
	return NewToolResult(out)
}
