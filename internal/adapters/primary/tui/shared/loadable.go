package shared

import "charm.land/lipgloss/v2"

// Loadable is a UI cache of asynchronously-loaded data with explicit
// loading and error states. The zero value renders as "no data yet"
// (Loading=false, Value=zero); call .Start() to mark as initial loading.
type Loadable[T any] struct {
	Value      T
	Loading    bool // true during initial load (no Value yet)
	Refreshing bool // true during background refresh (Value still valid)
	Err        error
	// loaded distinguishes "never loaded" from "loaded with zero value"
	loaded bool
}

// Start marks the loadable as initially loading. Returns a new value; the
// old Value is cleared because the caller is about to replace it.
func (l Loadable[T]) Start() Loadable[T] {
	var zero T
	return Loadable[T]{Value: zero, Loading: true}
}

// Refresh marks the loadable as refreshing in the background. Value
// stays visible during the refresh.
func (l Loadable[T]) Refresh() Loadable[T] {
	l.Refreshing = true
	l.Err = nil
	return l
}

// Done resolves the loadable with a result. Clears both Loading and
// Refreshing. If err is non-nil and we already had a Value, the Value
// is preserved (refresh failed but old data is still useful).
func (l Loadable[T]) Done(v T, err error) Loadable[T] {
	l.Loading = false
	l.Refreshing = false
	if err != nil {
		l.Err = err
		if !l.loaded {
			var zero T
			l.Value = zero
		}
		return l
	}
	l.Value = v
	l.Err = nil
	l.loaded = true
	return l
}

// HasValue reports whether a successful load has produced a Value at
// some point. Useful for "show data or skeleton" decisions.
func (l Loadable[T]) HasValue() bool { return l.loaded }

// RenderLoadable is a View helper that handles the four standard states
// uniformly. Callers supply the data renderer; everything else is templated.
func RenderLoadable[T any](
	l Loadable[T],
	render func(T) string,
	styles LoadableStyles,
) string {
	switch {
	case l.Loading && !l.loaded:
		return styles.Loading.Render("loading…")
	case l.Err != nil && !l.loaded:
		return styles.Error.Render("error: " + l.Err.Error())
	case !l.loaded:
		return styles.Empty.Render("(empty)")
	case l.Err != nil:
		// refresh failed but we still have old data — show it with a hint
		return render(l.Value) + "\n" + styles.StaleHint.Render("(stale: "+l.Err.Error()+")")
	default:
		return render(l.Value)
	}
}

type LoadableStyles struct {
	Loading, Error, Empty, StaleHint lipgloss.Style
}
