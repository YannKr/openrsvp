package templates

// EmailColors defines the brand color palette for email templates.
// All values are CSS hex color strings (e.g. "#E54666").
type EmailColors struct {
	Primary        string // #E54666 - warm rose, buttons, links, accents
	PrimaryHover   string // #D63D5C
	PrimaryLight   string // #FDE8EC - info box backgrounds
	PrimaryLighter string // #FFF1F3 - subtle fills
	Heading        string // #1C1917 - stone-900, email headers
	Text           string // #44403C - stone-700, body text
	TextSecondary  string // #78716C - stone-500, secondary text
	TextMuted      string // #A8A29E - stone-400, footer text
	Background     string // #FAFAF9 - stone-50, outer background
	CardBg         string // #FFFFFF - white card
	InfoBg         string // #F5F5F4 - stone-100, info box detail bg
	Border         string // #E7E5E4 - stone-200, borders
	Success        string // #16A34A
	Warning        string // #D97706
	WarningLight   string // #FEF3C7
	WarningDark    string // #78350F
	Error          string // #DC2626
	ErrorLight     string // #FEE2E2
	Link           string // #E54666 - same as primary
}

// DefaultEmailColors returns the standard brand color palette.
func DefaultEmailColors() EmailColors {
	return EmailColors{
		Primary:        "#E54666",
		PrimaryHover:   "#D63D5C",
		PrimaryLight:   "#FDE8EC",
		PrimaryLighter: "#FFF1F3",
		Heading:        "#1C1917",
		Text:           "#44403C",
		TextSecondary:  "#78716C",
		TextMuted:      "#A8A29E",
		Background:     "#FAFAF9",
		CardBg:         "#FFFFFF",
		InfoBg:         "#F5F5F4",
		Border:         "#E7E5E4",
		Success:        "#16A34A",
		Warning:        "#D97706",
		WarningLight:   "#FEF3C7",
		WarningDark:    "#78350F",
		Error:          "#DC2626",
		ErrorLight:     "#FEE2E2",
		Link:           "#E54666",
	}
}
