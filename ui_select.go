package main

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

func (m model) updateSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "ctrl+c":
			return m, tea.Interrupt
		case "q", "esc":
			if m.lst.FilterState() == list.Filtering {
				break
			}
			return m, tea.Quit
		case "enter", " ":
			if m.lst.FilterState() == list.Filtering {
				break
			}
			item, ok := m.lst.SelectedItem().(appItem)
			if !ok {
				return m, nil
			}
			chosen := string(item)
			m.phase = phaseMonitor
			m.tracer = newTracer(chosen)
			m.tbl.SetColumns(makeColumns(m.width))
			m.tbl.SetHeight(m.height - tableVertOverhead)
			m.openLog(chosen)
			m.rewriteLogHeader()
			return m, tea.Batch(tickCmd(), scanCmd(m.tracer))
		}
	}

	var cmd tea.Cmd
	m.lst, cmd = m.lst.Update(msg)
	return m, cmd
}

func (m model) viewSelect() string {
	return m.lst.View()
}
