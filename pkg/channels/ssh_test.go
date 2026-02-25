package channels

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/tui"
)

func TestNewSSHChannel_Basic(t *testing.T) {
	cfg := config.SSHConfig{
		Enabled: true,
		Address: "127.0.0.1:0",
	}
	msgBus := bus.NewMessageBus()

	ch, err := NewSSHChannel(cfg, msgBus)
	if err != nil {
		t.Fatalf("NewSSHChannel() error: %v", err)
	}
	if ch.Name() != "ssh" {
		t.Errorf("Name() = %q, want 'ssh'", ch.Name())
	}
	if ch.IsRunning() {
		t.Error("should not be running before Start()")
	}
}

func TestSSHChannel_IsAllowed_EmptyList(t *testing.T) {
	cfg := config.SSHConfig{Enabled: true, Address: "127.0.0.1:0"}
	msgBus := bus.NewMessageBus()

	ch, err := NewSSHChannel(cfg, msgBus)
	if err != nil {
		t.Fatalf("NewSSHChannel() error: %v", err)
	}

	if !ch.IsAllowed("anyone") {
		t.Error("empty allowlist should allow anyone")
	}
}

func TestSSHChannel_IsAllowed_WithList(t *testing.T) {
	cfg := config.SSHConfig{
		Enabled:   true,
		Address:   "127.0.0.1:0",
		AllowFrom: []string{"alice", "bob"},
	}
	msgBus := bus.NewMessageBus()

	ch, err := NewSSHChannel(cfg, msgBus)
	if err != nil {
		t.Fatalf("NewSSHChannel() error: %v", err)
	}

	if !ch.IsAllowed("alice") {
		t.Error("alice should be allowed")
	}
	if !ch.IsAllowed("bob") {
		t.Error("bob should be allowed")
	}
	if ch.IsAllowed("eve") {
		t.Error("eve should not be allowed")
	}
}

func TestSSHChannel_HandleMessage_PublishesToBus(t *testing.T) {
	cfg := config.SSHConfig{Enabled: true, Address: "127.0.0.1:0"}
	msgBus := bus.NewMessageBus()

	ch, err := NewSSHChannel(cfg, msgBus)
	if err != nil {
		t.Fatalf("NewSSHChannel() error: %v", err)
	}

	ch.HandleMessage("alice", "ssh:alice", "hello", nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	msg, ok := msgBus.ConsumeInbound(ctx)
	if !ok {
		t.Fatal("expected inbound message from bus")
	}
	if msg.Channel != "ssh" {
		t.Errorf("Channel = %q, want 'ssh'", msg.Channel)
	}
	if msg.SenderID != "alice" {
		t.Errorf("SenderID = %q, want 'alice'", msg.SenderID)
	}
	if msg.ChatID != "ssh:alice" {
		t.Errorf("ChatID = %q, want 'ssh:alice'", msg.ChatID)
	}
	if msg.Content != "hello" {
		t.Errorf("Content = %q, want 'hello'", msg.Content)
	}
}

func TestSSHChannel_Send_NoSession(t *testing.T) {
	cfg := config.SSHConfig{Enabled: true, Address: "127.0.0.1:0"}
	msgBus := bus.NewMessageBus()

	ch, err := NewSSHChannel(cfg, msgBus)
	if err != nil {
		t.Fatalf("NewSSHChannel() error: %v", err)
	}

	err = ch.Send(context.Background(), bus.OutboundMessage{
		Channel: "ssh",
		ChatID:  "ssh:nobody",
		Content: "hello",
	})
	if err == nil {
		t.Error("Send() to non-existent session should return error")
	}
}

func TestSSHChannel_Send_WithSession(t *testing.T) {
	cfg := config.SSHConfig{Enabled: true, Address: "127.0.0.1:0"}
	msgBus := bus.NewMessageBus()

	ch, err := NewSSHChannel(cfg, msgBus)
	if err != nil {
		t.Fatalf("NewSSHChannel() error: %v", err)
	}

	// Register a fake session
	outChan := make(chan string, 10)
	ch.registerSession("ssh:alice", outChan)

	err = ch.Send(context.Background(), bus.OutboundMessage{
		Channel: "ssh",
		ChatID:  "ssh:alice",
		Content: "response text",
	})
	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}

	select {
	case msg := <-outChan:
		if msg != "response text" {
			t.Errorf("outChan received %q, want 'response text'", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message on outChan")
	}

	// Cleanup
	ch.unregisterSession("ssh:alice")
}

func TestSSHChannel_StartStop(t *testing.T) {
	cfg := config.SSHConfig{
		Enabled: true,
		Address: "127.0.0.1:0", // random port
	}
	msgBus := bus.NewMessageBus()

	ch, err := NewSSHChannel(cfg, msgBus)
	if err != nil {
		t.Fatalf("NewSSHChannel() error: %v", err)
	}

	ctx := context.Background()
	if err := ch.Start(ctx); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	if !ch.IsRunning() {
		t.Error("should be running after Start()")
	}

	// Verify something is listening
	addr := ch.ListenAddr()
	if addr == "" {
		t.Fatal("ListenAddr() should return address after Start()")
	}

	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("could not connect to SSH server at %s: %v", addr, err)
	}
	conn.Close()

	// Stop
	stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := ch.Stop(stopCtx); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}

	if ch.IsRunning() {
		t.Error("should not be running after Stop()")
	}

	// Verify port is closed (may take a moment)
	time.Sleep(100 * time.Millisecond)
	conn, err = net.DialTimeout("tcp", addr, 500*time.Millisecond)
	if err == nil {
		conn.Close()
		t.Error("should not be able to connect after Stop()")
	}
}

func TestSSHChannel_PasswordAuth(t *testing.T) {
	cfg := config.SSHConfig{
		Enabled:  true,
		Address:  "127.0.0.1:0",
		Password: "secret123",
	}
	msgBus := bus.NewMessageBus()

	ch, err := NewSSHChannel(cfg, msgBus)
	if err != nil {
		t.Fatalf("NewSSHChannel() error: %v", err)
	}

	// Verify password validator is set
	if !ch.hasPasswordAuth() {
		t.Error("channel should have password auth when password is configured")
	}

	// Verify validator accepts correct password
	if !ch.validatePassword("anyuser", "secret123") {
		t.Error("correct password should be accepted")
	}

	// Verify validator rejects wrong password
	if ch.validatePassword("anyuser", "wrong") {
		t.Error("wrong password should be rejected")
	}
}

func TestSSHChannel_NoPasswordAuth(t *testing.T) {
	cfg := config.SSHConfig{
		Enabled: true,
		Address: "127.0.0.1:0",
	}
	msgBus := bus.NewMessageBus()

	ch, err := NewSSHChannel(cfg, msgBus)
	if err != nil {
		t.Fatalf("NewSSHChannel() error: %v", err)
	}

	if ch.hasPasswordAuth() {
		t.Error("channel should not have password auth when no password configured")
	}
}

// ── sshModel TUI tests (Stap 6) ──────────────────────────────────────

// newTestSSHModel creates a sshModel with default test config for TUI tests.
func newTestSSHModel(t *testing.T) *sshModel {
	t.Helper()
	cfg := config.SSHConfig{Enabled: true, Address: "127.0.0.1:0"}
	msgBus := bus.NewMessageBus()
	ch, err := NewSSHChannel(cfg, msgBus)
	if err != nil {
		t.Fatalf("NewSSHChannel: %v", err)
	}
	outChan := make(chan string, 64)
	return newSSHModel(ch, "testuser", "ssh:testuser", tui.DefaultStyles(), outChan, nil)
}

func TestSSHModel_Init_ReturnsCommands(t *testing.T) {
	m := newTestSSHModel(t)
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init() should return commands (textarea blink, spinner tick, outbound listener)")
	}
}

func TestSSHModel_WindowResize_InitializesViewport(t *testing.T) {
	m := newTestSSHModel(t)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model := updated.(*sshModel)
	if !model.ready {
		t.Error("should be ready after WindowSizeMsg")
	}
	if model.width != 120 {
		t.Errorf("width = %d, want 120", model.width)
	}
	if model.height != 40 {
		t.Errorf("height = %d, want 40", model.height)
	}
}

func TestSSHModel_OutboundMsg_AddsAssistantMessage(t *testing.T) {
	m := newTestSSHModel(t)
	// Init viewport
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(*sshModel)
	m.processing = true

	updated, _ = m.Update(sshOutboundMsg{content: "Hello from agent"})
	model := updated.(*sshModel)

	if model.processing {
		t.Error("should not be processing after receiving response")
	}
	if len(model.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(model.messages))
	}
	if model.messages[0].Role != "assistant" {
		t.Errorf("role = %q, want 'assistant'", model.messages[0].Role)
	}
	if model.messages[0].Content != "Hello from agent" {
		t.Errorf("content = %q, want 'Hello from agent'", model.messages[0].Content)
	}
}

func TestSSHModel_CtrlC_UnregistersAndQuits(t *testing.T) {
	cfg := config.SSHConfig{Enabled: true, Address: "127.0.0.1:0"}
	msgBus := bus.NewMessageBus()
	ch, _ := NewSSHChannel(cfg, msgBus)

	outChan := make(chan string, 64)
	chatID := "ssh:testuser"
	ch.registerSession(chatID, outChan)

	m := newSSHModel(ch, "testuser", chatID, tui.DefaultStyles(), outChan, nil)
	// Init viewport
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(*sshModel)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Error("Ctrl+C should return quit command")
	}

	// Verify session was unregistered
	ch.sessionsMu.RLock()
	_, exists := ch.sessions[chatID]
	ch.sessionsMu.RUnlock()
	if exists {
		t.Error("session should be unregistered after Ctrl+C")
	}
}

func TestSSHModel_SendMessage_PublishesToBus(t *testing.T) {
	cfg := config.SSHConfig{Enabled: true, Address: "127.0.0.1:0"}
	msgBus := bus.NewMessageBus()
	ch, _ := NewSSHChannel(cfg, msgBus)

	outChan := make(chan string, 64)
	m := newSSHModel(ch, "testuser", "ssh:testuser", tui.DefaultStyles(), outChan, nil)
	// Init viewport
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(*sshModel)

	// Set textarea value and press Enter
	m.textarea.SetValue("hello world")
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(*sshModel)

	if !model.processing {
		t.Error("should be processing after sending message")
	}
	if len(model.messages) == 0 {
		t.Fatal("expected at least 1 message")
	}
	if model.messages[0].Role != "user" {
		t.Errorf("role = %q, want 'user'", model.messages[0].Role)
	}
	if model.messages[0].Content != "hello world" {
		t.Errorf("content = %q, want 'hello world'", model.messages[0].Content)
	}

	// Verify message was published to bus
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	msg, ok := msgBus.ConsumeInbound(ctx)
	if !ok {
		t.Fatal("expected inbound message on bus")
	}
	if msg.Content != "hello world" {
		t.Errorf("bus content = %q, want 'hello world'", msg.Content)
	}
	if msg.Channel != "ssh" {
		t.Errorf("bus channel = %q, want 'ssh'", msg.Channel)
	}
	if msg.SenderID != "testuser" {
		t.Errorf("bus senderID = %q, want 'testuser'", msg.SenderID)
	}
}

func TestSSHModel_SendMessage_EmptyIgnored(t *testing.T) {
	m := newTestSSHModel(t)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(*sshModel)

	// Enter with empty textarea should be ignored
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(*sshModel)
	if model.processing {
		t.Error("should not be processing after empty Enter")
	}
	if len(model.messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(model.messages))
	}
}

func TestSSHModel_View_NotReady(t *testing.T) {
	m := newTestSSHModel(t)
	view := m.View()
	if view == "" {
		t.Error("View() should return something when not ready")
	}
}

func TestSSHModel_View_Ready(t *testing.T) {
	m := newTestSSHModel(t)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(*sshModel)

	view := m.View()
	if view == "" {
		t.Error("View() should return content when ready")
	}
}

func TestSSHModel_RenderMessages(t *testing.T) {
	m := newTestSSHModel(t)
	m.messages = []tui.ChatMessage{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there"},
		{Role: "error", Content: "something broke"},
	}
	result := m.renderMessages()
	if result == "" {
		t.Error("renderMessages should return non-empty for messages")
	}
}

func TestListenOutbound(t *testing.T) {
	ch := make(chan string, 1)
	ch <- "response"

	cmd := listenOutbound(ch)
	msg := cmd()
	outMsg, ok := msg.(sshOutboundMsg)
	if !ok {
		t.Fatalf("expected sshOutboundMsg, got %T", msg)
	}
	if outMsg.content != "response" {
		t.Errorf("content = %q, want 'response'", outMsg.content)
	}
}

func TestListenOutbound_ClosedChannel(t *testing.T) {
	ch := make(chan string)
	close(ch)

	cmd := listenOutbound(ch)
	msg := cmd()
	if _, ok := msg.(sshSessionClosedMsg); !ok {
		t.Fatalf("expected sshSessionClosedMsg, got %T", msg)
	}
}

// Helper to get a free port for testing
func getFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port, nil
}

func freeAddr(t *testing.T) string {
	t.Helper()
	port, err := getFreePort()
	if err != nil {
		t.Fatalf("getFreePort: %v", err)
	}
	return fmt.Sprintf("127.0.0.1:%d", port)
}
