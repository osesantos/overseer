package styles

type Glyphs struct {
	Branch        string
	ProjectMode   string
	Repo          string
	PRLink        string
	PRStateOpen   string
	PRStateClosed string
	PRStateMerged string
	PRStateDraft  string
	Added         string
	Removed       string
	Files         string
	CheckPass     string
	CheckFail     string
	CheckPending  string
	Warning       string
	LabelWIP      string
	LabelTesting  string
	LabelReady    string
	LabelDone     string
	LabelDraft    string
}

func NewGlyphs(disableEmoji bool) Glyphs {
	if disableEmoji {
		return Glyphs{
			Branch:        "⎇",
			ProjectMode:   "·",
			Repo:          "⊞",
			PRLink:        "⎘",
			PRStateOpen:   "●",
			PRStateClosed: "●",
			PRStateMerged: "●",
			PRStateDraft:  "●",
			Added:         "⊕",
			Removed:       "⊖",
			Files:         "▤",
			CheckPass:     "✓",
			CheckFail:     "✗",
			CheckPending:  "◷",
			Warning:       "⚠",
			LabelWIP:      "◐",
			LabelTesting:  "◉",
			LabelReady:    "◆",
			LabelDone:     "✓",
			LabelDraft:    "◌",
		}
	}
	return Glyphs{
		Branch:        "🌿",
		ProjectMode:   "📂",
		Repo:          "📦",
		PRLink:        "🔗",
		PRStateOpen:   "🟢",
		PRStateClosed: "🔴",
		PRStateMerged: "🟣",
		PRStateDraft:  "⚪",
		Added:         "🟩",
		Removed:       "🟥",
		Files:         "📄",
		CheckPass:     "✅",
		CheckFail:     "❌",
		CheckPending:  "⏳",
		Warning:       "⚠️",
		LabelWIP:      "🚧",
		LabelDraft:    "📝",
		LabelTesting:  "🧪",
		LabelReady:    "🚀",
		LabelDone:     "✅",
	}
}

func (g Glyphs) LabelGlyph(code string) string {
	switch code {
	case "WIP":
		return g.LabelWIP
	case "draft":
		return g.LabelDraft
	case "testing":
		return g.LabelTesting
	case "ready":
		return g.LabelReady
	case "done":
		return g.LabelDone
	}
	return ""
}
