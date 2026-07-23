// Package input turns raw pointer/keyboard/wheel events into dispatch
// against a core.Widget tree. Router is the package's central type: it
// hit-tests pointer events against the tree rooted at SetRoot, bubbles them
// (and Enter/Leave transitions) along the hit path, tracks an optional
// nested pointer capture (Capture/Release) that redirects all pointer
// events straight to one widget, and tracks keyboard focus (Focus/Focused/
// FocusNext/FocusPrev), bubbling KeyDown/KeyUp from the focused widget up
// its core.ParentOf ancestor chain. Widgets opt into event handling by
// implementing one or more of PointerHandler, KeyHandler, Focusable, or
// CursorShaper; PointerEvent and KeyEvent (each with a settable Handled
// flag) are the payloads delivered to them. Clipboard abstracts host clip-
// board access (wired via SetClipboard) so TextBox's Ctrl+C/X/V works
// without input depending on any windowing package. The app package is
// Router's principal driver, translating glfw callbacks into Router calls
// every frame.
package input
