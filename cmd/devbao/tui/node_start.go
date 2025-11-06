package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/openbao/devbao/pkg/bao"
)

type nodeStart struct {
	Type     *Radio
	NodeName *textinput.Model
	Force    *Checkbox

	Create   *Button
	Profiles []*Checkbox

	DevToken   *textinput.Model
	DevAddress *textinput.Model

	ProdListenerTypes  []*Radio
	ProdListenerValues []*textinput.Model
	ProdStorageType    *Radio
	ProdInitialize     *Checkbox
	ProdUnseal         *Checkbox

	FieldOrdering []Focusable

	Message string

	Width  int
	Height int
}

var _ tabModel = &nodeStart{}

func NodeStartModel() tabModel {
	namespace := "node-add"

	model := &nodeStart{
		Type:  NewRadio(namespace, "Type", "prod nodes support persistent storage", []string{"dev", "prod"}),
		Force: NewCheckbox(namespace, "Force", "force creation of node if it already exists"),

		Create: NewButton(namespace, "Create", nil),
	}

	model.Type.Action = model.RefreshState

	name := textinput.New()
	name.Placeholder = "set name..."
	model.NodeName = &name

	devToken := textinput.New()
	devToken.SetValue("devroot")
	devToken.Placeholder = "set dev-root-token-id..."
	model.DevToken = &devToken

	devAddress := textinput.New()
	devAddress.SetValue("127.0.0.1:8200")
	devAddress.Placeholder = "set dev-listen-address..."
	model.DevAddress = &devAddress

	prodListenerValue := textinput.New()
	prodListenerValue.SetValue("0.0.0.0:8200")
	prodListenerValue.Placeholder = "set bind address..."
	model.ProdListenerValues = append(model.ProdListenerValues, &prodListenerValue)
	model.ProdListenerTypes = append(model.ProdListenerTypes, NewRadio(namespace, "Listen 0 Type", "listener socket protocol", []string{"tcp", "unix"}))

	model.ProdStorageType = NewRadio(namespace, "Storage Type", "storage backend", []string{"raft", "file", "inmem"})
	model.ProdInitialize = NewCheckbox(namespace, "Initialize", "")
	model.ProdUnseal = NewCheckbox(namespace, "Unseal", "")

	model.ProdInitialize.Action = model.RefreshState
	model.ProdUnseal.Action = model.RefreshState

	for _, profiles := range bao.ListProfiles() {
		desc := bao.ProfileDescription(profiles)
		input := NewCheckbox(namespace, profiles, desc)
		input.Action = model.RefreshState
		model.Profiles = append(model.Profiles, input)
	}

	model.Create.Action = func() tea.Cmd { return model.DoCreate() }

	model.Type.Focus()

	return model
}

func (m nodeStart) Name() string { return "Start Node" }

func (m nodeStart) InTextbox() bool {
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

func (m nodeStart) Closeable() bool {
	if m.InTextbox() {
		return false
	}

	return true
}

func (m *nodeStart) Init() tea.Cmd {
	return m.RefreshState()
}

func (m *nodeStart) RefreshState() tea.Cmd {
	m.Message = ""

	m.FieldOrdering = []Focusable{
		m.Type,
		m.NodeName,
	}

	if m.Type.GetValue() == "dev" {
		m.FieldOrdering = append(m.FieldOrdering, []Focusable{
			m.DevToken,
			m.DevAddress,
			m.Force,
		}...)

		m.Force.Desc = "force creation of node if it already exists"
	} else {
		for index, listenerType := range m.ProdListenerTypes {
			listenerValue := m.ProdListenerValues[index]
			m.FieldOrdering = append(m.FieldOrdering, listenerType)
			m.FieldOrdering = append(m.FieldOrdering, listenerValue)
		}

		m.FieldOrdering = append(m.FieldOrdering, []Focusable{
			m.ProdStorageType,
			m.ProdInitialize,
			m.ProdUnseal,
			m.Force,
		}...)

		m.Force.Desc = ""
	}

	for _, profiles := range m.Profiles {
		m.FieldOrdering = append(m.FieldOrdering, profiles)
	}

	m.FieldOrdering = append(m.FieldOrdering, []Focusable{
		m.Create,
	}...)

	if m.Type.GetValue() == "prod" {
		doingProfile := false
		for _, profiles := range m.Profiles {
			if profiles.Value {
				doingProfile = true
				break
			}
		}

		if doingProfile && (!m.ProdUnseal.Value || !m.ProdInitialize.Value) {
			m.ProdUnseal.Value = true
			m.ProdInitialize.Value = true
			m.Message = "starting with profiles requires initialization & unseal"
		}

		if m.ProdUnseal.Value && !m.ProdInitialize.Value {
			m.ProdInitialize.Value = true
			m.Message = "unsealing requires initialization"
		}

		if m.Message == "" {
			for index, listenerValue := range m.ProdListenerValues {
				if listenerValue.Value() == "" {
					m.Message = fmt.Sprintf("listener %d lacks a value", index)
					break
				}
			}
		}
	}

	m.Create.Disabled = len(m.NodeName.Value()) == 0

	return nil
}

func (m *nodeStart) DoCreate() tea.Cmd {
	m.RefreshState()

	if !m.Force.Value {
		present, err := bao.NodeExists(m.NodeName.Value())
		if err != nil {
			m.Message = fmt.Sprintf("error checking if node exists: %v", err)
			return nil
		}

		if present {
			m.Message = fmt.Sprintf("node with this name exists")
			return nil
		}
	}

	var opts []bao.NodeConfigOpt
	if m.Type.GetValue() == "dev" {
		opts = append(opts, &bao.DevConfig{
			Token:   m.DevToken.Value(),
			Address: m.DevAddress.Value(),
		})
	} else {
		for index, listenerType := range m.ProdListenerTypes {
			listenerValue := m.ProdListenerValues[index]
			if listenerValue.Value() == "" {
				m.Message = fmt.Sprintf("missing required listener at index %d", index)
				return nil
			}

			if listenerType.GetValue() == "tcp" {
				opts = append(opts, &bao.TCPListener{
					Address: listenerValue.Value(),
				})
			} else {
				opts = append(opts, &bao.UnixListener{
					Path: listenerValue.Value(),
				})
			}
		}

		switch m.ProdStorageType.GetValue() {
		case "", "raft":
			opts = append(opts, &bao.RaftStorage{})
		case "file":
			opts = append(opts, &bao.FileStorage{})
		case "inmem":
			opts = append(opts, &bao.InmemStorage{})
		default:
			m.Message = "unknown storage type: " + m.ProdStorageType.GetValue()
			return nil
		}
	}

	node, err := bao.BuildNode(m.NodeName.Value(), "", opts...)
	if err != nil {
		m.Message = fmt.Sprintf("failed to build node: %w", err)
		return nil
	}

	if err := node.Start(); err != nil {
		m.Message = fmt.Sprintf("failed to start node: %w", err)
		return nil
	}

	if m.Type.GetValue() == "prod" {
		if m.ProdInitialize.Value {
			if err := node.Initialize(); err != nil {
				m.Message = fmt.Sprintf("failed to initialize node: %b", err)
				return nil
			}

			if m.ProdUnseal.Value {
				if _, err := node.Unseal(); err != nil {
					m.Message = fmt.Sprintf("failed to unseal node: %v", err)
				}

				// TODO: use a client request with proper back-off to determine
				// when the node is responding.
				time.Sleep(500 * time.Millisecond)
			}
		}
	}

	client, err := node.GetClient()
	if err != nil {
		m.Message = fmt.Sprintf("failed to get client for node %v: %v", node.Name, err)
		return nil
	}

	for _, profile := range m.Profiles {
		if !profile.Value {
			continue
		}

		warnings, err := bao.ProfileSetup(client, profile.Title)
		if len(warnings) > 0 || err != nil {
			m.Message += fmt.Sprintf("for profile %v:\n", profile.Title)
		}

		for index, warning := range warnings {
			m.Message += fmt.Sprintf("\n - [warning %d]: %v\n", index, warning)
		}

		if err != nil {
			m.Message += "\nerror: " + err.Error()
		}

		if m.Message != "" {
			m.Message += "\n"
		}
	}

	newPage := NodeInspectModel(node.Name, node, true)
	newPage.(*nodeInspect).Message = m.Message

	return tea.Batch(TabReplaceCmd(m, newPage))
}

func (m *nodeStart) SwitchInputs(inc int) tea.Cmd {
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

func (m *nodeStart) MsgComponents(msg tea.Msg, focused bool) tea.Cmd {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	if !focused || m.Type.Focused() {
		m.Type, cmd = m.Type.Update(msg)
		cmds = append(cmds, cmd)
	}

	if !focused || m.NodeName.Focused() {
		var name textinput.Model
		name, cmd = m.NodeName.Update(msg)
		*m.NodeName = name
		cmds = append(cmds, cmd)
	}

	if !focused || m.Force.Focused() {
		m.Force, cmd = m.Force.Update(msg)
		cmds = append(cmds, cmd)
	}

	if m.Type.GetValue() == "dev" {
		if !focused || m.DevToken.Focused() {
			var token textinput.Model
			token, cmd = m.DevToken.Update(msg)
			*m.DevToken = token
			cmds = append(cmds, cmd)
		}

		if !focused || m.DevAddress.Focused() {
			var addr textinput.Model
			addr, cmd = m.DevAddress.Update(msg)
			*m.DevAddress = addr
			cmds = append(cmds, cmd)
		}
	} else {
		for index, listenerType := range m.ProdListenerTypes {
			listenerValue := m.ProdListenerValues[index]

			if !focused || listenerType.Focused() {
				listenerType, cmd = listenerType.Update(msg)
				cmds = append(cmds, cmd)
			}

			if !focused || listenerValue.Focused() {
				var value textinput.Model
				value, cmd = listenerValue.Update(msg)
				*m.ProdListenerValues[index] = value
				cmds = append(cmds, cmd)
			}
		}

		if !focused || m.ProdStorageType.Focused() {
			m.ProdStorageType, cmd = m.ProdStorageType.Update(msg)
			cmds = append(cmds, cmd)
		}

		if !focused || m.ProdInitialize.Focused() {
			m.ProdInitialize, cmd = m.ProdInitialize.Update(msg)
			cmds = append(cmds, cmd)
		}

		if !focused || m.ProdUnseal.Focused() {
			m.ProdUnseal, cmd = m.ProdUnseal.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	for index, profile := range m.Profiles {
		if !focused || profile.Focused() {
			m.Profiles[index], cmd = profile.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	if !focused || m.Create.Focused() {
		m.Create, cmd = m.Create.Update(msg)
		cmds = append(cmds, cmd)
	}

	return tea.Batch(cmds...)
}

func (m *nodeStart) UpdateTab(msg tea.Msg) (tabModel, tabModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height

		cmds = append(cmds, m.MsgComponents(msg, false))
	case tea.KeyMsg:
		wasInTextbox := m.InTextbox()

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

		if wasInTextbox || m.InTextbox() {
			cmds = append(cmds, m.RefreshState())
		}
	}

	cmds = append(cmds, m.MsgComponents(msg, true))

	return m, nil, tea.Batch(cmds...)
}

func (m *nodeStart) View() string {
	var msg string
	if m.Message != "" {
		msg += warningStyle.Width(m.Width).Render("Message: "+m.Message) + "\n\n"
	}

	msg += m.Type.View() + "\n\n"

	msg += "Name" + m.NodeName.View() + "\n\n"

	if m.Type.GetValue() == "dev" {
		msg += "Root Token:\nToken" + m.DevToken.View() + "\n\n"
		msg += "Listen Address:\nAddr" + m.DevAddress.View() + "\n\n"

		msg += m.Force.View() + "\n\n"
	} else {
		msg += "Listeners:\n"
		for index, listenerType := range m.ProdListenerTypes {
			listenerValue := m.ProdListenerValues[index]
			msg += listenerType.View() + "\n"
			msg += "Address" + listenerValue.View() + "\n\n"
		}

		msg += m.ProdStorageType.View() + "\n"
		dsc := ""
		if m.ProdInitialize.Focused() {
			dsc = "auto-initialize the instance to get unseal keys and root token"
		} else if m.ProdUnseal.Focused() {
			dsc = "auto-unseal the instance to begin using immediately"
		} else if m.Force.Focused() {
			dsc = "force creation of node if it already exists"
		}

		msg += lipgloss.JoinHorizontal(lipgloss.Top, m.ProdInitialize.View(), m.ProdUnseal.View(), m.Force.View()) + "\n" + dsc + "\n\n"
	}

	msg += "Profiles:\n"
	for _, profile := range m.Profiles {
		msg += profile.View() + "\n"
	}

	msg += "\n" + m.Create.View() + "\n"

	return lipgloss.NewStyle().Align(lipgloss.Left).Render(msg)
}
