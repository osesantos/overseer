package styles

import (
	"image/color"

	"github.com/lucasb-eyer/go-colorful"
)

// OmarchyTheme maps an Omarchy palette onto a Theme; false when background, foreground or accent is missing or not hex.
func OmarchyTheme(p map[string]string) (Theme, bool) {
	c := func(keys ...string) color.Color {
		for _, k := range keys {
			if v, err := colorful.Hex(p[k]); err == nil {
				return v
			}
		}
		return nil
	}
	bg, fg, accent := c("background"), c("foreground"), c("accent", "blue")
	if bg == nil || fg == nil || accent == nil {
		return Theme{}, false
	}
	or := func(v, def color.Color) color.Color {
		if v == nil {
			return def
		}
		return v
	}
	tint := func(v color.Color) color.Color {
		a, _ := colorful.MakeColor(v)
		b, _ := colorful.MakeColor(bg)
		return b.BlendLab(a, 0.2).Clamped()
	}
	muted := or(c("muted", "dark_foreground"), fg)
	sel := or(c("selection", "lighter_background"), muted)
	green, yellow, red := or(c("green"), accent), or(c("yellow"), accent), or(c("red"), accent)
	sub := or(c("light_foreground"), fg)
	return Theme{
		Primary:         accent,
		Accent:          green,
		Warning:         yellow,
		Danger:          red,
		Muted:           muted,
		Text:            fg,
		Subtext:         sub,
		Border:          sel,
		BorderFocus:     accent,
		SelectionBg:     sel,
		TitleText:       bg,
		TitleSubtext:    or(c("dark_background"), bg),
		HelpBg:          or(c("darker_background", "dark_background"), bg),
		HelpBarBg:       or(c("dark_background"), bg),
		HelpKeyBg:       sel,
		ModalBg:         or(c("lighter_background"), bg),
		OverlayBg:       or(c("darker_background", "dark_background"), bg),
		StatusRunningFg: green,
		StatusRunningBg: tint(green),
		StatusWaitingFg: yellow,
		StatusWaitingBg: tint(yellow),
		StatusIdleFg:    muted,
		StatusDeadFg:    red,
		StatusDeadBg:    tint(red),
		StatusUnknownFg: sub,
	}, true
}

// FollowsOmarchy reports whether the configured theme name defers to Omarchy.
func FollowsOmarchy(name string) bool { return name == "" || name == "omarchy" }

// ResolveTheme picks a named theme first, then the Omarchy palette, then dark.
func ResolveTheme(name string, palette map[string]string) Theme {
	if !FollowsOmarchy(name) {
		return LoadTheme(name)
	}
	if t, ok := OmarchyTheme(palette); ok {
		return t
	}
	return DarkTheme()
}
