package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/indiekitai/cron-health/internal/db"
	"github.com/indiekitai/cron-health/internal/monitor"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Interactive terminal UI dashboard",
	Long: `Launch an interactive terminal UI for monitoring cron jobs.

Keybindings:
  j/↓     Move down
  k/↑     Move up
  Enter   View monitor details
  a       Add new monitor
  d       Delete monitor
  r       Refresh list
  q/Esc   Quit`,
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open()
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}

		p := tea.NewProgram(initialModel(database), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			database.Close()
			return err
		}
		database.Close()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}

// View modes
type viewMode int

const (
	viewList viewMode = iota
	viewDetail
	viewAdd
	viewConfirmDelete
)

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86")).
			MarginBottom(1)

	statusOKStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42"))

	statusLateStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))

	statusDownStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	detailStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("86")).
			Padding(1, 2).
			MarginTop(1)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)
)

// Key bindings
type keyMap struct {
	Up       key.Binding
	Down     key.Binding
	Enter    key.Binding
	Add      key.Binding
	Delete   key.Binding
	Refresh  key.Binding
	Back     key.Binding
	Quit     key.Binding
	Confirm  key.Binding
	NextStep key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Enter, k.Add, k.Delete, k.Refresh, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Enter},
		{k.Add, k.Delete, k.Refresh, k.Quit},
	}
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "details"),
	),
	Add: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "add"),
	),
	Delete: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "delete"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc", "backspace"),
		key.WithHelp("esc", "back"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Confirm: key.NewBinding(
		key.WithKeys("y"),
		key.WithHelp("y", "yes"),
	),
	NextStep: key.NewBinding(
		key.WithKeys("tab", "enter"),
		key.WithHelp("tab/enter", "next"),
	),
}

// Messages
type tickMsg time.Time
type refreshMsg struct{}

// Model
type model struct {
	db       *db.DB
	monitors []*db.Monitor
	table    table.Model
	help     help.Model
	mode     viewMode
	selected *db.Monitor
	err      error

	// Add form
	addStep     int
	addName     textinput.Model
	addInterval textinput.Model
	addGrace    textinput.Model
	addCron     textinput.Model

	// Terminal size
	width  int
	height int
}

func initialModel(database *db.DB) model {
	// Initialize text inputs for add form
	nameInput := textinput.New()
	nameInput.Placeholder = "monitor-name"
	nameInput.CharLimit = 50

	intervalInput := textinput.New()
	intervalInput.Placeholder = "24h"
	intervalInput.CharLimit = 20

	graceInput := textinput.New()
	graceInput.Placeholder = "1h (optional)"
	graceInput.CharLimit = 20

	cronInput := textinput.New()
	cronInput.Placeholder = "0 2 * * * (optional, replaces interval)"
	cronInput.CharLimit = 50

	columns := []table.Column{
		{Title: "Name", Width: 20},
		{Title: "Status", Width: 10},
		{Title: "Interval/Cron", Width: 15},
		{Title: "Last Ping", Width: 20},
		{Title: "Next Expected", Width: 20},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	m := model{
		db:          database,
		table:       t,
		help:        help.New(),
		mode:        viewList,
		addName:     nameInput,
		addInterval: intervalInput,
		addGrace:    graceInput,
		addCron:     cronInput,
	}

	m.loadMonitors()
	return m
}

func (m *model) loadMonitors() {
	monitors, err := m.db.ListMonitors()
	if err != nil {
		m.err = err
		return
	}
	m.monitors = monitors

	rows := make([]table.Row, len(monitors))
	for i, mon := range monitors {
		status := monitor.CalculateStatus(mon)
		mon.Status = status

		var intervalStr string
		if mon.CronExpr != "" {
			intervalStr = mon.CronExpr
		} else {
			intervalStr = monitor.FormatDuration(time.Duration(mon.IntervalSeconds) * time.Second)
		}

		var lastPingStr string
		if mon.LastPing != nil {
			lastPingStr = monitor.TimeAgo(*mon.LastPing)
		} else {
			lastPingStr = "never"
		}

		var nextExpectedStr string
		if mon.NextExpected != nil {
			nextExpectedStr = monitor.TimeUntil(*mon.NextExpected)
		} else if mon.LastPing != nil {
			nextTime := mon.LastPing.Add(time.Duration(mon.IntervalSeconds) * time.Second)
			nextExpectedStr = monitor.TimeUntil(nextTime)
		} else {
			nextExpectedStr = "-"
		}

		rows[i] = table.Row{
			mon.Name,
			status,
			intervalStr,
			lastPingStr,
			nextExpectedStr,
		}
	}
	m.table.SetRows(rows)
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.SetHeight(msg.Height - 10)
		return m, nil

	case tickMsg:
		m.loadMonitors()
		return m, tickCmd()

	case refreshMsg:
		m.loadMonitors()
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case viewList:
			return m.updateList(msg)
		case viewDetail:
			return m.updateDetail(msg)
		case viewAdd:
			return m.updateAdd(msg)
		case viewConfirmDelete:
			return m.updateConfirmDelete(msg)
		}
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch {
	case key.Matches(msg, keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, keys.Enter):
		if len(m.monitors) > 0 {
			idx := m.table.Cursor()
			if idx < len(m.monitors) {
				m.selected = m.monitors[idx]
				m.mode = viewDetail
			}
		}
		return m, nil

	case key.Matches(msg, keys.Add):
		m.mode = viewAdd
		m.addStep = 0
		m.addName.Reset()
		m.addInterval.Reset()
		m.addGrace.Reset()
		m.addCron.Reset()
		m.addName.Focus()
		return m, nil

	case key.Matches(msg, keys.Delete):
		if len(m.monitors) > 0 {
			idx := m.table.Cursor()
			if idx < len(m.monitors) {
				m.selected = m.monitors[idx]
				m.mode = viewConfirmDelete
			}
		}
		return m, nil

	case key.Matches(msg, keys.Refresh):
		m.loadMonitors()
		return m, nil
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Back), key.Matches(msg, keys.Quit):
		m.mode = viewList
		m.selected = nil
	}
	return m, nil
}

func (m model) updateAdd(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch {
	case key.Matches(msg, keys.Back):
		m.mode = viewList
		return m, nil

	case key.Matches(msg, keys.NextStep):
		if m.addStep < 3 {
			m.addStep++
			m.addName.Blur()
			m.addInterval.Blur()
			m.addGrace.Blur()
			m.addCron.Blur()
			switch m.addStep {
			case 1:
				m.addInterval.Focus()
			case 2:
				m.addGrace.Focus()
			case 3:
				m.addCron.Focus()
			}
		} else {
			// Submit
			return m.submitAdd()
		}
		return m, nil
	}

	// Update the focused input
	switch m.addStep {
	case 0:
		m.addName, cmd = m.addName.Update(msg)
	case 1:
		m.addInterval, cmd = m.addInterval.Update(msg)
	case 2:
		m.addGrace, cmd = m.addGrace.Update(msg)
	case 3:
		m.addCron, cmd = m.addCron.Update(msg)
	}

	return m, cmd
}

func (m model) submitAdd() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.addName.Value())
	intervalStr := strings.TrimSpace(m.addInterval.Value())
	graceStr := strings.TrimSpace(m.addGrace.Value())
	cronStr := strings.TrimSpace(m.addCron.Value())

	if name == "" {
		m.err = fmt.Errorf("name is required")
		return m, nil
	}

	if intervalStr == "" && cronStr == "" {
		m.err = fmt.Errorf("interval or cron expression is required")
		return m, nil
	}

	var intervalSeconds int64 = 24 * 60 * 60 // default 24h
	var graceSeconds int64 = 0

	if intervalStr != "" {
		interval, err := monitor.ParseDuration(intervalStr)
		if err != nil {
			m.err = fmt.Errorf("invalid interval: %v", err)
			return m, nil
		}
		intervalSeconds = int64(interval.Seconds())
	}

	if graceStr != "" {
		grace, err := monitor.ParseDuration(graceStr)
		if err != nil {
			m.err = fmt.Errorf("invalid grace: %v", err)
			return m, nil
		}
		graceSeconds = int64(grace.Seconds())
	}

	// Create the monitor
	if cronStr != "" {
		// TODO: validate cron and calculate next expected
		_, err := m.db.CreateMonitorWithCron(name, intervalSeconds, graceSeconds, cronStr, nil)
		if err != nil {
			m.err = err
			return m, nil
		}
	} else {
		_, err := m.db.CreateMonitor(name, intervalSeconds, graceSeconds)
		if err != nil {
			m.err = err
			return m, nil
		}
	}

	m.mode = viewList
	m.err = nil
	m.loadMonitors()
	return m, nil
}

func (m model) updateConfirmDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Confirm):
		if m.selected != nil {
			m.db.DeleteMonitor(m.selected.Name)
			m.loadMonitors()
		}
		m.mode = viewList
		m.selected = nil

	case key.Matches(msg, keys.Back), key.Matches(msg, key.NewBinding(key.WithKeys("n"))):
		m.mode = viewList
		m.selected = nil
	}
	return m, nil
}

func (m model) View() string {
	var s strings.Builder

	switch m.mode {
	case viewList:
		s.WriteString(titleStyle.Render("🩺 cron-health Dashboard"))
		s.WriteString("\n\n")

		if len(m.monitors) == 0 {
			s.WriteString("No monitors configured. Press 'a' to add one.\n")
		} else {
			s.WriteString(m.table.View())
		}

		if m.err != nil {
			s.WriteString(fmt.Sprintf("\n\n❌ Error: %v", m.err))
		}

		s.WriteString("\n")
		s.WriteString(helpStyle.Render(m.help.ShortHelpView(keys.ShortHelp())))

	case viewDetail:
		s.WriteString(titleStyle.Render("📊 Monitor Details"))
		s.WriteString("\n")

		if m.selected != nil {
			mon := m.selected
			status := monitor.CalculateStatus(mon)

			details := fmt.Sprintf("Name:     %s\n", mon.Name)
			details += fmt.Sprintf("Status:   %s\n", formatStatusColored(status))

			if mon.CronExpr != "" {
				details += fmt.Sprintf("Cron:     %s\n", mon.CronExpr)
			} else {
				details += fmt.Sprintf("Interval: %s\n", monitor.FormatDuration(time.Duration(mon.IntervalSeconds)*time.Second))
			}

			if mon.GraceSeconds > 0 {
				details += fmt.Sprintf("Grace:    %s\n", monitor.FormatDuration(time.Duration(mon.GraceSeconds)*time.Second))
			}

			if mon.LastPing != nil {
				details += fmt.Sprintf("Last Ping: %s (%s)\n",
					mon.LastPing.Format("2006-01-02 15:04:05"),
					monitor.TimeAgo(*mon.LastPing))
			} else {
				details += "Last Ping: never\n"
			}

			if mon.NextExpected != nil {
				details += fmt.Sprintf("Next Expected: %s (%s)\n",
					mon.NextExpected.Format("2006-01-02 15:04:05"),
					monitor.TimeUntil(*mon.NextExpected))
			}

			details += fmt.Sprintf("Created:  %s\n", mon.CreatedAt.Format("2006-01-02 15:04:05"))

			s.WriteString(detailStyle.Render(details))
		}

		s.WriteString("\n\n")
		s.WriteString(helpStyle.Render("Press Esc or Backspace to go back"))

	case viewAdd:
		s.WriteString(titleStyle.Render("➕ Add Monitor"))
		s.WriteString("\n\n")

		s.WriteString(fmt.Sprintf("Name:     %s\n", m.addName.View()))
		s.WriteString(fmt.Sprintf("Interval: %s\n", m.addInterval.View()))
		s.WriteString(fmt.Sprintf("Grace:    %s\n", m.addGrace.View()))
		s.WriteString(fmt.Sprintf("Cron:     %s\n", m.addCron.View()))

		if m.err != nil {
			s.WriteString(fmt.Sprintf("\n❌ %v", m.err))
		}

		s.WriteString("\n\n")
		s.WriteString(helpStyle.Render("Tab/Enter: next field • Esc: cancel"))

	case viewConfirmDelete:
		s.WriteString(titleStyle.Render("🗑️ Delete Monitor"))
		s.WriteString("\n\n")

		if m.selected != nil {
			s.WriteString(fmt.Sprintf("Are you sure you want to delete '%s'?\n\n", m.selected.Name))
			s.WriteString("Press 'y' to confirm or 'n'/Esc to cancel")
		}
	}

	return s.String()
}

func formatStatusColored(status string) string {
	switch status {
	case "OK":
		return statusOKStyle.Render("● OK")
	case "LATE":
		return statusLateStyle.Render("● LATE")
	case "DOWN":
		return statusDownStyle.Render("● DOWN")
	default:
		return status
	}
}
