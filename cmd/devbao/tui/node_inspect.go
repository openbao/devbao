package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/openbao/devbao/pkg/bao"
)

type nodeInspect struct {
	NodeName string
	Node     *bao.Node
	Running  bool

	ResumeStop *Button
	SealUnseal *Button
	Clean      *Button

	UnsealKeys []*textinput.Model
	RemoveKeys *Button
	AddKey     *Button

	Token *textinput.Model

	Save *Button

	FieldOrdering []Focusable

	Message string

	Width  int
	Height int
}

var _ tabModel = &nodeInspect{}

func NodeInspectModel(name string, node *bao.Node, running bool) tabModel {
	namespace := fmt.Sprintf("node-%v", name)

	model := &nodeInspect{
		NodeName: name,
		Node:     node,
		Running:  running,

		ResumeStop: NewButton(namespace, "Resume", nil),
		SealUnseal: NewButton(namespace, "Seal", nil),
		Clean:      NewButton(namespace, "Clean", nil),
		Save:       NewButton(namespace, "Save", nil),

		RemoveKeys: NewButton(namespace, "Remove All", nil),
		AddKey:     NewButton(namespace, "Add", nil),
	}

	model.ResumeStop.Action = func() tea.Cmd { return model.DoResumeStop() }
	model.SealUnseal.Action = func() tea.Cmd { return model.DoSealUnseal() }
	model.Clean.Action = func() tea.Cmd { return model.DoClean() }
	model.Save.Action = func() tea.Cmd { return model.DoSave() }

	model.RemoveKeys.Action = func() tea.Cmd { return model.DoRemoveKeys() }
	model.AddKey.Action = func() tea.Cmd { return model.DoAddKey() }

	for _, key := range node.UnsealKeys {
		input := textinput.New()
		input.SetValue(key)
		input.Placeholder = "set unseal key..."
		model.UnsealKeys = append(model.UnsealKeys, &input)
	}

	token := textinput.New()
	model.Token = &token

	model.ResumeStop.Focus()

	return model
}

func (m nodeInspect) Name() string { return "Node: " + m.NodeName }

func (m nodeInspect) InTextbox() bool {
	for _, candidate := range m.FieldOrdering {
		if candidate.Focused() {
			_, ok := candidate.(*textinput.Model)
			if ok {
				return true
			}
		}
	}

	return false
}

func (m nodeInspect) Closeable() bool {
	if m.InTextbox() {
		return false
	}

	return true
}

func (m *nodeInspect) Init() tea.Cmd {
	return m.RefreshState(true)
}

func (m *nodeInspect) RefreshState(hard bool) tea.Cmd {
	node, err := bao.LoadNode(m.NodeName)
	if err != nil {
		m.Message = fmt.Sprintf("failed to reload node: %v", err)
		return nil
	}

	m.Node = node

	m.Running = true
	if err := m.Node.Exec.ValidateRunning(); err != nil {
		m.Running = false
	}

	if !m.Running {
		m.ResumeStop.Text = "Resume"
		m.SealUnseal.Text = "Unseal"
		m.SealUnseal.Disabled = true
	} else {
		m.ResumeStop.Text = "Stop"

		client, err := m.Node.GetClient()
		if err != nil {
			m.Message = fmt.Sprintf("failed to get client: %v", err)
			return nil
		}

		status, err := client.Sys().SealStatus()
		if err != nil {
			m.Message = fmt.Sprintf("failed to get seal status: %v", err)
			return nil
		}

		if status.Sealed {
			m.SealUnseal.Text = "Unseal"
		} else {
			m.SealUnseal.Text = "Seal"
		}

		m.SealUnseal.Disabled = len(m.Node.UnsealKeys) == 0
	}

	if hard {
		if m.Node.Token != "" {
			m.Token.SetValue(m.Node.Token)
		} else {
			m.Token.SetValue("")
		}
		m.Token.Placeholder = "set root token..."

		m.UnsealKeys = nil
		for _, key := range m.Node.UnsealKeys {
			input := textinput.New()
			input.SetValue(key)
			input.Placeholder = "set unseal key..."
			m.UnsealKeys = append(m.UnsealKeys, &input)
		}
	}

	m.FieldOrdering = []Focusable{
		m.ResumeStop,
		m.SealUnseal,
		m.Clean,
	}

	for _, key := range m.UnsealKeys {
		m.FieldOrdering = append(m.FieldOrdering, key)
	}

	m.FieldOrdering = append(m.FieldOrdering, []Focusable{
		m.RemoveKeys,
		m.AddKey,
		m.Token,
		m.Save,
	}...)

	changed := len(m.UnsealKeys) != len(m.Node.UnsealKeys)
	if !changed {
		for index, key := range m.UnsealKeys {
			if key.Value() != m.Node.UnsealKeys[index] {
				changed = true
				break
			}
		}

		changed = changed || (m.Token.Value() != m.Node.Token)
	}

	m.Save.Disabled = !changed

	return nil
}

func (m *nodeInspect) DoResumeStop() tea.Cmd {
	m.Message = ""

	if !m.Running {
		if err := m.Node.Resume(); err != nil {
			m.Message = fmt.Sprintf("error resuming: %v", err)
			return nil
		}

		if m.Node.Config.Dev != nil {
			m.Message = "warning: resumed dev mode instance won't preserve state"
		}
	} else {
		if err := m.Node.Kill(); err != nil {
			m.Message = fmt.Sprintf("error killing: %v", err)
			return nil
		}
	}

	return m.RefreshState(false)
}

func (m *nodeInspect) DoSealUnseal() tea.Cmd {
	m.Message = ""

	if len(m.Node.UnsealKeys) == 0 {
		m.Message = "No unseal keys available; refusing to " + strings.ToLower(m.SealUnseal.Text)
		return nil
	}

	if m.SealUnseal.Text == "Seal" {
		client, err := m.Node.GetClient()
		if err != nil {
			m.Message = fmt.Sprintf("failed to get client: %v", err)
			return nil
		}

		if err := client.Sys().Seal(); err != nil {
			m.Message = fmt.Sprintf("failed to seal: %v", err)
			return nil
		}
	} else {
		unsealed, err := m.Node.Unseal()
		if err != nil {
			m.Message = fmt.Sprintf("failed to unseal: %v", err)
			return nil
		}
		if !unsealed {
			m.Message = "node already unsealed"
		}
	}

	return m.RefreshState(false)
}

func (m *nodeInspect) DoClean() tea.Cmd {
	m.Message = ""

	if m.Running {
		if err := m.Node.Kill(); err != nil {
			m.Message = fmt.Sprintf("failed to stop node: %w", err)
			return nil
		}
	}

	if err := m.Node.Clean(false); err != nil {
		m.Message = fmt.Sprintf("failed to clean node: %w", err)
		return nil
	}

	return TabCloseCmd
}

func (m *nodeInspect) DoSave() tea.Cmd {
	m.Message = ""

	if m.Token.Value() != m.Node.Token {
		if err := m.Node.ValidateToken(m.Token.Value()); err != nil {
			m.Message = fmt.Sprintf("invalid root token: %v", err)
			return nil
		}

		m.Node.Token = m.Token.Value()
	}

	var unsealKeys []string
	for _, key := range m.UnsealKeys {
		value := key.Value()
		if value != "" {
			unsealKeys = append(unsealKeys, value)
		}
	}

	if strings.Join(m.Node.UnsealKeys, "\n") != strings.Join(unsealKeys, "\n") {
		m.Node.UnsealKeys = unsealKeys
	}

	if err := m.Node.Validate(); err != nil {
		m.Message = fmt.Sprintf("invalid config: %v", err)

		// Still refresh state as we overwrote our config.
		return m.RefreshState(true)
	}

	if err := m.Node.SaveConfig(); err != nil {
		m.Message = fmt.Sprintf("failed to save config: %v", err)

		// Still refresh state as we overwrote our config.
		return m.RefreshState(true)
	}

	return m.RefreshState(true)
}

func (m *nodeInspect) DoRemoveKeys() tea.Cmd {
	m.UnsealKeys = nil
	return m.RefreshState(false)
}

func (m *nodeInspect) DoAddKey() tea.Cmd {
	input := textinput.New()
	input.Placeholder = "set unseal key..."
	m.UnsealKeys = append(m.UnsealKeys, &input)

	return m.RefreshState(false)
}

func (m *nodeInspect) SwitchInputs(inc int) tea.Cmd {
	selected := -1
	for index, field := range m.FieldOrdering {
		if field.Focused() {
			selected = index
			break
		}
	}

	if selected == -1 {
		selected = 0
	} else {
		m.FieldOrdering[selected].Blur()
		selected = (selected + inc + len(m.FieldOrdering)) % len(m.FieldOrdering)
	}

	return m.FieldOrdering[selected].Focus()
}

func (m *nodeInspect) MsgComponents(msg tea.Msg, focused bool) tea.Cmd {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	if !focused || m.ResumeStop.Focused() {
		m.ResumeStop, cmd = m.ResumeStop.Update(msg)
		cmds = append(cmds, cmd)
	}

	if !focused || m.SealUnseal.Focused() {
		m.SealUnseal, cmd = m.SealUnseal.Update(msg)
		cmds = append(cmds, cmd)
	}

	if !focused || m.Clean.Focused() {
		m.Clean, cmd = m.Clean.Update(msg)
		cmds = append(cmds, cmd)
	}

	for _, key := range m.UnsealKeys {
		if !focused || key.Focused() {
			var newKey textinput.Model
			newKey, cmd = key.Update(msg)
			*key = newKey
			cmds = append(cmds, cmd)
		}
	}

	if !focused || m.RemoveKeys.Focused() {
		m.RemoveKeys, cmd = m.RemoveKeys.Update(msg)
		cmds = append(cmds, cmd)
	}

	if !focused || m.AddKey.Focused() {
		m.AddKey, cmd = m.AddKey.Update(msg)
		cmds = append(cmds, cmd)
	}

	if !focused || m.Token.Focused() {
		var token textinput.Model
		token, cmd = m.Token.Update(msg)
		*m.Token = token
		cmds = append(cmds, cmd)
	}

	if !focused || m.Save.Focused() {
		m.Save, cmd = m.Save.Update(msg)
		cmds = append(cmds, cmd)
	}

	return tea.Batch(cmds...)
}

func (m *nodeInspect) UpdateTab(msg tea.Msg) (tabModel, tabModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height

		cmds = append(cmds, m.MsgComponents(msg, false))
	case tea.KeyMsg:
		switch msg.String() {
		case "up":
			if cmd := m.SwitchInputs(-1); cmd != nil {
				cmds = append(cmds, cmd)
			}
		case "k":
			if !m.InTextbox() {
				if cmd := m.SwitchInputs(-1); cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
		case "down":
			if cmd := m.SwitchInputs(1); cmd != nil {
				cmds = append(cmds, cmd)
			}
		case "j":
			if !m.InTextbox() {
				if cmd := m.SwitchInputs(1); cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
		}
	}

	cmds = append(cmds, m.MsgComponents(msg, true))

	changed := len(m.UnsealKeys) != len(m.Node.UnsealKeys)
	if !changed {
		for index, key := range m.UnsealKeys {
			if key.Value() != m.Node.UnsealKeys[index] {
				changed = true
				break
			}
		}

		changed = changed || (m.Token.Value() != m.Node.Token)
	}

	m.Save.Disabled = !changed

	return m, nil, tea.Batch(cmds...)
}

func (m *nodeInspect) View() string {
	var msg string
	if m.Message != "" {
		msg += warningStyle.Width(m.Width).Render("Message: "+m.Message) + "\n\n"
	}

	// Buttons
	msg += lipgloss.JoinHorizontal(lipgloss.Top, m.ResumeStop.View(), m.SealUnseal.View(), m.Clean.View())

	msg += "\n\nName:    " + m.NodeName

	nodeType := "prod"
	if m.Node.Config.Dev != nil {
		nodeType = "dev"
	}

	addr, ca, err := m.Node.GetConnectAddr()
	if err != nil {
		addr = fmt.Sprintf("[err: %v]", err)
	}

	status := "Stop"
	if m.Running {
		status = fmt.Sprintf("Running [pid: %d]", m.Node.Exec.Pid)
	}

	msg += "\nState:   " + status
	msg += "\nType:    " + nodeType
	msg += "\nAddress: " + addr

	if ca != "" {
		msg += "\nCA Certificate: " + ca
	}

	msg += "\n\nSeal Keys\n"
	if len(m.UnsealKeys) > 0 {
		for index, key := range m.UnsealKeys {
			msg += fmt.Sprintf("Key %d%v\n", index+1, key.View())
		}
	}
	msg += lipgloss.JoinHorizontal(lipgloss.Top, m.RemoveKeys.View(), m.AddKey.View())

	msg += "\n\nRoot Token\nToken" + m.Token.View()

	msg += "\n\n"
	msg += lipgloss.JoinHorizontal(lipgloss.Top, m.Save.View()) + "\n"
	if !m.Save.Disabled {
		msg += "(unsaved changes)\n"
	}

	return lipgloss.NewStyle().Align(lipgloss.Left).Render(msg)
}
