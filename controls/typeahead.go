package controls

import (
	"strings"
	"unicode"
)

// typeAheadResetSeconds is how long a type-ahead prefix survives between
// keystrokes. After a longer pause the next printable key starts a fresh
// prefix, matching the OS list-box convention: a deliberate pause means "new
// search", not "extend the previous one".
const typeAheadResetSeconds = 0.5

// typeAhead is the shared list type-ahead state — a case-insensitive prefix
// buffer plus the timestamp of the last keystroke that fed it. ComboBox,
// ListView, and TreeView each embed one and feed printable runes to it (see
// feed). It is UI-agnostic (it only computes an index) and clock-agnostic (the
// caller passes the event time via KeyEvent.Time), so feed's whole contract is
// exercised directly in typeahead_test without any widget.
type typeAhead struct {
	buf  string
	last float64
}

// feed advances the type-ahead for one printable rune typed at time now
// (monotonic seconds; see KeyEvent.Time) over count items whose labels come
// from label(i), with current the selected index (-1 for none). It returns the
// index the selection should move to and whether it matched at all — false
// leaves the caller's selection untouched.
//
// Behavior, matching OS list type-ahead:
//   - After a pause longer than typeAheadResetSeconds the buffer resets and
//     this rune starts a fresh single-character search.
//   - A repeated same character (the buffer already IS that character) CYCLES
//     to the next item beginning with it, searching from current+1.
//   - A different character within the window EXTENDS the prefix and searches
//     from current, so a selection that still matches the longer prefix stays
//     put instead of jumping to another match.
//   - Matching is case-insensitive prefix matching, and the search wraps.
func (ta *typeAhead) feed(now float64, r rune, count, current int, label func(int) string) (int, bool) {
	if count <= 0 {
		return -1, false
	}
	lc := string(unicode.ToLower(r))
	fromNext := false
	switch {
	case now-ta.last > typeAheadResetSeconds:
		ta.buf = lc
		fromNext = true
	case ta.buf == lc: // repeated single character — cycle to the next match
		fromNext = true
	default:
		ta.buf += lc
	}
	ta.last = now

	// current == -1 (nothing selected) starts the scan at 0 either way; a real
	// selection starts at current+1 when cycling/refreshing, or at current
	// itself when extending the prefix (so a still-matching selection holds).
	start := 0
	if current >= 0 {
		if fromNext {
			start = current + 1
		} else {
			start = current
		}
	}
	for k := 0; k < count; k++ {
		i := (start + k) % count
		if strings.HasPrefix(strings.ToLower(label(i)), ta.buf) {
			return i, true
		}
	}
	return -1, false
}
