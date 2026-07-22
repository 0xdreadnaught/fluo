package input

import "github.com/0xdreadnaught/fluo/core"

// Router is a minimal stub for event routing. It will be extended in later tasks.
// For now, it holds a reference to the root widget for context in event handlers.
type Router struct {
	root core.Widget
}

// NewRouter creates a new Router for the given root widget.
func NewRouter(root core.Widget) *Router {
	return &Router{root: root}
}

// SetRoot sets the root widget for this router.
func (r *Router) SetRoot(w core.Widget) {
	r.root = w
}

// Root returns the root widget for this router.
func (r *Router) Root() core.Widget {
	return r.root
}
