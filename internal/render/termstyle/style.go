package termstyle

import "charm.land/lipgloss/v2"

const (
	CursorBackgroundANSI = "\x1b[48;5;236m"
	ANSIReset            = "\x1b[m"
)

var (
	KeyStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	StringStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	ScalarStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	SyntaxStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	NoteStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	RequiredStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9"))
	URLStyle      = lipgloss.NewStyle().Underline(true).Foreground(lipgloss.Color("4"))

	CursorStyle       = lipgloss.NewStyle().Background(lipgloss.Color("236"))
	DetailTitleStyle  = lipgloss.NewStyle().Bold(true).Underline(true).Foreground(lipgloss.Color("15"))
	FilterHitStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("214"))
	FilterStatusStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("214"))
	DetailLabelStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("8"))
	DetailValueStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	OptionalStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	FooterStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	SeparatorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)
