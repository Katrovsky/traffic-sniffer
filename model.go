package main

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type phase int

const (
	phaseSelect phase = iota
	phaseMonitor
)

type tickMsg time.Time
type connUpdateMsg map[string]*Conn

type connRow struct {
	key    string
	IP     string
	Port   string
	Proto  string
	Domain string
	Count  int
	Last   time.Time
}

const (
	colWidthIP         = 18
	colWidthPort       = 6
	colWidthProto      = 5
	colWidthHits       = 5
	colWidthLastSeen   = 10
	tableHorizOverhead = 14
	tableVertOverhead  = 8
)

type appItem string

func (a appItem) FilterValue() string { return string(a) }
func (a appItem) Title() string       { return string(a) }
func (a appItem) Description() string { return "" }

func newAppList(apps []string, width, height int) list.Model {
	items := make([]list.Item, len(apps))
	for i, a := range apps {
		items[i] = appItem(a)
	}

	d := list.NewDefaultDelegate()
	d.ShowDescription = false
	d.SetSpacing(0)
	d.Styles.SelectedTitle = d.Styles.SelectedTitle.
		Foreground(lipgloss.Color("#FFFFFF")).
		BorderLeftForeground(lipgloss.Color("#7C3AED"))
	d.Styles.NormalTitle = d.Styles.NormalTitle.
		Foreground(lipgloss.Color("#E2E8F0"))

	l := list.New(items, d, width, height)
	l.Title = "Select application"
	l.Styles.Title = titleStyle
	l.Styles.FilterPrompt = dimStyle
	l.Styles.FilterCursor = lipgloss.NewStyle().Foreground(lipgloss.Color("#7C3AED"))
	l.DisableQuitKeybindings()
	return l
}

type model struct {
	phase phase
	lst   list.Model

	tracer     *Tracer
	conns      []connRow
	tbl        table.Model
	sortBy     int
	totalConns int

	log *logger

	startTime time.Time
	width     int
	height    int
}

func initialModel() model {
	cols := makeColumns(80)
	t := table.New(
		table.WithColumns(cols),
		table.WithFocused(true),
		table.WithHeight(20),
	)
	t.SetStyles(tableStyles())

	return model{
		phase:     phaseSelect,
		lst:       newAppList(getApps(), 80, 20),
		tbl:       t,
		startTime: time.Now(),
		width:     80,
		height:    24,
	}
}

func makeColumns(termWidth int) []table.Column {
	fixed := colWidthIP + colWidthPort + colWidthProto + colWidthHits + colWidthLastSeen
	domainWidth := max(termWidth-tableHorizOverhead-fixed, 15)
	return []table.Column{
		{Title: "IP", Width: colWidthIP},
		{Title: "Port", Width: colWidthPort},
		{Title: "Proto", Width: colWidthProto},
		{Title: "Domain", Width: domainWidth},
		{Title: "Hits", Width: colWidthHits},
		{Title: "Last", Width: colWidthLastSeen},
	}
}

func tableStyles() table.Styles {
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#4C1D95")).
		BorderBottom(true).
		Bold(true).
		Foreground(lipgloss.Color("#A78BFA"))
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#5B21B6")).
		Bold(false)
	return s
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if sz, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = sz.Width
		m.height = sz.Height
		m.lst.SetSize(m.width, m.height-2)
		if m.phase == phaseMonitor {
			m.tbl.SetColumns(makeColumns(m.width))
			m.tbl.SetHeight(m.height - tableVertOverhead)
		}
		return m, nil
	}
	switch m.phase {
	case phaseSelect:
		return m.updateSelect(msg)
	case phaseMonitor:
		return m.updateMonitor(msg)
	}
	return m, nil
}

func (m model) View() string {
	switch m.phase {
	case phaseSelect:
		return m.viewSelect()
	case phaseMonitor:
		return m.viewMonitor()
	}
	return ""
}

func (m *model) resetForSelect() {
	if m.log != nil {
		m.log.close(m.tracer)
		m.log = nil
	}
	m.tracer = nil
	m.conns = nil
	m.totalConns = 0
	m.startTime = time.Now()
	m.lst = newAppList(getApps(), m.width, m.height-2)
	m.phase = phaseSelect
}

func (m *model) logPath() string {
	if m.log == nil {
		return ""
	}
	return m.log.path
}

func (m *model) statusLine() string {
	if m.tracer == nil {
		return ""
	}
	elapsed := time.Since(m.startTime).Round(time.Second)
	pidCount := m.tracer.PIDCount()

	pidInfo := fmt.Sprintf("%d PID(s)", pidCount)
	if pidCount == 0 {
		pidInfo = warnStyle.Render("no PIDs found")
	}

	return fmt.Sprintf("  %s connections  •  %s  •  %s  •  log: %s",
		countStyle.Render(fmt.Sprintf("%d", m.totalConns)),
		dimStyle.Render(elapsed.String()),
		dimStyle.Render(pidInfo),
		dimStyle.Render(m.logPath()),
	)
}
