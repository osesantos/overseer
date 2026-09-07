package styles

import (
	"image/color"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ListIndentUnit is the canonical column count per nesting level for any
// list/tree view in the TUI. Compose it at the render site to indent rows:
//
//	indent := strings.Repeat(" ", depth * styles.ListIndentUnit)
//	row    := s.ListRow.Normal.Render(indent + label)
//
// Do NOT add Padding/Margin to ListRow itself, and do NOT create
// domain-named row styles that bake position into their definition
// (Session.Item.PaddingLeft was that mistake). Position is data —
// it belongs in the render function, computed from the tree's depth.
const ListIndentUnit = 1

type BorderStyles struct {
	Focused     lipgloss.Style
	Blurred     lipgloss.Style
	CharFocused lipgloss.Style
	CharBlurred lipgloss.Style
	Title       lipgloss.Style
}

type PaneStyles struct {
	Sessions  lipgloss.Style
	Status    lipgloss.Style
	Preview   lipgloss.Style
	Container lipgloss.Style
}

type GroupStyles struct {
	Header         lipgloss.Style
	HeaderSelected lipgloss.Style
}

type StatusStyles struct {
	Label     lipgloss.Style
	Value     lipgloss.Style
	Separator lipgloss.Style
}

type FormFieldStyles struct {
	Label        lipgloss.Style
	LabelFocused lipgloss.Style
	Input        lipgloss.Style
	Error        lipgloss.Style
}

type FormStyles struct {
	Field     FormFieldStyles
	Container lipgloss.Style
	// Title is the heading rendered at the top of each modal form body
	// (e.g. "New Session", "Open Branch"). Forms compose the title above
	// the field stack so users can tell the popups apart at a glance.
	Title lipgloss.Style
	// Hint styles the dim per-field hints ("←/→ cycle · ...") and the bottom
	// help line ("Tab next · Enter submit · Esc cancel") rendered inside
	// modal forms. Carries NO background — using Help.Description here would
	// leak the help-bar's distinct bg into the modal as visible stripes.
	Hint lipgloss.Style
	// Input holds the bubbles textinput.Styles shared by every form field
	// (placeholder appearance, focused/blurred prompt color, cursor blink).
	// Configured once at theme load — see [New].
	Input textinput.Styles
}

type HelpStyles struct {
	Bar         lipgloss.Style
	Key         lipgloss.Style
	Description lipgloss.Style
	Separator   lipgloss.Style
}

type EmptyStateStyles struct {
	Title lipgloss.Style
	Hint  lipgloss.Style
}

type TooSmallStyles struct {
	Message lipgloss.Style
}

type LayoutStyles struct {
	Box lipgloss.Style
}

type TabStyles struct {
	Active   lipgloss.Style
	Inactive lipgloss.Style
	Bar      lipgloss.Style
}

// BoardStyles styles the swarm Agents Board: a chat-shaped transcript where each
// agent needs to be tellable apart at a glance.
//
// AuthorColors is derived here rather than added to Theme on purpose — Theme is
// compared with == in its tests, so a slice field would make it non-comparable.
// Deriving from colours every theme already defines also means all twelve themes
// get a working palette with no per-theme edits.
type BoardStyles struct {
	// Author is the base style for an agent's name; callers apply the
	// per-agent foreground from AuthorColorFor on top.
	Author lipgloss.Style
	// AuthorHuman styles the operator's own name, held apart from the agent
	// palette so a human instruction never reads as just another agent.
	AuthorHuman lipgloss.Style
	// Text styles a message body.
	Text lipgloss.Style
	// Seq styles the message number.
	//
	// Held apart from Timestamp and rendered bolder on purpose: agents cite each
	// other by sequence number constantly — 86% of posts in one review run — so it
	// is a navigation aid the operator scans for, not incidental metadata.
	Seq lipgloss.Style
	// Timestamp styles the time prefix on a message.
	Timestamp lipgloss.Style
	// SystemText styles board notices that came from Overseer itself.
	SystemText lipgloss.Style
	// Separator styles the rule drawn between messages.
	Separator lipgloss.Style
	// AuthorColors is the per-agent palette, indexed by agent number via
	// AuthorColorFor.
	AuthorColors []color.Color
}

// AuthorColorFor returns the palette entry for a 1-based agent index, wrapping
// so any index — including zero or negative from malformed board data — yields a
// usable colour instead of panicking mid-render.
func (b BoardStyles) AuthorColorFor(agentIndex int) color.Color {
	if len(b.AuthorColors) == 0 {
		return nil
	}
	slot := (agentIndex - 1) % len(b.AuthorColors)
	if slot < 0 {
		slot += len(b.AuthorColors)
	}
	return b.AuthorColors[slot]
}

type ChatStyles struct {
	// Panel is the border style used for the chat panel container.
	Panel lipgloss.Style
	// UserLabel styles the "You  " prefix on user messages.
	UserLabel lipgloss.Style
	// UserText styles the body of user messages.
	UserText lipgloss.Style
	// AgentLabel styles the "Agent" prefix on agent messages.
	AgentLabel lipgloss.Style
	// AgentText styles the body of agent messages.
	AgentText lipgloss.Style
	// Separator styles the rule drawn between chat messages.
	Separator lipgloss.Style
	// SystemText styles operator-command feedback and loop notices.
	// Rendered dimmer than agent/user text so it reads as infrastructure
	// noise rather than conversational content.
	SystemText lipgloss.Style
	// ThinkingPrefix styles the spinner shown while the agent is thinking.
	ThinkingPrefix lipgloss.Style
	// ThinkingText styles the "thinking…" label beside the spinner.
	ThinkingText lipgloss.Style
}

type SessionDetailsStyles struct {
	// SectionTitle styles the heading rendered above each grouped block
	// ("Pull Request", "Repository"). Bold in the theme's subtext colour
	// so it reads as a secondary heading beneath the panel's border title.
	SectionTitle lipgloss.Style
	// SectionDivider styles the horizontal rule rendered under each
	// SectionTitle; uses the theme border colour to nest inside the panel.
	SectionDivider lipgloss.Style
	// FieldLabel styles the caption rendered in the left column of every
	// row ("Base Branch", "Changes", …). Subtext colour without bold, so
	// it shares the section title's colour family for legibility but
	// stays visually subordinate via weight. Muted has too little
	// contrast against the panel's dark background on themes like
	// Dracula where Muted resolves to a slate-blue.
	FieldLabel lipgloss.Style
	Glyph      lipgloss.Style
	Value      lipgloss.Style
	Good       lipgloss.Style
	Bad        lipgloss.Style
	Warn       lipgloss.Style
	Special    lipgloss.Style
	Hint       lipgloss.Style
}

type Styles struct {
	Border   BorderStyles
	TitleBar struct {
		Base     lipgloss.Style
		Branding lipgloss.Style
		Subtext  lipgloss.Style
	}
	Pane PaneStyles
	// ListRow describes the *appearance* of a row in any list/tree view —
	// foreground, weight, selection background. It deliberately carries no
	// padding or margin: spatial position (depth indent, prefixes) is the
	// render function's job, composed via [ListIndentUnit]. Never add
	// Padding to these styles or create per-depth row variants.
	//
	// Number / NumberSelected style the leading "NN. " prefix used by
	// digit-jump lists; they share the selection background of their
	// matching label state so the row reads as a single highlight.
	//
	// Aux / AuxSelected style right-aligned auxiliary text (e.g. a
	// relative timestamp) appended to a row; like Number/NumberSelected
	// they share the selection background of their matching label state
	// so a selected row reads as a single highlight strip.
	//
	// StatusRunning / StatusWaiting / StatusDead paint the row body with
	// a subtle tinted background derived from the per-status theme palette.
	// Idle and Unknown rows render with Normal — they carry no tint. These
	// variants apply ONLY when the row is not focused; selection always
	// wins over status tint so the user can still see what they have
	// highlighted (see renderSessionNode in session/model.go).
	ListRow struct {
		Normal         lipgloss.Style
		Selected       lipgloss.Style
		Number         lipgloss.Style
		NumberSelected lipgloss.Style
		Aux            lipgloss.Style
		AuxSelected    lipgloss.Style
		StatusRunning  lipgloss.Style
		StatusWaiting  lipgloss.Style
		StatusDead     lipgloss.Style
	}
	Group         GroupStyles
	Status        StatusStyles
	StatusSegment struct {
		Default   lipgloss.Style
		Highlight lipgloss.Style
	}
	Form FormStyles
	Danger struct {
		Title lipgloss.Style
		Body  lipgloss.Style
	}
	Modal struct {
		Box          lipgloss.Style
		Overlay      color.Color
		OverlayStyle lipgloss.Style
		// LinePad is a blank style pre-filled with the modal background.
		// Use it instead of lipgloss.NewStyle().Background(bg) inside modal
		// rendering functions to keep all style origins in styles.go.
		LinePad lipgloss.Style
	}
	Badge struct {
		Key   lipgloss.Style
		Label lipgloss.Style
	}
	Divider struct {
		Horizontal lipgloss.Style
	}
	Help       HelpStyles
	EmptyState EmptyStateStyles
	TooSmall   TooSmallStyles
	Layout     LayoutStyles
	Tab        TabStyles
	// SessionLabel renders the right-aligned status badge on each session
	// row. The badge's foreground colour is set at render time per-label
	// via .Foreground(lipgloss.Color(label.Color)); this base style carries
	// only the typographic weight so colour is the only per-label variable.
	SessionLabel   lipgloss.Style
	SessionDetails SessionDetailsStyles
	// Toast holds the styles used by the transient top-right notification
	// that appears during and after the startup repo-discovery scan.
	Toast struct {
		// Indexing is shown while the background discovery job is running.
		Indexing lipgloss.Style
		// Done is shown briefly after discovery completes successfully.
		Done lipgloss.Style
		// GapLine is the full-width right-aligned wrapper for the gap row.
		GapLine lipgloss.Style
	}
	Glyphs Glyphs
	Chat   ChatStyles
	Board  BoardStyles
}

// New builds *Styles using the dark theme; production code should use NewWithTheme.
func New() *Styles {
	return NewWithTheme("dark", false)
}

// NewWithTheme builds *Styles using the named theme; unknown names fall back to dark.
func NewWithTheme(themeName string, disableEmoji bool) *Styles {
	theme := LoadTheme(themeName)

	helpKeyStyle := lipgloss.NewStyle().Foreground(theme.Text).Background(theme.HelpBarBg).Bold(true)
	helpBarStyle := lipgloss.NewStyle().Background(theme.HelpBarBg).Padding(0, 1)

	return &Styles{
		Border: BorderStyles{
			Focused: lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(theme.BorderFocus),
			Blurred: lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(theme.Border),
			CharFocused: lipgloss.NewStyle().Foreground(theme.BorderFocus),
			CharBlurred: lipgloss.NewStyle().Foreground(theme.Border),
			Title:       lipgloss.NewStyle().Foreground(theme.Text).Bold(true),
		},
		TitleBar: struct {
			Base     lipgloss.Style
			Branding lipgloss.Style
			Subtext  lipgloss.Style
		}{
			Base:     lipgloss.NewStyle().Background(theme.Primary).Foreground(theme.TitleText),
			Branding: lipgloss.NewStyle().Background(theme.Primary).Foreground(theme.TitleText).Bold(true).Padding(0, 1).AlignHorizontal(lipgloss.Center),
			Subtext:  lipgloss.NewStyle().Background(theme.Primary).Foreground(theme.TitleSubtext).Padding(0, 1),
		},
		Pane: PaneStyles{
			Sessions:  lipgloss.NewStyle().Padding(0, 1),
			Status:    lipgloss.NewStyle().Padding(0, 1),
			Preview:   lipgloss.NewStyle().Padding(0, 1),
			Container: lipgloss.NewStyle().Padding(0, 1),
		},
		ListRow: struct {
			Normal         lipgloss.Style
			Selected       lipgloss.Style
			Number         lipgloss.Style
			NumberSelected lipgloss.Style
			Aux            lipgloss.Style
			AuxSelected    lipgloss.Style
			StatusRunning  lipgloss.Style
			StatusWaiting  lipgloss.Style
			StatusDead     lipgloss.Style
		}{
			Normal:         lipgloss.NewStyle().Foreground(theme.Text),
			Selected:       lipgloss.NewStyle().Foreground(theme.Text).Bold(true).Background(theme.SelectionBg),
			Number:         lipgloss.NewStyle().Foreground(theme.Subtext),
			NumberSelected: lipgloss.NewStyle().Foreground(theme.Subtext).Bold(true).Background(theme.SelectionBg),
			Aux:            lipgloss.NewStyle().Foreground(theme.Muted),
			AuxSelected:    lipgloss.NewStyle().Foreground(theme.Subtext).Background(theme.SelectionBg),
			StatusRunning:  lipgloss.NewStyle().Foreground(theme.Text).Background(theme.StatusRunningBg),
			StatusWaiting:  lipgloss.NewStyle().Foreground(theme.Text).Background(theme.StatusWaitingBg),
			StatusDead:     lipgloss.NewStyle().Foreground(theme.Text).Background(theme.StatusDeadBg),
		},
		Group: GroupStyles{
			Header:         lipgloss.NewStyle().Foreground(theme.Accent).Bold(true),
			HeaderSelected: lipgloss.NewStyle().Foreground(theme.Accent).Bold(true).Background(theme.SelectionBg),
		},
		Status: StatusStyles{
			Label:     lipgloss.NewStyle().Foreground(theme.Subtext),
			Value:     lipgloss.NewStyle().Foreground(theme.Text),
			Separator: lipgloss.NewStyle().Foreground(theme.Muted),
		},
		StatusSegment: struct {
			Default   lipgloss.Style
			Highlight lipgloss.Style
		}{
			Default:   lipgloss.NewStyle().Background(theme.HelpBg).Foreground(theme.Subtext).Padding(0, 1),
			Highlight: lipgloss.NewStyle().Background(theme.Primary).Foreground(theme.TitleText).Padding(0, 1).Bold(true),
		},
		Form: FormStyles{
			Container: lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(theme.BorderFocus).
				Padding(1, 2),
			Title: lipgloss.NewStyle().Foreground(theme.Primary).Background(theme.ModalBg).Bold(true).MarginBottom(1),
			Hint:  lipgloss.NewStyle().Foreground(theme.Subtext).Background(theme.ModalBg),
			Field: FormFieldStyles{
				Label:        lipgloss.NewStyle().Foreground(theme.Subtext).Background(theme.ModalBg),
				LabelFocused: lipgloss.NewStyle().Foreground(theme.Accent).Background(theme.ModalBg).Bold(true),
				Input:        lipgloss.NewStyle().Foreground(theme.Text).Background(theme.ModalBg),
				Error:        lipgloss.NewStyle().Foreground(theme.Warning).Background(theme.ModalBg),
			},
			Input: textinput.Styles{
				Focused: textinput.StyleState{
					Placeholder: lipgloss.NewStyle().Foreground(theme.Muted).Background(theme.ModalBg).Italic(true),
					Prompt:      lipgloss.NewStyle().Foreground(theme.Accent).Background(theme.ModalBg).Bold(true),
					Text:        lipgloss.NewStyle().Foreground(theme.Text).Background(theme.ModalBg),
				},
				Blurred: textinput.StyleState{
					Placeholder: lipgloss.NewStyle().Foreground(theme.Muted).Background(theme.ModalBg).Italic(true),
					Prompt:      lipgloss.NewStyle().Foreground(theme.Muted).Background(theme.ModalBg),
					Text:        lipgloss.NewStyle().Foreground(theme.Subtext).Background(theme.ModalBg),
				},
				Cursor: textinput.CursorStyle{
					Color: theme.Accent,
					Shape: tea.CursorBlock,
					Blink: true,
				},
			},
		},
		Danger: struct {
			Title lipgloss.Style
			Body  lipgloss.Style
		}{
			Title: lipgloss.NewStyle().Foreground(theme.Danger).Bold(true),
			Body:  lipgloss.NewStyle().Foreground(theme.Danger),
		},
		Modal: struct {
			Box          lipgloss.Style
			Overlay      color.Color
			OverlayStyle lipgloss.Style
			LinePad      lipgloss.Style
		}{
			Box: lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(theme.BorderFocus).
				Background(theme.ModalBg).
				Foreground(theme.Text).
				Padding(1, 3),
			Overlay:      theme.OverlayBg,
			OverlayStyle: lipgloss.NewStyle().Background(theme.OverlayBg),
			LinePad:      lipgloss.NewStyle().Background(theme.ModalBg),
		},
		Badge: struct {
			Key   lipgloss.Style
			Label lipgloss.Style
		}{
			Key:   helpKeyStyle,
			Label: lipgloss.NewStyle().Foreground(theme.Subtext),
		},
		Divider: struct {
			Horizontal lipgloss.Style
		}{
			Horizontal: lipgloss.NewStyle().Foreground(theme.Border),
		},
		Help: HelpStyles{
			Bar:         helpBarStyle,
			Key:         helpKeyStyle,
			Description: lipgloss.NewStyle().Foreground(theme.Subtext).Background(theme.HelpBarBg),
			Separator:   lipgloss.NewStyle().Foreground(theme.Muted).Background(theme.HelpBarBg),
		},
		EmptyState: EmptyStateStyles{
			Title: lipgloss.NewStyle().Foreground(theme.Subtext).Bold(true),
			Hint:  lipgloss.NewStyle().Foreground(theme.Subtext),
		},
		TooSmall: TooSmallStyles{
			Message: lipgloss.NewStyle().Foreground(theme.Warning).Bold(true),
		},
		Layout: LayoutStyles{
			Box: lipgloss.NewStyle(),
		},
		Tab: TabStyles{
			Active:   lipgloss.NewStyle().Foreground(theme.TitleText).Background(theme.Primary).Bold(true).Padding(0, 2),
			Inactive: lipgloss.NewStyle().Foreground(theme.Subtext).Padding(0, 2),
			Bar:      lipgloss.NewStyle().Foreground(theme.Border),
		},
		SessionLabel: lipgloss.NewStyle().Bold(true),
		SessionDetails: SessionDetailsStyles{
			SectionTitle:   lipgloss.NewStyle().Foreground(theme.Subtext).Bold(true),
			SectionDivider: lipgloss.NewStyle().Foreground(theme.Border),
			FieldLabel:     lipgloss.NewStyle().Foreground(theme.Subtext),
			Glyph:          lipgloss.NewStyle().Foreground(theme.Subtext),
			Value:          lipgloss.NewStyle().Foreground(theme.Text),
			Good:           lipgloss.NewStyle().Foreground(theme.Accent).Bold(true),
			Bad:            lipgloss.NewStyle().Foreground(theme.Danger).Bold(true),
			Warn:           lipgloss.NewStyle().Foreground(theme.Warning).Bold(true),
			Special:        lipgloss.NewStyle().Foreground(theme.Primary).Bold(true),
			Hint:           lipgloss.NewStyle().Foreground(theme.Subtext).Italic(true),
		},
		Toast: struct {
			Indexing lipgloss.Style
			Done     lipgloss.Style
			GapLine  lipgloss.Style
		}{
			Indexing: lipgloss.NewStyle().Foreground(theme.Warning).Bold(true),
			Done:     lipgloss.NewStyle().Foreground(theme.Accent).Bold(true),
			GapLine:  lipgloss.NewStyle().AlignHorizontal(lipgloss.Right),
		},
		Glyphs: NewGlyphs(disableEmoji),
		Chat: ChatStyles{
			Panel: lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(theme.BorderFocus),
			UserLabel:      lipgloss.NewStyle().Foreground(theme.Accent).Bold(true),
			UserText:       lipgloss.NewStyle().Foreground(theme.Text),
			AgentLabel:     lipgloss.NewStyle().Foreground(theme.Primary).Bold(true),
			AgentText:      lipgloss.NewStyle().Foreground(theme.Text),
			SystemText:     lipgloss.NewStyle().Foreground(theme.Muted).Italic(true),
			Separator:      lipgloss.NewStyle().Foreground(theme.Border),
			ThinkingPrefix: lipgloss.NewStyle().Foreground(theme.Accent),
			ThinkingText:   lipgloss.NewStyle().Foreground(theme.Muted).Italic(true),
		},
		Board: BoardStyles{
			Author:      lipgloss.NewStyle().Bold(true).Foreground(theme.Primary),
			AuthorHuman: lipgloss.NewStyle().Bold(true).Foreground(theme.TitleText),
			Text:        lipgloss.NewStyle().Foreground(theme.Text),
			Seq:         lipgloss.NewStyle().Foreground(theme.Subtext).Bold(true),
			Timestamp:   lipgloss.NewStyle().Foreground(theme.Subtext),
			SystemText:  lipgloss.NewStyle().Foreground(theme.Muted).Italic(true),
			Separator:   lipgloss.NewStyle().Foreground(theme.Border),
			// Chosen for spread, not aesthetics: every theme aliases
			// StatusRunningFg to Accent and StatusWaitingFg to Warning, so
			// including those would silently collapse the palette to ~5 slots
			// and make agents 6-8 look like agents 1-3.
			AuthorColors: []color.Color{
				theme.Primary,
				theme.Accent,
				theme.Warning,
				theme.Danger,
				theme.TitleSubtext,
				theme.BorderFocus,
				theme.Subtext,
				theme.Muted,
			},
		},
	}
}
