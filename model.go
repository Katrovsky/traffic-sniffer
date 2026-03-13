package main

import (
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

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
	Domain string
	Count  int
	Last   time.Time
}

const (
	colWidthIP         = 18
	colWidthPort       = 6
	colWidthHits       = 5
	colWidthLastSeen   = 10
	tableHorizOverhead = 12
	tableVertOverhead  = 8
	logColIP           = 20
	logColPort         = 6
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

	logPath      string
	loggedKeys   map[string]bool
	logFile      *os.File
	maxDomainLen int

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
		phase:        phaseSelect,
		lst:          newAppList(getApps(), 80, 20),
		tbl:          t,
		loggedKeys:   map[string]bool{},
		startTime:    time.Now(),
		maxDomainLen: 6,
		width:        80,
		height:       24,
	}
}

func makeColumns(termWidth int) []table.Column {
	fixed := colWidthIP + colWidthPort + colWidthHits + colWidthLastSeen
	domainWidth := max(termWidth-tableHorizOverhead-fixed, 15)
	return []table.Column{
		{Title: "IP", Width: colWidthIP},
		{Title: "Port", Width: colWidthPort},
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

func (m *model) openLog(app string) {
	ts := time.Now().Format("20060102_150405")
	m.logPath = fmt.Sprintf("nettracer_%s_%s.log", app, ts)
	f, err := os.Create(m.logPath)
	if err != nil {
		return
	}
	m.logFile = f
}

func (m *model) rewriteLogHeader() {
	if m.logFile == nil {
		return
	}
	lineLen := logColIP + 1 + logColPort + 1 + m.maxDomainLen + 1 + 4 + 1 + 8
	header := fmt.Sprintf("%-*s %-*s %-*s %-4s  %s\n",
		logColIP, "IP",
		logColPort, "Port",
		m.maxDomainLen, "Domain",
		"Hits",
		"First seen",
	)
	separator := strings.Repeat("─", lineLen) + "\n"
	m.logFile.Seek(0, 0)
	m.logFile.WriteString(header)
	m.logFile.WriteString(separator)
}

func (m *model) flushLog() {
	if m.logFile == nil || m.tracer == nil {
		return
	}
	m.tracer.Mux.RLock()
	defer m.tracer.Mux.RUnlock()

	headerDirty := false
	for key, c := range m.tracer.Conns {
		if c.Domain == "resolving..." || m.loggedKeys[key] {
			continue
		}
		if n := utf8.RuneCountInString(c.Domain); n > m.maxDomainLen {
			m.maxDomainLen = n
			headerDirty = true
		}
	}
	if headerDirty {
		m.rewriteLogHeader()
	}

	for key, c := range m.tracer.Conns {
		if c.Domain == "resolving..." || m.loggedKeys[key] {
			continue
		}
		fmt.Fprintf(m.logFile, "%-*s %-*s %-*s %-4d  %s\n",
			logColIP, c.IP,
			logColPort, c.Port,
			m.maxDomainLen, c.Domain,
			c.Count,
			c.FirstSeen.Format("15:04:05"),
		)
		m.loggedKeys[key] = true
	}
}

func (m *model) closeLog() {
	m.flushLog()
	if m.logFile != nil {
		m.logFile.Close()
		m.logFile = nil
	}
}

func (m *model) resetForSelect() {
	m.closeLog()
	m.tracer = nil
	m.conns = nil
	m.totalConns = 0
	m.loggedKeys = map[string]bool{}
	m.maxDomainLen = 6
	m.startTime = time.Now()
	m.lst = newAppList(getApps(), m.width, m.height-2)
	m.phase = phaseSelect
}
