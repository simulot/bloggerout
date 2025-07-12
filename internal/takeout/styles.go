package takeout

import "github.com/charmbracelet/lipgloss/v2"

var (
	BlogTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	BlogDescStyle   = lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("245"))
	PostsCountStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("36")).Bold(true)
	SpacerStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
)
