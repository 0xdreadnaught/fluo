// Package controls provides fluo's built-in widget set, all styled from
// theme.Active() rather than hard-coded values. Layout containers —
// Border, Fixed, StackPanel, Grid, DockPanel, WrapPanel, Canvas, and
// ScrollViewer — arrange child core.Widgets without drawing interactive
// chrome of their own; SplitPanel sits alongside them as the one container
// that does, giving its two panes a draggable divider that rewrites the
// split ratio. Interactive controls — Button, ToggleButton,
// CheckBox, RadioButton/RadioGroup, ToggleSwitch, TextBox, Slider,
// ProgressBar, ComboBox, and ToolTipArea — follow one uniform contract:
// programmatic setters (SetChecked, SetValue, SetText, ...) are silent,
// while OnChanged fires only for user-driven changes. Popup-capable
// controls (ComboBox's dropdown, ToolTipArea's tip, menus, dialogs) render
// into the nearest OverlayHost ancestor, which also implements light-
// dismiss capture via SetRouter. Advanced controls — ListView, TreeView,
// TabControl, Expander, MenuBar, Dialog (ShowDialog), DataGrid, TitleBar,
// and AcrylicSurface — round out the set: ListView and DataGrid are both
// virtualized, but only ListView owns an external subscription that must
// be released via Dispose when its tree is discarded — DataGrid's Dispose
// exists purely as a documented no-op, so a cancel path can call Dispose
// uniformly on every virtualized control without a type switch. Toast is
// the one overlay with no widget of its own to construct: OverlayHost.
// ShowToast stacks a transient, self-dismissing card in the host's
// bottom-right corner, optionally accented by Severity and auto-closing on
// a wired timers.Queue. Package
// bind connects core.Property[T] and bind.List[T] values to these
// controls; cmd/fluo-gallery is the reference composition of the whole
// set.
package controls
