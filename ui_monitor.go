package main

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

var sortLabels = []string{"hits", "ip", "port", "domain"}

func tickCmd() tea.Cmd {
	return tea.Tick(1500*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func scanCmd(tr *Tracer) tea.Cmd {
	return func() tea.Msg {
		tr.scan()
		return connUpdateMsg(tr.snapshot())
	}
}

func (m model) updateMonitor(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			if m.log != nil {
				m.log.close(m.tracer)
			}
			return m, tea.Interrupt
		case "q":
			if m.log != nil {
				m.log.close(m.tracer)
			}
			return m, tea.Quit
		case "esc", "b":
			m.resetForSelect()
			return m, nil
		case "r":
			m.tracer.resetCounters()
			m.conns = nil
			m.tbl.SetRows(nil)
		case "s":
			m.sortBy = (m.sortBy + 1) % 4
			if len(m.conns) > 0 {
				sortRows(m.conns, m.sortBy)
				m.tbl.SetRows(toTableRows(m.conns))
			}
		}
		m.tbl, cmd = m.tbl.Update(msg)

	case tickMsg:
		return m, tea.Batch(tickCmd(), scanCmd(m.tracer))

	case connUpdateMsg:
		m.totalConns = len(msg)
		m.conns = sortedRows(msg, m.sortBy)
		m.tbl.SetRows(toTableRows(m.conns))
		if m.log != nil {
			m.log.flush(m.tracer)
		}
	}
	return m, cmd
}

func sortedRows(msg connUpdateMsg, sortBy int) []connRow {
	rows := make([]connRow, 0, len(msg))
	for k, c := range msg {
		rows = append(rows, connRow{
			key: k, IP: c.IP, Port: c.Port, Proto: c.Proto,
			Domain: c.Domain, Count: c.Count, Last: c.LastSeen,
		})
	}
	sortRows(rows, sortBy)
	return rows
}

func sortRows(rows []connRow, sortBy int) {
	switch sortBy {
	case 0:
		slices.SortFunc(rows, func(a, b connRow) int { return cmp.Compare(b.Count, a.Count) })
	case 1:
		slices.SortFunc(rows, func(a, b connRow) int { return cmp.Compare(a.IP, b.IP) })
	case 2:
		slices.SortFunc(rows, func(a, b connRow) int {
			pa, _ := strconv.Atoi(a.Port)
			pb, _ := strconv.Atoi(b.Port)
			return cmp.Compare(pa, pb)
		})
	case 3:
		slices.SortFunc(rows, func(a, b connRow) int { return cmp.Compare(a.Domain, b.Domain) })
	}
}

func toTableRows(rows []connRow) []table.Row {
	out := make([]table.Row, 0, len(rows))
	for _, r := range rows {
		domain := r.Domain
		if domain == "resolving..." {
			domain = "⟳ resolving..."
		}
		out = append(out, table.Row{
			r.IP, r.Port, r.Proto, domain,
			strconv.Itoa(r.Count),
			r.Last.Format("15:04:05"),
		})
	}
	return out
}

func (m model) viewMonitor() string {
	if m.tracer == nil {
		return ""
	}
	pidCount := m.tracer.PIDCount()

	header := titleStyle.Render("● NetTracer") +
		"  " + headerStyle.Render(m.tracer.AppName) +
		"  " + dimStyle.Render("sort: "+sortLabels[m.sortBy])

	if pidCount == 0 {
		header += "  " + warnStyle.Render("⚠ no PIDs found — is the app running?")
	}

	var tableContent string
	if m.totalConns == 0 {
		empty := dimStyle.Render("no connections yet — waiting for traffic…")
		tableContent = borderStyle.Width(m.width - 2).Render(
			m.tbl.View() + "\n" +
				strings.Repeat(" ", (m.width/2)-20) + empty,
		)
	} else {
		tableContent = borderStyle.Width(m.width - 2).Render(m.tbl.View())
	}

	helpText := "  ↑/↓ navigate  •  s sort  •  r reset hits  •  b/Esc back  •  q quit  "
	help := statusBarStyle.Width(m.width).Render(helpText)

	return strings.Join([]string{
		header,
		m.statusLine(),
		"",
		tableContent,
		help,
	}, "\n")
}
