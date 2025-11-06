package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/openbao/devbao/pkg/bao"
)

type profileItem string

func (i profileItem) Title() string       { return string(i) }
func (i profileItem) Description() string { return bao.ProfileDescription(i.Title()) }
func (i profileItem) FilterValue() string { return string(i) }

type profiles struct {
	items []list.Item
	list  list.Model

	Width  int
	Height int
}

var _ tabModel = &profiles{}

func newProfilesListDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()

	d.ShowDescription = true

	help := []key.Binding{
		key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "execute"),
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

func ProfilesModel() tabModel {
	var items []list.Item
	for _, policy := range bao.ListProfiles() {
		items = append(items, profileItem(policy))
	}

	pList := list.New(items, newProfilesListDelegate(), 20, 20)
	pList.Title = "Profiles"
	pList.Styles.Title = titleStyle
	pList.DisableQuitKeybindings()

	return &profiles{
		items: items,
		list:  pList,
	}
}

func (m profiles) Name() string { return "Profiles" }

func (m profiles) InTextbox() bool { return m.list.FilterState() == list.Filtering }
func (m profiles) Closeable() bool { return false }

func (m *profiles) Init() tea.Cmd {
	return nil
}

func (m *profiles) UpdateTab(msg tea.Msg) (tabModel, tabModel, tea.Cmd) {
	var cmds []tea.Cmd
	var newTab tabModel

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.list.SetSize(msg.Width, msg.Height)
	case tea.KeyMsg:
		// Don't match any of the keys below if we're actively filtering.
		if m.list.FilterState() == list.Filtering {
			break
		}

		switch msg.String() {
		case "enter":
			// Spawn a new instance of this profile.
			selected := m.list.SelectedItem().(list.DefaultItem).Title()

			newTab = ProfileApplyModel(selected)
			if cmd := newTab.Init(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}

	var listCmd tea.Cmd = nil
	m.list, listCmd = m.list.Update(msg)
	if listCmd != nil {
		cmds = append(cmds, listCmd)
	}

	return m, newTab, tea.Batch(cmds...)
}

func (m *profiles) View() string {
	return m.list.View()
}
