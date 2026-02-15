package cli

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Logo contains the ASCII art for the application
const Logo = `░█▀█░█▀▀░▀█▀░█▀▄░█▀█░█▀▀░▀█▀░█▀▀
░█░█░█░░░░█░░█▀▄░█▀█░█▀▀░░█░░█░░
░▀▀▀░▀▀▀░░▀░░▀░▀░▀░▀░▀░░░▀▀▀░▀▀▀`

// RenderLogo returns the styled logo with gradient colors.
func RenderLogo() string {
	logoLines := strings.Split(Logo, "\n")
	styledLogo := make([]string, len(logoLines))

	for i, line := range logoLines {
		var styledLine strings.Builder
		for _, char := range line {
			if char == '░' {
				styledLine.WriteString(lipgloss.NewStyle().Foreground(Theme.TextSubtle).Render(string(char)))
			} else {
				color := Theme.LogoGradient[i%len(Theme.LogoGradient)]
				styledLine.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Bold(true).Render(string(char)))
			}
		}
		styledLogo[i] = styledLine.String()
	}

	return strings.Join(styledLogo, "\n")
}

// Theme defines the Sky Blue color palette for the entire application
var Theme = struct {
	// Primary colors - Sky Blue theme
	Primary       lipgloss.Color // Main brand color (Sky Blue 400) #38BDF8
	PrimaryDark   lipgloss.Color // Darker variant (Sky Blue 500) #0EA5E9
	PrimaryStrong lipgloss.Color // Strong variant (Sky Blue 500) #0EA5E9
	PrimaryLight  lipgloss.Color // Light variant (Sky Blue 300) #7DD3FC

	// Cyan accent colors
	Cyan      lipgloss.Color // Cyan 400 #22D3EE
	CyanLight lipgloss.Color // Cyan 300 #67E8F9

	// Semantic colors - HTTP methods (pastel shades)
	Success lipgloss.Color // GET - Emerald 400 #34D399
	Error   lipgloss.Color // DELETE - Rose 400 #FB7185
	Warning lipgloss.Color // PUT - Amber 400 #FBBF24
	Info    lipgloss.Color // POST - Sky Blue 400 #38BDF8

	// Special accent colors
	Yellow  lipgloss.Color // Highlight - Yellow 300 #FDE047
	Fuchsia lipgloss.Color // Strong accent - Fuchsia 400 #E879F9

	// Text colors - readable hierarchy
	Text       lipgloss.Color // Primary text (Slate 50) #F8FAFC
	TextMuted  lipgloss.Color // Secondary text (Slate 300) #CBD5E1
	TextSubtle lipgloss.Color // Muted text (Slate 400) #94A3B8
	Gray       lipgloss.Color // Subtle text (Slate 500) #64748B
	White      lipgloss.Color // Pure white #FFFFFF

	// Background colors - depth and contrast
	BgDark      lipgloss.Color // Main background (Slate 950) #020617
	BgSecondary lipgloss.Color // Secondary background (Slate 900) #0F172A
	BgElevated  lipgloss.Color // Elevated background (Slate 800) #1E293B
	BgCode      lipgloss.Color // Code background (Slate 800) #1E293B
	BgSelected  lipgloss.Color // Selected item background (Slate 800) #1E293B

	// Border colors
	BorderSubtle  lipgloss.Color // Subtle border (Slate 700) #334155
	BorderDefault lipgloss.Color // Default border (Slate 600) #475569
	BorderBright  lipgloss.Color // Bright border (Sky Blue 500) #0EA5E9

	// Blue/Indigo shades (for gradients/headers)
	Blue      lipgloss.Color // Blue 400 #60A5FA
	BlueLight lipgloss.Color // Blue 300 #93C5FD
	Indigo    lipgloss.Color // Indigo 400 #818CF8
	Violet    lipgloss.Color // Violet 400 #A78BFA

	// Gradients
	LogoGradient      []string // Sky blue progression for logo
	AnimationGradient []string // Rainbow effect for animations
	HeaderGradient    []string // Sky to purple for headers
}{
	// Primary - Sky Blue
	Primary:       lipgloss.Color("#38BDF8"), // Sky Blue 400
	PrimaryDark:   lipgloss.Color("#0EA5E9"), // Sky Blue 500
	PrimaryStrong: lipgloss.Color("#0EA5E9"), // Sky Blue 500
	PrimaryLight:  lipgloss.Color("#7DD3FC"), // Sky Blue 300

	// Cyan accents
	Cyan:      lipgloss.Color("#22D3EE"), // Cyan 400
	CyanLight: lipgloss.Color("#67E8F9"), // Cyan 300

	// Semantic - HTTP methods (pastel)
	Success: lipgloss.Color("#34D399"), // GET - Emerald 400
	Error:   lipgloss.Color("#FB7185"), // DELETE - Rose 400
	Warning: lipgloss.Color("#FBBF24"), // PUT - Amber 400
	Info:    lipgloss.Color("#38BDF8"), // POST - Sky Blue 400

	// Special accents
	Yellow:  lipgloss.Color("#FDE047"), // Highlight - Yellow 300
	Fuchsia: lipgloss.Color("#E879F9"), // Strong accent - Fuchsia 400

	// Text - hierarchy
	Text:       lipgloss.Color("#F8FAFC"), // Slate 50
	TextMuted:  lipgloss.Color("#CBD5E1"), // Slate 300
	TextSubtle: lipgloss.Color("#94A3B8"), // Slate 400
	Gray:       lipgloss.Color("#64748B"), // Slate 500
	White:      lipgloss.Color("#FFFFFF"), // Pure white

	// Backgrounds
	BgDark:      lipgloss.Color("#020617"), // Slate 950
	BgSecondary: lipgloss.Color("#0F172A"), // Slate 900
	BgElevated:  lipgloss.Color("#1E293B"), // Slate 800
	BgCode:      lipgloss.Color("#1E293B"), // Slate 800
	BgSelected:  lipgloss.Color("#2a2a2a"), // Selected item background

	// Borders
	BorderSubtle:  lipgloss.Color("#334155"), // Slate 700
	BorderDefault: lipgloss.Color("#475569"), // Slate 600
	BorderBright:  lipgloss.Color("#0EA5E9"), // Sky Blue 500

	// Blue/Indigo shades
	Blue:      lipgloss.Color("#60A5FA"), // Blue 400
	BlueLight: lipgloss.Color("#93C5FD"), // Blue 300
	Indigo:    lipgloss.Color("#818CF8"), // Indigo 400
	Violet:    lipgloss.Color("#A78BFA"), // Violet 400

	// Gradients
	LogoGradient: []string{
		"#0EA5E9", // Sky Blue 500
		"#38BDF8", // Sky Blue 400
		"#7DD3FC", // Sky Blue 300
		"#BAE6FD", // Sky Blue 200
	},
	AnimationGradient: []string{
		"#22D3EE", // Cyan 400
		"#38BDF8", // Sky Blue 400
		"#60A5FA", // Blue 400
		"#818CF8", // Indigo 400
	},
	HeaderGradient: []string{
		"#38BDF8", // Sky Blue 400
		"#60A5FA", // Blue 400
		"#818CF8", // Indigo 400
		"#A78BFA", // Violet 400
	},
}

var ProcessingAnimationFrames = []string{
	"● ○ ○ ○ ○ ○",
	"● ● ○ ○ ○ ○",
	"● ● ● ○ ○ ○",
	"● ● ● ● ○ ○",
	"● ● ● ● ● ○",
	"● ● ● ● ● ●",
	"○ ● ● ● ● ●",
	"○ ○ ● ● ● ●",
	"○ ○ ○ ● ● ●",
	"○ ○ ○ ○ ● ●",
	"○ ○ ○ ○ ○ ●",
	"○ ○ ○ ○ ● ●",
	"○ ○ ○ ● ● ●",
	"○ ○ ● ● ● ●",
	"○ ● ● ● ● ●",
	"● ● ● ● ● ●",
	"● ● ● ● ● ○",
	"● ● ● ● ○ ○",
	"● ● ● ○ ○ ○",
	"● ● ○ ○ ○ ○",
	"● ○ ○ ○ ○ ○",
}

func RenderProcessingAnimation(frame int) string {
	return ProcessingAnimationFrames[frame%len(ProcessingAnimationFrames)]
}
