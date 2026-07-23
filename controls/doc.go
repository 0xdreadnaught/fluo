// Package controls provides fluo's built-in widget set, all styled from
// theme.Active() rather than hard-coded values. Layout containers —
// Border, Fixed, StackPanel, Grid, DockPanel, WrapPanel, Canvas, and
// ScrollViewer — arrange child core.Widgets without drawing interactive
// chrome of their own. Interactive controls — Button, ToggleButton,
// CheckBox, RadioButton/RadioGroup, ToggleSwitch, TextBox, Slider,
// ProgressBar, ComboBox, and ToolTipArea — follow one uniform contract:
// programmatic setters (SetChecked, SetValue, SetText, ...) are silent,
// while OnChanged fires only for user-driven changes. Popup-capable
// controls (ComboBox's dropdown, ToolTipArea's tip, menus, dialogs) render
// into the nearest OverlayHost ancestor, which also implements light-
// dismiss capture via SetRouter. Advanced controls — ListView, TreeView,
// TabControl, Expander, MenuBar, Dialog (ShowDialog), DataGrid, TitleBar,
// and AcrylicSurface — round out the set: ListView and DataGrid are
// virtualized and, uniquely, own an external subscription that must be
// released via Dispose when their tree is discarded. Package bind connects
// core.Property[T] and bind.List[T] values to these controls; cmd/
// fluo-gallery is the reference composition of the whole set.
package controls
