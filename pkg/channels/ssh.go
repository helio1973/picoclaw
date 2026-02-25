package channels

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	"github.com/charmbracelet/wish/bubbletea"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/tui"
)

// SSHChannel implements the Channel interface using a Wish SSH server
// that serves a Bubble Tea TUI to each connecting client.
type SSHChannel struct {
	*BaseChannel
	sshConfig  config.SSHConfig
	server     *ssh.Server
	listener   net.Listener
	sessions   map[string]chan string // chatID -> outbound channel
	sessionsMu sync.RWMutex
}

// NewSSHChannel creates a new SSH channel.
func NewSSHChannel(cfg config.SSHConfig, msgBus *bus.MessageBus) (*SSHChannel, error) {
	base := NewBaseChannel("ssh", cfg, msgBus, cfg.AllowFrom)

	ch := &SSHChannel{
		BaseChannel: base,
		sshConfig:   cfg,
		sessions:    make(map[string]chan string),
	}

	return ch, nil
}

// Start starts the Wish SSH server.
func (c *SSHChannel) Start(ctx context.Context) error {
	address := c.sshConfig.Address
	if address == "" {
		address = "0.0.0.0:2222"
	}

	opts := []ssh.Option{
		wish.WithAddress(address),
		wish.WithMiddleware(
			bubbletea.Middleware(c.teaHandler),
			activeterm.Middleware(),
		),
	}

	if c.sshConfig.HostKeyPath != "" {
		opts = append(opts, wish.WithHostKeyPath(c.sshConfig.HostKeyPath))
	}

	if c.hasPasswordAuth() {
		opts = append(opts, wish.WithPasswordAuth(func(_ ssh.Context, password string) bool {
			return c.validatePassword("", password)
		}))
	}

	srv, err := wish.NewServer(opts...)
	if err != nil {
		return fmt.Errorf("create SSH server: %w", err)
	}
	c.server = srv

	// Listen manually so we can retrieve the actual address (important for port 0)
	ln, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", address, err)
	}
	c.listener = ln

	c.setRunning(true)
	logger.InfoCF("ssh", "SSH channel started", map[string]any{
		"address": ln.Addr().String(),
	})

	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			logger.ErrorCF("ssh", "SSH server error", map[string]any{
				"error": err.Error(),
			})
		}
	}()

	return nil
}

// Stop shuts down the SSH server.
func (c *SSHChannel) Stop(ctx context.Context) error {
	logger.InfoC("ssh", "Stopping SSH channel")
	c.setRunning(false)

	if c.server != nil {
		if err := c.server.Shutdown(ctx); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			return fmt.Errorf("shutdown SSH server: %w", err)
		}
	}

	return nil
}

// Send delivers an outbound message to the appropriate SSH session.
func (c *SSHChannel) Send(ctx context.Context, msg bus.OutboundMessage) error {
	c.sessionsMu.RLock()
	outChan, ok := c.sessions[msg.ChatID]
	c.sessionsMu.RUnlock()

	if !ok {
		return fmt.Errorf("no SSH session for chatID %q", msg.ChatID)
	}

	select {
	case outChan <- msg.Content:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout sending to SSH session %q", msg.ChatID)
	}
}

// ListenAddr returns the actual address the server is listening on.
// Useful when started with port 0 (random port).
func (c *SSHChannel) ListenAddr() string {
	if c.listener != nil {
		return c.listener.Addr().String()
	}
	return ""
}

// registerSession registers an outbound channel for a chat session.
func (c *SSHChannel) registerSession(chatID string, outChan chan string) {
	c.sessionsMu.Lock()
	defer c.sessionsMu.Unlock()
	c.sessions[chatID] = outChan
}

// unregisterSession removes a chat session.
func (c *SSHChannel) unregisterSession(chatID string) {
	c.sessionsMu.Lock()
	defer c.sessionsMu.Unlock()
	delete(c.sessions, chatID)
}

// hasPasswordAuth returns true if password authentication is configured.
func (c *SSHChannel) hasPasswordAuth() bool {
	return c.sshConfig.Password != ""
}

// validatePassword checks if the provided password matches the configured password.
func (c *SSHChannel) validatePassword(_ string, password string) bool {
	return password == c.sshConfig.Password
}

// teaHandler creates a Bubble Tea model for each SSH session.
func (c *SSHChannel) teaHandler(sess ssh.Session) (tea.Model, []tea.ProgramOption) {
	username := sess.User()
	chatID := fmt.Sprintf("ssh:%s", username)

	renderer := bubbletea.MakeRenderer(sess)
	styles := tui.NewStyles(renderer)

	outChan := make(chan string, 64)
	c.registerSession(chatID, outChan)

	model := newSSHModel(c, username, chatID, styles, outChan, renderer)

	return model, []tea.ProgramOption{tea.WithAltScreen()}
}

// sshOutboundMsg carries a response delivered via the outbound channel.
type sshOutboundMsg struct{ content string }

// sshSessionClosedMsg signals that the outbound channel was closed.
type sshSessionClosedMsg struct{}

// listenOutbound returns a tea.Cmd that reads from the outbound channel.
func listenOutbound(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		content, ok := <-ch
		if !ok {
			return sshSessionClosedMsg{}
		}
		return sshOutboundMsg{content: content}
	}
}

// sshModel is the Bubble Tea model for an SSH session.
// It provides a full chat TUI and connects to the MessageBus
// via the SSHChannel's HandleMessage/Send flow.
type sshModel struct {
	channel  *SSHChannel
	username string
	chatID   string
	styles   tui.Styles
	outChan  chan string
	renderer *lipgloss.Renderer

	messages        []tui.ChatMessage
	textarea        textarea.Model
	viewport        viewport.Model
	spinner         spinner.Model
	processing      bool
	width           int
	height          int
	ready           bool
	glamourRenderer *glamour.TermRenderer
}

func newSSHModel(ch *SSHChannel, username, chatID string, styles tui.Styles, outChan chan string, renderer *lipgloss.Renderer) *sshModel {
	ta := textarea.New()
	ta.Placeholder = "Type your message... (Enter to send, Ctrl+C to quit)"
	ta.Focus()
	ta.CharLimit = 4096
	ta.SetHeight(3)
	ta.ShowLineNumbers = false

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = styles.Spinner

	gr, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)

	return &sshModel{
		channel:         ch,
		username:        username,
		chatID:          chatID,
		styles:          styles,
		outChan:         outChan,
		renderer:        renderer,
		messages:        []tui.ChatMessage{},
		textarea:        ta,
		spinner:         sp,
		glamourRenderer: gr,
	}
}

func (m *sshModel) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.spinner.Tick, listenOutbound(m.outChan))
}

func (m *sshModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			m.channel.unregisterSession(m.chatID)
			return m, tea.Quit
		case tea.KeyEnter:
			if !m.processing {
				return m.sendMessage()
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		headerHeight := 6 // textarea (3) + status (1) + padding (2)
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-headerHeight)
			m.viewport.SetContent(m.renderMessages())
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - headerHeight
		}
		m.textarea.SetWidth(msg.Width - 4)
		return m, nil

	case sshOutboundMsg:
		m.processing = false
		m.messages = append(m.messages, tui.ChatMessage{
			Role:    "assistant",
			Content: msg.content,
		})
		if m.ready {
			m.viewport.SetContent(m.renderMessages())
			m.viewport.GotoBottom()
		}
		m.textarea.Focus()
		return m, listenOutbound(m.outChan)

	case sshSessionClosedMsg:
		m.channel.unregisterSession(m.chatID)
		return m, tea.Quit

	case spinner.TickMsg:
		if m.processing {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	// Update textarea when not processing
	if !m.processing {
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Update viewport
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *sshModel) sendMessage() (*sshModel, tea.Cmd) {
	input := m.textarea.Value()
	if input == "" {
		return m, nil
	}

	m.messages = append(m.messages, tui.ChatMessage{
		Role:    "user",
		Content: input,
	})
	m.textarea.Reset()
	m.processing = true

	if m.ready {
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
	}

	// Publish to MessageBus via BaseChannel
	m.channel.HandleMessage(m.username, m.chatID, input, nil, nil)

	return m, m.spinner.Tick
}

func (m *sshModel) renderMessages() string {
	if len(m.messages) == 0 {
		return ""
	}

	var s string
	for _, msg := range m.messages {
		switch msg.Role {
		case "user":
			s += m.styles.User.Render("You: "+msg.Content) + "\n\n"
		case "assistant":
			rendered := msg.Content
			if m.glamourRenderer != nil {
				if r, err := m.glamourRenderer.Render(msg.Content); err == nil {
					rendered = r
				}
			}
			s += m.styles.Assistant.Render("Assistant:") + "\n" + rendered + "\n"
		case "error":
			s += m.styles.Error.Render(msg.Content) + "\n\n"
		}
	}
	return s
}

func (m *sshModel) View() string {
	if !m.ready {
		return fmt.Sprintf("Connecting as %s...", m.username)
	}

	var status string
	if m.processing {
		status = m.styles.Status.Render(m.spinner.View() + " Thinking...")
	} else {
		status = m.styles.Status.Render("Ready | Ctrl+C to quit")
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.viewport.View(),
		status,
		m.textarea.View(),
	)
}
