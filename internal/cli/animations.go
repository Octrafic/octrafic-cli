package cli

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func animationTick() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return animationTickMsg(t)
	})
}
