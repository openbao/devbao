package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/openbao/devbao/pkg/bao"
)

type nodeItem struct {
	Name    string
	Node    *bao.Node
	Running bool
}

func (i nodeItem) Title() string {
	return i.Name
}

func (i nodeItem) Description() string {
	mode := ""
	if i.Node.Config.Dev != nil {
		mode = "[dev-mode] "
	}

	addr, ca, err := i.Node.GetConnectAddr()
	if err != nil {
		addr = fmt.Sprintf("[err: %v]", err)
	}

	if addr != "" {
		addr = "\nVAULT_ADDR=" + addr
	}

	token := i.Node.Token
	if token != "" {
		token = "\nVAULT_TOKEN=" + token
	}

	if ca != "" {
		ca = "\nVAULT_CACERT=" + ca
	}

	desc := fmt.Sprintf("%v%v%v%v", mode, addr, token, ca)

	if !i.Running {
		desc = "[stopped] " + desc
	} else {
		desc = fmt.Sprintf("[pid: %d] %v", i.Node.Exec.Pid, desc)
	}

	return desc
}

func (i nodeItem) FilterValue() string {
	return i.Name
}

type nodeListMsg struct {
	Items []*nodeItem
}

var _ tea.Msg = &nodeListMsg{}

type nodeListErrorMsg struct {
	Error error
}

var _ tea.Msg = &nodeListErrorMsg{}

type nodes struct {
	items []list.Item
	list  *list.Model

	Display string

	Width  int
	Height int
}

var _ tabModel = &nodes{}

func newNodesListDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()

	d.ShowDescription = true
	d.SetHeight(4)

	help := []key.Binding{
		key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "inspect"),
		),
		key.NewBinding(
			key.WithKeys("s", "+"),
			key.WithHelp("+/s", "start a new node"),
		),
	}

	d.ShortHelpFunc = func() []key.Binding {
		return help[:]
	}

	d.FullHelpFunc = func() [][]key.Binding {
		return [][]key.Binding{
			help[:],
		}
	}

	return d
}

func NodesModel() tabModel {
	return &nodes{
		Display: "Loading nodes...",
	}
}

func (m nodes) Name() string    { return "Nodes" }
func (m nodes) InTextbox() bool { return m.list.FilterState() == list.Filtering }
func (m nodes) Closeable() bool { return false }

func (m *nodes) Init() tea.Cmd {
	return m.DoRefresh(time.Nanosecond)
}

func (m *nodes) DoRefresh(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		nodes, err := bao.ListNodes()
		if err != nil {
			return &nodeListErrorMsg{err}
		}

		var items []*nodeItem
		for index, name := range nodes {
			node, err := bao.LoadNode(name)
			if err != nil {
				return &nodeListErrorMsg{fmt.Errorf("failed to load node [%v/%v]: %w", index, name, err)}
			}

			running := true
			if err := node.Exec.ValidateRunning(); err != nil {
				running = false
			}

			item := &nodeItem{
				Name:    name,
				Node:    node,
				Running: running,
			}

			items = append(items, item)
		}

		return &nodeListMsg{items}
	})
}

func (m *nodes) UpdateTab(msg tea.Msg) (tabModel, tabModel, tea.Cmd) {
	var cmds []tea.Cmd
	var newTab tabModel

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		if m.list != nil {
			m.list.SetSize(msg.Width, msg.Height)
		}
	case tea.KeyMsg:
		// Don't match any of the keys below if we're actively filtering.
		if m.list != nil && m.list.FilterState() == list.Filtering {
			break
		}

		switch msg.String() {
		case "enter":
			// View the selected node.
			selected := m.list.SelectedItem().(*nodeItem)

			newTab = NodeInspectModel(selected.Name, selected.Node, selected.Running)
			if cmd := newTab.Init(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		case "+", "s", "a":
			// Create a new node.
			newTab = NodeStartModel()
			if cmd := newTab.Init(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	case *nodeListMsg:
		if m.list == nil {
			var items []list.Item
			for _, item := range msg.Items {
				items = append(items, item)
			}

			nList := list.New(items, newNodesListDelegate(), m.Width, m.Height)
			nList.Title = "Nodes"
			nList.Styles.Title = titleStyle
			nList.DisableQuitKeybindings()

			m.list = &nList
		} else {
			var items []list.Item
			for _, item := range msg.Items {
				items = append(items, item)
			}

			cmds = append(cmds, m.list.SetItems(items))
		}

		cmds = append(cmds, m.DoRefresh(time.Second))
	case *nodeListErrorMsg:
		m.list = nil
		m.Display = msg.Error.Error()

		cmds = append(cmds, m.DoRefresh(time.Second))
	case TabFocusMsg:
		cmds = append(cmds, m.DoRefresh(time.Nanosecond))
	}

	if m.list != nil {
		var newList list.Model
		var listCmd tea.Cmd = nil
		newList, listCmd = m.list.Update(msg)
		if listCmd != nil {
			cmds = append(cmds, listCmd)
		}

		m.list = &newList
	}

	return m, newTab, tea.Batch(cmds...)
}

func (m *nodes) View() string {
	if m.list != nil {
		return m.list.View()
	}

	return m.Display
}
