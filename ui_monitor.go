package main

import (
	"cmp"
	"fmt"
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
		tr.Mux.RLock()
		cp := make(map[string]*Conn, len(tr.Conns))
		for k, v := range tr.Conns {
			c := *v
			cp[k] = &c
		}
		tr.Mux.RUnlock()
		return connUpdateMsg(cp)
	}
}

func (m model) updateMonitor(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.closeLog()
			return m, tea.Interrupt
		case "q":
			m.closeLog()
			return m, tea.Quit
		case "esc", "b":
			m.resetForSelect()
			return m, nil
		case "r":
			m.tracer.resetCounters()
			m.conns = nil
		case "s":
			m.sortBy = (m.sortBy + 1) % 4
		}
		m.tbl, cmd = m.tbl.Update(msg)

	case tickMsg:
		return m, tea.Batch(tickCmd(), scanCmd(m.tracer))

	case connUpdateMsg:
		m.totalConns = len(msg)
		m.conns = sortedRows(msg, m.sortBy)
		m.tbl.SetRows(toTableRows(m.conns))
		m.flushLog()
	}
	return m, cmd
}

func sortedRows(msg connUpdateMsg, sortBy int) []connRow {
	rows := make([]connRow, 0, len(msg))
	for k, c := range msg {
		rows = append(rows, connRow{
			key: k, IP: c.IP, Port: c.Port,
			Domain: c.Domain, Count: c.Count, Last: c.LastSeen,
		})
	}
	switch sortBy {
	case 0:
		slices.SortFunc(rows, func(a, b connRow) int { return cmp.Compare(b.Count, a.Count) })
	case 1:
		slices.SortFunc(rows, func(a, b connRow) int { return cmp.Compare(a.IP, b.IP) })
	case 2:
		slices.SortFunc(rows, func(a, b connRow) int { return cmp.Compare(a.Port, b.Port) })
	case 3:
		slices.SortFunc(rows, func(a, b connRow) int { return cmp.Compare(a.Domain, b.Domain) })
	}
	return rows
}

func toTableRows(rows []connRow) []table.Row {
	out := make([]table.Row, 0, len(rows))
	for _, r := range rows {
		domain := r.Domain
		if domain == "resolving..." {
			domain = "⟳ resolving..."
		}
		out = append(out, table.Row{
			r.IP, r.Port, domain,
			strconv.Itoa(r.Count),
			r.Last.Format("15:04:05"),
		})
	}
	return out
}

func (m model) viewMonitor() string {
	elapsed := time.Since(m.startTime).Round(time.Second)

	header := titleStyle.Render("● NetTracer") +
		"  " + headerStyle.Render(m.tracer.AppName) +
		"  " + dimStyle.Render("sort: "+sortLabels[m.sortBy])

	stats := fmt.Sprintf("  %s connections  •  %s  •  log: %s",
		countStyle.Render(strconv.Itoa(m.totalConns)),
		dimStyle.Render(elapsed.String()),
		dimStyle.Render(m.logPath),
	)

	helpText := "  ↑/↓ navigate  •  s sort  •  r reset hits  •  b/Esc back  •  q quit  "
	help := statusBarStyle.Width(m.width).Render(helpText)

	tableContent := borderStyle.Width(m.width - 2).Render(m.tbl.View())

	return strings.Join([]string{header, stats, "", tableContent, help}, "\n")
}
