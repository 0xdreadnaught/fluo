package input

import (
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
)

// HitPath returns the widget chain root→…→topmost leaf whose arranged bounds
// contain p. Hidden widgets (core.IsVisible false) are skipped along with
// their subtrees. Children are tested LAST-to-first (topmost painted wins).
// A widget is on the path only if its own bounds contain p. Empty if the root
// misses. Root need not contain p for children to be tested? NO — the root
// must contain p (bounds gate applies at every level).
func HitPath(root core.Widget, p render.Point) []core.Widget {
	// Check if root is visible and bounds contain the point
	if !core.IsVisible(root) || !core.BoundsOf(root).Contains(p) {
		return nil
	}

	// Start with the root in the path
	path := []core.Widget{root}

	// Get children and test them in reverse order (LAST-to-first)
	children := root.Children()
	if len(children) == 0 {
		return path
	}

	// Test children in reverse order (topmost painted wins)
	for i := len(children) - 1; i >= 0; i-- {
		child := children[i]
		childPath := HitPath(child, p)
		if childPath != nil {
			// Append the child's path to our path
			return append(path, childPath...)
		}
	}

	// No child hit; return just the root
	return path
}
