package tools

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// setupGitRepo creates a temporary directory with an initialized git repository.
// It configures user.email and user.name so that commits can be made.
func setupGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")
	return dir
}

// commitFile is a helper that creates a file, stages it, and commits it.
func commitFile(t *testing.T, dir, filename, content, message string) {
	t.Helper()
	path := filepath.Join(dir, filename)
	err := os.WriteFile(path, []byte(content), 0o644)
	if err != nil {
		t.Fatalf("write file %s: %v", filename, err)
	}
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	run("add", filename)
	run("commit", "-m", message)
}

// --- git_status tests ---

func TestGitStatus_Clean(t *testing.T) {
	dir := setupGitRepo(t)
	// Make an initial commit so the repo is not empty
	commitFile(t, dir, "init.txt", "init", "initial commit")

	tool := NewGitStatusTool(dir)
	assert.Equal(t, "git_status", tool.Name())
	assert.NotEmpty(t, tool.Description())

	ctx := context.Background()
	result := tool.Execute(ctx, map[string]any{})

	assert.False(t, result.IsError, "expected no error, got: %s", result.ForLLM)
	assert.True(t,
		strings.Contains(result.ForLLM, "nothing to commit") ||
			strings.Contains(result.ForLLM, "clean"),
		"expected clean status, got: %s", result.ForLLM)
}

func TestGitStatus_Modified(t *testing.T) {
	dir := setupGitRepo(t)
	commitFile(t, dir, "file.txt", "original", "add file")

	// Modify the file without staging
	err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("modified"), 0o644)
	assert.NoError(t, err)

	tool := NewGitStatusTool(dir)
	ctx := context.Background()
	result := tool.Execute(ctx, map[string]any{})

	assert.False(t, result.IsError, "expected no error, got: %s", result.ForLLM)
	assert.Contains(t, result.ForLLM, "file.txt",
		"expected modified file in status output")
}

// --- git_diff tests ---

func TestGitDiff_Unstaged(t *testing.T) {
	dir := setupGitRepo(t)
	commitFile(t, dir, "hello.txt", "hello\n", "add hello")

	// Modify the tracked file without staging
	err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello world\n"), 0o644)
	assert.NoError(t, err)

	tool := NewGitDiffTool(dir)
	assert.Equal(t, "git_diff", tool.Name())

	ctx := context.Background()
	result := tool.Execute(ctx, map[string]any{
		"staged": false,
	})

	assert.False(t, result.IsError, "expected no error, got: %s", result.ForLLM)
	assert.Contains(t, result.ForLLM, "hello world",
		"expected diff to show the new content")
}

func TestGitDiff_Staged(t *testing.T) {
	dir := setupGitRepo(t)
	commitFile(t, dir, "hello.txt", "hello\n", "add hello")

	// Modify and stage the change
	err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("staged change\n"), 0o644)
	assert.NoError(t, err)

	cmd := exec.Command("git", "add", "hello.txt")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	assert.NoError(t, err, "git add failed: %s", out)

	tool := NewGitDiffTool(dir)
	ctx := context.Background()
	result := tool.Execute(ctx, map[string]any{
		"staged": true,
	})

	assert.False(t, result.IsError, "expected no error, got: %s", result.ForLLM)
	assert.Contains(t, result.ForLLM, "staged change",
		"expected staged diff to show the new content")
}

func TestGitDiff_File(t *testing.T) {
	dir := setupGitRepo(t)
	commitFile(t, dir, "a.txt", "aaa\n", "add a")
	commitFile(t, dir, "b.txt", "bbb\n", "add b")

	// Modify both files
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("aaa modified\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("bbb modified\n"), 0o644)

	tool := NewGitDiffTool(dir)
	ctx := context.Background()
	result := tool.Execute(ctx, map[string]any{
		"staged": false,
		"file":   "a.txt",
	})

	assert.False(t, result.IsError, "expected no error, got: %s", result.ForLLM)
	assert.Contains(t, result.ForLLM, "aaa modified",
		"expected diff to contain changes for a.txt")
}

// --- git_log tests ---

func TestGitLog_Default(t *testing.T) {
	dir := setupGitRepo(t)
	commitFile(t, dir, "first.txt", "1", "first commit")
	commitFile(t, dir, "second.txt", "2", "second commit")

	tool := NewGitLogTool(dir)
	assert.Equal(t, "git_log", tool.Name())

	ctx := context.Background()
	result := tool.Execute(ctx, map[string]any{})

	assert.False(t, result.IsError, "expected no error, got: %s", result.ForLLM)
	assert.Contains(t, result.ForLLM, "first commit",
		"expected log to contain first commit message")
	assert.Contains(t, result.ForLLM, "second commit",
		"expected log to contain second commit message")
}

func TestGitLog_MaxCount(t *testing.T) {
	dir := setupGitRepo(t)
	commitFile(t, dir, "a.txt", "a", "commit alpha")
	commitFile(t, dir, "b.txt", "b", "commit beta")
	commitFile(t, dir, "c.txt", "c", "commit gamma")

	tool := NewGitLogTool(dir)
	ctx := context.Background()
	result := tool.Execute(ctx, map[string]any{
		"max_count": float64(1),
	})

	assert.False(t, result.IsError, "expected no error, got: %s", result.ForLLM)
	assert.Contains(t, result.ForLLM, "commit gamma",
		"expected log to show the most recent commit")
	assert.NotContains(t, result.ForLLM, "commit alpha",
		"expected log to NOT contain the oldest commit with max_count=1")
}

func TestGitLog_Oneline(t *testing.T) {
	dir := setupGitRepo(t)
	commitFile(t, dir, "a.txt", "a", "commit oneline test")

	tool := NewGitLogTool(dir)
	ctx := context.Background()
	result := tool.Execute(ctx, map[string]any{
		"oneline": true,
	})

	assert.False(t, result.IsError, "expected no error, got: %s", result.ForLLM)
	assert.Contains(t, result.ForLLM, "commit oneline test")
	// Oneline format should be more compact (no Author/Date lines)
	assert.NotContains(t, result.ForLLM, "Author:",
		"oneline format should not contain Author line")
}

func TestGitLog_File(t *testing.T) {
	dir := setupGitRepo(t)
	commitFile(t, dir, "tracked.txt", "v1", "add tracked")
	commitFile(t, dir, "other.txt", "v1", "add other")
	commitFile(t, dir, "tracked.txt", "v2", "update tracked")

	tool := NewGitLogTool(dir)
	ctx := context.Background()
	result := tool.Execute(ctx, map[string]any{
		"file": "tracked.txt",
	})

	assert.False(t, result.IsError, "expected no error, got: %s", result.ForLLM)
	assert.Contains(t, result.ForLLM, "update tracked")
	assert.Contains(t, result.ForLLM, "add tracked")
	assert.NotContains(t, result.ForLLM, "add other",
		"expected file-filtered log to not contain commits for other files")
}

// --- git_show tests ---

func TestGitShow_Default(t *testing.T) {
	dir := setupGitRepo(t)
	commitFile(t, dir, "show.txt", "show me", "commit to show")

	tool := NewGitShowTool(dir)
	assert.Equal(t, "git_show", tool.Name())

	ctx := context.Background()
	result := tool.Execute(ctx, map[string]any{})

	assert.False(t, result.IsError, "expected no error, got: %s", result.ForLLM)
	assert.Contains(t, result.ForLLM, "commit to show",
		"expected show output to contain commit message")
	assert.Contains(t, result.ForLLM, "show me",
		"expected show output to contain file content diff")
}

func TestGitShow_WithRef(t *testing.T) {
	dir := setupGitRepo(t)
	commitFile(t, dir, "first.txt", "first", "first for show")
	commitFile(t, dir, "second.txt", "second", "second for show")

	tool := NewGitShowTool(dir)
	ctx := context.Background()
	// Show HEAD~1 (the first commit)
	result := tool.Execute(ctx, map[string]any{
		"ref": "HEAD~1",
	})

	assert.False(t, result.IsError, "expected no error, got: %s", result.ForLLM)
	assert.Contains(t, result.ForLLM, "first for show",
		"expected show HEAD~1 to contain the first commit message")
}

// --- git_branch tests ---

func TestGitBranch_List(t *testing.T) {
	dir := setupGitRepo(t)
	// Need at least one commit for branches to exist
	commitFile(t, dir, "init.txt", "init", "initial commit")

	tool := NewGitBranchTool(dir)
	assert.Equal(t, "git_branch", tool.Name())

	ctx := context.Background()
	result := tool.Execute(ctx, map[string]any{
		"list": true,
	})

	assert.False(t, result.IsError, "expected no error, got: %s", result.ForLLM)
	// Default branch could be main or master depending on git config
	assert.True(t,
		strings.Contains(result.ForLLM, "main") || strings.Contains(result.ForLLM, "master"),
		"expected branch list to contain main or master, got: %s", result.ForLLM)
}

func TestGitBranch_Create(t *testing.T) {
	dir := setupGitRepo(t)
	commitFile(t, dir, "init.txt", "init", "initial commit")

	tool := NewGitBranchTool(dir)
	ctx := context.Background()

	// Create a new branch
	result := tool.Execute(ctx, map[string]any{
		"name": "feature-test",
	})

	assert.False(t, result.IsError, "expected no error, got: %s", result.ForLLM)

	// Verify the branch exists by listing
	listResult := tool.Execute(ctx, map[string]any{
		"list": true,
	})

	assert.False(t, listResult.IsError)
	assert.Contains(t, listResult.ForLLM, "feature-test",
		"expected new branch to appear in branch list")
}

// --- git_commit tests ---

func TestGitCommit_Success(t *testing.T) {
	dir := setupGitRepo(t)
	// Create and stage a file
	path := filepath.Join(dir, "commit-me.txt")
	err := os.WriteFile(path, []byte("commit this"), 0o644)
	assert.NoError(t, err)

	cmd := exec.Command("git", "add", "commit-me.txt")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	assert.NoError(t, err, "git add failed: %s", out)

	tool := NewGitCommitTool(dir)
	assert.Equal(t, "git_commit", tool.Name())

	ctx := context.Background()
	result := tool.Execute(ctx, map[string]any{
		"message": "test commit message",
	})

	assert.False(t, result.IsError, "expected no error, got: %s", result.ForLLM)

	// Verify the commit was made by checking the log
	logCmd := exec.Command("git", "log", "--oneline", "-1")
	logCmd.Dir = dir
	logOut, err := logCmd.CombinedOutput()
	assert.NoError(t, err)
	assert.Contains(t, string(logOut), "test commit message")
}

func TestGitCommit_NoMessage(t *testing.T) {
	dir := setupGitRepo(t)

	tool := NewGitCommitTool(dir)
	ctx := context.Background()
	result := tool.Execute(ctx, map[string]any{})

	assert.True(t, result.IsError, "expected error when no message provided")
	assert.NotEmpty(t, result.ForLLM, "expected error message in ForLLM")
}

func TestGitCommit_WithFiles(t *testing.T) {
	dir := setupGitRepo(t)
	commitFile(t, dir, "init.txt", "init", "initial commit")

	// Create files but do not stage them
	os.WriteFile(filepath.Join(dir, "auto1.txt"), []byte("auto1"), 0o644)
	os.WriteFile(filepath.Join(dir, "auto2.txt"), []byte("auto2"), 0o644)

	tool := NewGitCommitTool(dir)
	ctx := context.Background()
	result := tool.Execute(ctx, map[string]any{
		"message": "commit with auto-stage",
		"files":   []any{"auto1.txt", "auto2.txt"},
	})

	assert.False(t, result.IsError, "expected no error, got: %s", result.ForLLM)

	// Verify both files were committed
	logCmd := exec.Command("git", "log", "--oneline", "-1")
	logCmd.Dir = dir
	logOut, err := logCmd.CombinedOutput()
	assert.NoError(t, err)
	assert.Contains(t, string(logOut), "commit with auto-stage")
}

// --- git_add tests ---

func TestGitAdd_Files(t *testing.T) {
	dir := setupGitRepo(t)
	commitFile(t, dir, "init.txt", "init", "initial commit")

	// Create a new file
	newFile := filepath.Join(dir, "new-file.txt")
	err := os.WriteFile(newFile, []byte("new content"), 0o644)
	assert.NoError(t, err)

	tool := NewGitAddTool(dir)
	assert.Equal(t, "git_add", tool.Name())

	ctx := context.Background()
	result := tool.Execute(ctx, map[string]any{
		"files": []any{"new-file.txt"},
	})

	assert.False(t, result.IsError, "expected no error, got: %s", result.ForLLM)

	// Verify the file is staged
	statusCmd := exec.Command("git", "status", "--porcelain")
	statusCmd.Dir = dir
	statusOut, err := statusCmd.CombinedOutput()
	assert.NoError(t, err)
	output := string(statusOut)
	assert.Contains(t, output, "new-file.txt",
		"expected file to appear in status")
	assert.True(t, strings.Contains(output, "A ") || strings.Contains(output, "A  "),
		"expected file to be staged (A prefix), got: %s", output)
}

func TestGitAdd_NoFiles(t *testing.T) {
	dir := setupGitRepo(t)

	tool := NewGitAddTool(dir)
	ctx := context.Background()
	result := tool.Execute(ctx, map[string]any{})

	assert.True(t, result.IsError, "expected error when no files provided")
}

// --- git_reset tests ---

func TestGitReset_Unstage(t *testing.T) {
	dir := setupGitRepo(t)
	commitFile(t, dir, "init.txt", "init", "initial commit")

	// Create and stage a file
	path := filepath.Join(dir, "staged.txt")
	err := os.WriteFile(path, []byte("staged content"), 0o644)
	assert.NoError(t, err)

	cmd := exec.Command("git", "add", "staged.txt")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	assert.NoError(t, err, "git add failed: %s", out)

	// Verify file is staged
	statusCmd := exec.Command("git", "status", "--porcelain")
	statusCmd.Dir = dir
	beforeOut, _ := statusCmd.CombinedOutput()
	assert.Contains(t, string(beforeOut), "A")

	tool := NewGitResetTool(dir)
	assert.Equal(t, "git_reset", tool.Name())

	ctx := context.Background()
	result := tool.Execute(ctx, map[string]any{
		"files": []any{"staged.txt"},
	})

	assert.False(t, result.IsError, "expected no error, got: %s", result.ForLLM)

	// Verify the file is no longer staged
	statusCmd2 := exec.Command("git", "status", "--porcelain")
	statusCmd2.Dir = dir
	afterOut, err := statusCmd2.CombinedOutput()
	assert.NoError(t, err)
	output := string(afterOut)
	assert.Contains(t, output, "??",
		"expected file to be untracked after reset, got: %s", output)
}

func TestGitReset_All(t *testing.T) {
	dir := setupGitRepo(t)
	commitFile(t, dir, "init.txt", "init", "initial commit")

	// Create and stage multiple files
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0o644)

	addCmd := exec.Command("git", "add", "a.txt", "b.txt")
	addCmd.Dir = dir
	addCmd.CombinedOutput()

	tool := NewGitResetTool(dir)
	ctx := context.Background()
	// No files param = reset all
	result := tool.Execute(ctx, map[string]any{})

	assert.False(t, result.IsError, "expected no error, got: %s", result.ForLLM)

	// Verify both files are unstaged
	statusCmd := exec.Command("git", "status", "--porcelain")
	statusCmd.Dir = dir
	out, _ := statusCmd.CombinedOutput()
	output := string(out)
	assert.NotContains(t, output, "A ",
		"expected no staged files after full reset, got: %s", output)
}

// --- git_checkout tests ---

func TestGitCheckout_Branch(t *testing.T) {
	dir := setupGitRepo(t)
	commitFile(t, dir, "init.txt", "init", "initial commit")

	// Create a new branch
	branchCmd := exec.Command("git", "branch", "test-branch")
	branchCmd.Dir = dir
	out, err := branchCmd.CombinedOutput()
	assert.NoError(t, err, "git branch failed: %s", out)

	tool := NewGitCheckoutTool(dir)
	assert.Equal(t, "git_checkout", tool.Name())

	ctx := context.Background()
	result := tool.Execute(ctx, map[string]any{
		"ref": "test-branch",
	})

	assert.False(t, result.IsError, "expected no error, got: %s", result.ForLLM)

	// Verify HEAD is on the new branch
	headCmd := exec.Command("git", "symbolic-ref", "--short", "HEAD")
	headCmd.Dir = dir
	headOut, err := headCmd.CombinedOutput()
	assert.NoError(t, err)
	assert.Equal(t, "test-branch", strings.TrimSpace(string(headOut)))
}

func TestGitCheckout_NoRef(t *testing.T) {
	dir := setupGitRepo(t)
	commitFile(t, dir, "init.txt", "init", "initial commit")

	tool := NewGitCheckoutTool(dir)
	ctx := context.Background()
	result := tool.Execute(ctx, map[string]any{})

	assert.True(t, result.IsError, "expected error when no ref provided")
}

// --- git_pull tests ---

func TestGitPull_NoRemote(t *testing.T) {
	dir := setupGitRepo(t)
	commitFile(t, dir, "init.txt", "init", "initial commit")

	tool := NewGitPullTool(dir)
	assert.Equal(t, "git_pull", tool.Name())

	ctx := context.Background()
	result := tool.Execute(ctx, map[string]any{})

	// Should fail because there is no remote configured
	assert.True(t, result.IsError, "expected error because no remote is configured")
}

func TestGitPull_WithRemote(t *testing.T) {
	// Create a "remote" repo and a "local" clone
	remoteDir := setupGitRepo(t)
	commitFile(t, remoteDir, "init.txt", "init", "initial commit")

	localDir := t.TempDir()
	cmd := exec.Command("git", "clone", remoteDir, localDir)
	out, err := cmd.CombinedOutput()
	assert.NoError(t, err, "git clone failed: %s", out)

	// Add a new commit to the remote
	commitFile(t, remoteDir, "new.txt", "new", "remote commit")

	tool := NewGitPullTool(localDir)
	ctx := context.Background()
	result := tool.Execute(ctx, map[string]any{})

	assert.False(t, result.IsError, "expected no error, got: %s", result.ForLLM)
}

// --- git_merge tests ---

func TestGitMerge_Success(t *testing.T) {
	dir := setupGitRepo(t)
	commitFile(t, dir, "init.txt", "init", "initial commit")

	// Create a feature branch and add a commit
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	run("checkout", "-b", "feature")
	commitFile(t, dir, "feature.txt", "feature", "feature commit")
	run("checkout", "master")

	tool := NewGitMergeTool(dir)
	assert.Equal(t, "git_merge", tool.Name())

	ctx := context.Background()
	result := tool.Execute(ctx, map[string]any{"branch": "feature"})

	assert.False(t, result.IsError, "expected no error, got: %s", result.ForLLM)
}

func TestGitMerge_NoBranch(t *testing.T) {
	dir := setupGitRepo(t)

	tool := NewGitMergeTool(dir)
	ctx := context.Background()
	result := tool.Execute(ctx, map[string]any{})

	assert.True(t, result.IsError, "expected error when no branch provided")
}

// --- git_stash tests ---

func TestGitStash_Push(t *testing.T) {
	dir := setupGitRepo(t)
	commitFile(t, dir, "init.txt", "init", "initial commit")

	// Modify a tracked file
	os.WriteFile(filepath.Join(dir, "init.txt"), []byte("modified"), 0o644)

	tool := NewGitStashTool(dir)
	assert.Equal(t, "git_stash", tool.Name())
	assert.NotEmpty(t, tool.Description())

	ctx := context.Background()
	result := tool.Execute(ctx, map[string]any{
		"action": "push",
	})

	assert.False(t, result.IsError, "expected no error, got: %s", result.ForLLM)

	// Verify the working tree is clean after stash
	statusCmd := exec.Command("git", "status", "--porcelain")
	statusCmd.Dir = dir
	statusOut, _ := statusCmd.CombinedOutput()
	assert.Empty(t, strings.TrimSpace(string(statusOut)),
		"expected clean working tree after stash push")
}

func TestGitStash_PushWithMessage(t *testing.T) {
	dir := setupGitRepo(t)
	commitFile(t, dir, "init.txt", "init", "initial commit")

	os.WriteFile(filepath.Join(dir, "init.txt"), []byte("modified"), 0o644)

	tool := NewGitStashTool(dir)
	ctx := context.Background()
	result := tool.Execute(ctx, map[string]any{
		"action":  "push",
		"message": "work in progress",
	})

	assert.False(t, result.IsError, "expected no error, got: %s", result.ForLLM)

	// Verify the stash has the message
	listCmd := exec.Command("git", "stash", "list")
	listCmd.Dir = dir
	listOut, _ := listCmd.CombinedOutput()
	assert.Contains(t, string(listOut), "work in progress")
}

func TestGitStash_List(t *testing.T) {
	dir := setupGitRepo(t)
	commitFile(t, dir, "init.txt", "init", "initial commit")

	// Create a stash
	os.WriteFile(filepath.Join(dir, "init.txt"), []byte("stashed"), 0o644)
	stashCmd := exec.Command("git", "stash", "push", "-m", "test stash")
	stashCmd.Dir = dir
	stashCmd.CombinedOutput()

	tool := NewGitStashTool(dir)
	ctx := context.Background()
	result := tool.Execute(ctx, map[string]any{
		"action": "list",
	})

	assert.False(t, result.IsError, "expected no error, got: %s", result.ForLLM)
	assert.Contains(t, result.ForLLM, "test stash")
}

func TestGitStash_ListEmpty(t *testing.T) {
	dir := setupGitRepo(t)
	commitFile(t, dir, "init.txt", "init", "initial commit")

	tool := NewGitStashTool(dir)
	ctx := context.Background()
	result := tool.Execute(ctx, map[string]any{
		"action": "list",
	})

	assert.False(t, result.IsError, "expected no error, got: %s", result.ForLLM)
	assert.Contains(t, result.ForLLM, "No stashes found")
}

func TestGitStash_Pop(t *testing.T) {
	dir := setupGitRepo(t)
	commitFile(t, dir, "init.txt", "init", "initial commit")

	// Stash a change
	os.WriteFile(filepath.Join(dir, "init.txt"), []byte("to be stashed"), 0o644)
	stashCmd := exec.Command("git", "stash", "push")
	stashCmd.Dir = dir
	stashCmd.CombinedOutput()

	tool := NewGitStashTool(dir)
	ctx := context.Background()
	result := tool.Execute(ctx, map[string]any{
		"action": "pop",
	})

	assert.False(t, result.IsError, "expected no error, got: %s", result.ForLLM)

	// Verify the change is restored
	content, _ := os.ReadFile(filepath.Join(dir, "init.txt"))
	assert.Equal(t, "to be stashed", string(content))
}

func TestGitStash_InvalidAction(t *testing.T) {
	dir := setupGitRepo(t)

	tool := NewGitStashTool(dir)
	ctx := context.Background()
	result := tool.Execute(ctx, map[string]any{
		"action": "invalid",
	})

	assert.True(t, result.IsError, "expected error for invalid action")
}

func TestGitStash_NoAction(t *testing.T) {
	dir := setupGitRepo(t)

	tool := NewGitStashTool(dir)
	ctx := context.Background()
	result := tool.Execute(ctx, map[string]any{})

	// Default should be "push"
	// Will fail since there's nothing to stash, but should not be an "invalid action" error
	assert.True(t, result.IsError)
	assert.NotContains(t, result.ForLLM, "invalid action")
}

// --- git_push tests ---

func TestGitPush_Disabled(t *testing.T) {
	dir := setupGitRepo(t)
	commitFile(t, dir, "init.txt", "init", "initial commit")

	tool := NewGitPushTool(dir, false)
	assert.Equal(t, "git_push", tool.Name())

	ctx := context.Background()
	result := tool.Execute(ctx, map[string]any{})

	assert.True(t, result.IsError, "expected error when push is disabled")
	assert.Contains(t, strings.ToLower(result.ForLLM), "push",
		"expected error message to mention push")
}

func TestGitPush_Enabled(t *testing.T) {
	dir := setupGitRepo(t)
	commitFile(t, dir, "init.txt", "init", "initial commit")

	tool := NewGitPushTool(dir, true)
	ctx := context.Background()
	result := tool.Execute(ctx, map[string]any{})

	// Push should fail because there is no remote configured, but it should
	// be a git error (the tool itself allowed the push attempt), not a tool-level
	// "push disabled" error.
	assert.True(t, result.IsError, "expected error because no remote is configured")
	assert.NotContains(t, strings.ToLower(result.ForLLM), "disabled",
		"error should be from git (no remote), not from tool policy")
}
