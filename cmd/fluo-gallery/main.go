// Command fluo-gallery is the widget gallery: it grows a page per control as
// phases land. Phase 3: interactive swatches (pointer/focus/cursor) plus a
// ScrollViewer over a taller-than-viewport content stack. Phase 4: the whole
// tree is built from theme.Active()'s tokens (buildUI), plus a live T-key
// toggle between classic Light and Dark. Phase 5: a Controls section at the
// top of the scroll content exercises every core control built this phase
// (Button/ToggleButton/CheckBox/RadioButton+Group/ToggleSwitch/TextBox/
// Slider/ProgressBar/ComboBox/ToolTipArea), and the root becomes an
// OverlayHost (controls.NewOverlayHost) so ComboBox's popup and
// ToolTipArea's tip have somewhere to render above the rest of the tree.
// Phase 6 adds a Binding demo to the Controls section: a
// core.Property[string] two-way bound to the existing TextBox plus one-way
// to a mirror TextBlock, a core.Property[float32] two-way bound to the
// Slider and one-way to the ProgressBar (replacing Phase 5's direct
// Slider.OnChanged wiring), and a bind.List[string] rendered into a
// StackPanel via bind.Items, appended to by an "Add" Button. The three
// models (textProp/sliderProp/itemList) are constructed once in main and
// outlive every theme-toggle rebuild; buildUI only ever creates fresh
// bindings onto them. Every binder buildUI creates returns a cancel func,
// collected into main's cancels slice — see the T-key toggle in main for
// the cancel-before-rebuild discipline this enables.
//
// Phase 8 gives the gallery real window chrome: app.Config.Undecorated is
// set, and buildUI's dock gains a controls.TitleBar (DockTop, replacing the
// old plain-Border title strip) wired to Ctx.Minimize/ToggleMaximize/Close.
// Dragging the bar isn't the TitleBar's own job (DragRegion is a pure
// geometry query — see its doc comment); main's frame callback does the
// press-edge check itself each frame (current Ctx.Mouse.Down true, previous
// frame's false, and the press position inside the CURRENT titleBar's
// DragRegion), calling Ctx.BeginDrag exactly once per press rather than
// every frame a button stays held (which would keep re-arming the drag's
// start position and cancel out all movement — see BeginDrag's own doc
// comment on window.go's Run). Because every rebuild (theme toggle, page
// change) replaces the whole tree, buildUI now returns the fresh TitleBar
// alongside the OverlayHost root so main's build() can keep a live
// reference for that per-frame check. The content pane's plain
// WindowBackground Border is replaced by a controls.AcrylicSurface, so the
// translucent backdrop-blur (or its tinted-fallback degrade — see
// AcrylicSurface's own doc comment) shows behind the page content instead of
// a flat fill. The Controls page's four Button-family widgets (Click me,
// Accent, Toggle, Add) opt into SetAnimated(true) + SetTimers(tq) so their
// fill cross-fades instead of snapping between rest/hover/pressed — the
// other Controls-page widgets (CheckBox/RadioButton/ToggleSwitch) have no
// SetAnimated of their own to opt into (see anim/controls.animation.go).
//
// Phase 7 turns the nav sidebar into a real page switcher: the three static
// "Layout"/"Panels"/"Text" TextBlocks are replaced by a ListView of page
// names ("Controls"/"Advanced") — dogfooding ListView itself as the nav
// widget — bound two-way via bind.ListSelected to a core.Property[int]
// (pageProp) that main owns and never recreates, exactly like
// textProp/sliderProp/itemList. Clicking a page name pushes the new index
// into pageProp; a permanent subscription in main (installed once, never
// canceled, since it isn't tied to any one built tree) sets a pending flag
// that main's frame loop notices and rebuilds from — the same
// cancel-old-bindings-then-build()-again discipline as the T-key toggle,
// just triggered by a different pending flag. Page 0 ("Controls") is
// Phase 5/6's existing scroll content, moved into buildControlsPage
// unchanged; page 1 ("Advanced", the startup default so the gallery opens
// showing the newest work) is buildAdvancedPage: a MenuBar (File/Edit, the
// latter with a submenu) above a 3-tab TabControl — List (a 30-row
// ListView plus a selected-index TextBlock, again via bind.ListSelected),
// Tree (a small nested TreeView beside an Expander showing the selected
// node's detail), and Data (a 3-column/50-row DataGrid plus a button that
// fires ShowDialog, its result mirrored into a TextBlock via bind.OneWay).
// ListView is fluo's one disposable control (see ListView.Dispose's doc
// comment): every ListView buildUI creates — the nav switcher and the List
// tab's — has its Dispose appended to *cancels alongside the ordinary
// binder cancels, so a stale items-channel subscription never outlives its
// tree. DataGrid.Dispose is a no-op but is called too, for the same reason
// its own doc comment gives: uniform Dispose-everything-virtualized in the
// cancel path, no type switch needed.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/0xdreadnaught/fluo/app"
	"github.com/0xdreadnaught/fluo/bind"
	"github.com/0xdreadnaught/fluo/controls"
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/text"
	"github.com/0xdreadnaught/fluo/theme"
	"github.com/0xdreadnaught/fluo/timers"

	"golang.org/x/image/font/gofont/goregular"
)

// swatch is a color block that reacts to pointer hover/press: it embeds
// core.Element for layout, implements input.PointerHandler to track
// hover/selected state, input.Focusable so it participates in tab order and
// press-to-focus, and input.CursorShaper to show a hand cursor. It exists
// here (rather than in package controls) as the consumer-side example of
// wiring a widget up to fluo's event API. The color it fills with is DATA
// (one of the six swatch-palette samples, not a theme token); the ring and
// hover-stroke colors it draws ARE tokens, captured from theme.Active() at
// construction by buildUI so a theme toggle recolors them on rebuild.
type swatch struct {
	core.Element

	w, h       float32
	color      render.Color
	ringColor  render.Color // selected-state stroke (th.Color.Accent)
	hoverColor render.Color // hover-state stroke (th.Color.TextPrimary)
	hover      bool
	selected   bool
}

// newSwatch returns a swatch that measures to (w, h), fills with c, and
// strokes its selected/hover rings with ring/hoverColor.
func newSwatch(w, h float32, c, ring, hoverColor render.Color) *swatch {
	return &swatch{w: w, h: h, color: c, ringColor: ring, hoverColor: hoverColor}
}

// MeasureContent returns the fixed (w, h) size regardless of the space
// available.
func (s *swatch) MeasureContent(available render.Size) render.Size {
	return render.Size{W: s.w, H: s.h}
}

// Render fills the color block, then draws a 2px ring stroke at the
// swatch's own bounds when selected, and a 2px stroke inset 3px from those
// bounds when hovered — the inset keeps the two strokes from overdrawing
// each other, so a swatch that is both hovered and selected shows both
// rings distinctly instead of one overpainting the other.
func (s *swatch) Render(r render.Renderer) {
	b := s.Bounds()
	if s.color.A != 0 {
		r.FillRect(b, s.color)
	}
	if s.selected {
		r.StrokeRoundedRect(b, 0, 2, s.ringColor)
	}
	if s.hover {
		r.StrokeRoundedRect(b.Inset(render.Uniform(3)), 0, 2, s.hoverColor)
	}
}

// OnPointer implements input.PointerHandler: Enter/Leave toggle hover,
// Press toggles selected and is marked handled.
func (s *swatch) OnPointer(e *input.PointerEvent) {
	switch e.Action {
	case input.Enter:
		s.hover = true
	case input.Leave:
		s.hover = false
	case input.Press:
		s.selected = !s.selected
		e.Handled = true
	}
}

// AcceptsFocus implements input.Focusable: swatches join tab order and can
// be press-to-focused.
func (s *swatch) AcceptsFocus() bool { return true }

// Cursor implements input.CursorShaper: a hand cursor over any swatch.
func (s *swatch) Cursor() input.Cursor { return input.CursorHand }

// galleryRoot is a trivial single-child wrapper (same shape as
// controls.Border) around the real DockPanel tree, whose only job is
// implementing input.KeyHandler for the theme-toggle shortcut. It must be an
// actual node in the tree recorded as its child's ancestor (via
// core.SetParent in newGalleryRoot) — not a promoted-method embed of
// *DockPanel — so that input.Router's key-bubbling (which walks
// core.ParentOf from whichever widget holds focus) reaches OnKey whenever
// something under the dock is focused.
//
// Since Phase 5 Task 9, galleryRoot is no longer the router's own root: it
// sits as buildUI's OverlayHost's content, one level below the host (see
// buildUI). input.Router.dispatchKey delivers to the focused widget's own
// core.ParentOf chain (which still includes galleryRoot, and above it the
// host) whenever something is focused, and content sits on that chain, so
// the T-key toggle reaches galleryRoot's OnKey via the ordinary bubble in
// that case. With NOTHING focused (e.g. immediately after SetRoot, which
// always clears focus), dispatchKey would otherwise deliver only to the
// bare router root — the OverlayHost — alone; but OverlayHost.OnKey DOES
// delegate an unfocused key to its own content whenever content implements
// input.KeyHandler, which galleryRoot does (see OverlayHost.OnKey's own doc
// comment). So the T-key toggle fires reliably even from a pristine,
// nothing-yet-focused launch — the single-OverlayHost-root tradeoff noted
// here in earlier revisions of this comment no longer applies.
type galleryRoot struct {
	core.Element

	child  core.Widget
	toggle func()
}

// newGalleryRoot wraps child, re-parenting it, and calls toggle on KeyT.
func newGalleryRoot(child core.Widget, toggle func()) *galleryRoot {
	g := &galleryRoot{child: child, toggle: toggle}
	core.SetParent(child, g)
	return g
}

func (g *galleryRoot) MeasureContent(available render.Size) render.Size {
	core.MeasureWidget(g.child, available)
	return core.DesiredSizeOf(g.child)
}

func (g *galleryRoot) ArrangeContent(bounds render.Rect) {
	core.ArrangeWidget(g.child, bounds)
}

func (g *galleryRoot) Children() []core.Widget { return []core.Widget{g.child} }

// OnKey implements input.KeyHandler: an unmodified KeyT press-down toggles
// the active theme. The actual SetActive/rebuild/SetRoot happens in main's
// frame callback on the NEXT frame (a pending flag set here), not inline —
// see toggle's construction in main for why.
func (g *galleryRoot) OnKey(e *input.KeyEvent) {
	if e.Action == input.Press && e.Key == input.KeyT && e.Mods == 0 {
		g.toggle()
		e.Handled = true
	}
}

// swatchPalette is DATA: sample colors shown in the gallery's color-swatch
// row, not theme chrome, so (unlike everything else buildUI draws) these
// stay literal across a theme toggle.
var swatchPalette = []render.Color{
	render.RGB(0, 120, 215), render.RGB(255, 185, 0), render.RGB(16, 124, 16),
	render.RGB(232, 17, 35), render.RGB(136, 23, 152), render.RGB(0, 153, 188),
}

// swatchColorNames names swatchPalette's entries in the same order, reused
// verbatim as the Controls section's ComboBox items — the gallery's one
// piece of "data that appears in two widgets", kept as a single slice so the
// two can never drift out of sync.
var swatchColorNames = []string{"Blue", "Yellow", "Green", "Red", "Purple", "Teal"}

// buildControlsSection builds the Controls section: one HStack per control
// family (Button/ToggleButton, CheckBox/RadioButton+Group/ToggleSwitch,
// TextBox/ComboBox, Slider/ProgressBar) plus a Phase 6 Binding demo (text
// mirror, property-driven slider/progress, list-backed StackPanel), stacked
// vertically with th's PaddingM gap, all styled from th like everything else
// buildUI draws. tq (may be nil, e.g. before the app's first frame hands
// buildUI a real timers.Queue) is threaded into the TextBox's caret blink,
// the accent button's ToolTipArea dwell timer, and (Phase 8) every
// Button-family widget's SetAnimated(true) cross-fade via their respective
// SetTimers — nil disables the timing behavior but leaves every control
// otherwise functional (solid caret, immediate-show tooltip, instant-snap
// fill instead of a cross-fade), matching each control's own documented
// no-queue convention. counter/onToggle wire up the
// demo button's click count and the theme-toggle shortcut respectively
// (onToggle is consumed by galleryRoot, not here). textProp/sliderProp/
// itemList are the Phase 6 binding models, constructed once in main and
// threaded through every rebuild — every binder created here appends its
// cancel func to *cancels so the caller can tear the OLD tree's bindings
// down before the next buildUI call creates fresh ones on the new tree.
func buildControlsSection(th *theme.Theme, body *text.Face, counter *int, tq *timers.Queue, textProp *core.Property[string], sliderProp *core.Property[float32], itemList *bind.List[string], cancels *[]func()) core.Widget {
	// Row 1: Button (with click counter), accent Button (tooltipped),
	// ToggleButton.
	counterLabel := controls.NewTextBlock(body, fmt.Sprintf("Clicked %d times", *counter)).
		SetColor(th.Color.TextSecondary)
	demoButton := controls.NewButton(body, "Click me").OnClick(func() {
		*counter++
		counterLabel.SetText(fmt.Sprintf("Clicked %d times", *counter))
	})
	demoButton.SetAnimated(true).SetTimers(tq)
	accentButton := controls.NewButton(body, "Accent").SetAccent(true)
	accentButton.SetAnimated(true).SetTimers(tq)
	accentTip := controls.NewToolTipArea(accentButton, body, "Accent button").SetTimers(tq)
	toggleButton := controls.NewToggleButton(body, "Toggle")
	toggleButton.SetAnimated(true).SetTimers(tq)

	row1 := controls.NewStackPanel(controls.Horizontal).SetGap(th.Metric.PaddingM).
		Add(demoButton, counterLabel, accentTip, toggleButton)

	// Row 2: CheckBox, RadioButton "A"+"B" in a RadioGroup, ToggleSwitch.
	checkBox := controls.NewCheckBox(body, "Enable")

	radioA := controls.NewRadioButton(body, "A")
	radioB := controls.NewRadioButton(body, "B")
	controls.NewRadioGroup().Add(radioA).Add(radioB)

	toggleSwitch := controls.NewToggleSwitch()

	row2 := controls.NewStackPanel(controls.Horizontal).SetGap(th.Metric.PaddingM).
		Add(checkBox, radioA, radioB, toggleSwitch)

	// Row 3: TextBox (placeholder + caret blink) two-way bound to textProp,
	// a mirror TextBlock one-way bound to the same property (so typing in
	// the TextBox updates the mirror live), and ComboBox (swatch color
	// names, not yet bound).
	textBox := controls.NewTextBox(body).SetPlaceholder("Type here…").SetTimers(tq)
	textBox.SetWidth(200)
	mirror := controls.NewTextBlock(body, "").SetColor(th.Color.TextSecondary)
	comboBox := controls.NewComboBox(body).SetItems(swatchColorNames)

	*cancels = append(*cancels,
		bind.Text(textProp, textBox),
		bind.OneWay(textProp, func(s string) { mirror.SetText(s) }),
	)

	row3 := controls.NewStackPanel(controls.Horizontal).SetGap(th.Metric.PaddingM).
		Add(textBox, mirror, comboBox)

	// Row 4: Slider two-way bound to sliderProp, ProgressBar one-way bound
	// to the same property — replacing Phase 5's direct
	// slider.OnChanged(progressBar.SetValue) wiring. bind.Value seeds the
	// Slider from sliderProp.Get() and bind.OneWay seeds the ProgressBar the
	// same way, so both widgets start in sync with the model with no manual
	// SetValue seeding needed here.
	progressBar := controls.NewProgressBar()
	slider := controls.NewSlider()

	*cancels = append(*cancels,
		bind.Value(sliderProp, slider),
		bind.OneWay(sliderProp, func(v float32) { progressBar.SetValue(v) }),
	)

	row4 := controls.NewStackPanel(controls.Horizontal).SetGap(th.Metric.PaddingM).
		Add(slider, progressBar)

	// Row 5: ObservableList demo. An "Add" Button appends "Item N" to
	// itemList; bind.Items keeps listPanel's TextBlock children in sync with
	// itemList's contents via a full Clear+rebuild on every change (v0 —
	// virtualization is Phase 7). itemList.Len() is read fresh from the
	// model on every click, so the numbering stays correct even across a
	// theme-toggle rebuild (itemList itself is never recreated).
	listPanel := controls.NewStackPanel(controls.Vertical).SetGap(th.Metric.PaddingS)
	*cancels = append(*cancels, bind.Items(itemList, listPanel, func(item string, _ int) core.Widget {
		return controls.NewTextBlock(body, item).SetColor(th.Color.TextSecondary)
	}))

	addButton := controls.NewButton(body, "Add").OnClick(func() {
		itemList.Add(fmt.Sprintf("Item %d", itemList.Len()+1))
	})
	addButton.SetAnimated(true).SetTimers(tq)
	// Left-align to natural width: row5 is a vertical stack, so without this
	// the button stretches to the full content width.
	addButton.SetAlign(core.Start, core.Start)

	row5 := controls.NewStackPanel(controls.Vertical).SetGap(th.Metric.PaddingS).
		Add(addButton, listPanel)

	return controls.NewStackPanel(controls.Vertical).SetGap(th.Metric.PaddingM).
		Add(row1, row2, row3, row4, row5)
}

// buildControlsPage builds page 0's content: the Phase 5/6 Controls section
// plus the swatch WrapPanel plus a 20-row filler stack, all inside a
// ScrollViewer — pulled out of buildUI unchanged (byte-for-byte the same
// tree it always built) so buildUI can choose between it and
// buildAdvancedPage per the current page selection. See buildControlsSection
// and buildUI's own doc comments for the params threaded through.
func buildControlsPage(th *theme.Theme, body *text.Face, counter *int, tq *timers.Queue, textProp *core.Property[string], sliderProp *core.Property[float32], itemList *bind.List[string], cancels *[]func()) core.Widget {
	swatches := controls.NewWrapPanel().SetGap(th.Metric.PaddingM)
	for _, c := range swatchPalette {
		swatches.Add(newSwatch(72, 48, c, th.Color.Accent, th.Color.TextPrimary))
	}

	controlsSection := buildControlsSection(th, body, counter, tq, textProp, sliderProp, itemList, cancels)

	content := controls.NewStackPanel(controls.Vertical).SetGap(th.Metric.PaddingM).
		Add(controlsSection, swatches)
	for i := 1; i <= 20; i++ {
		content.Add(controls.NewTextBlock(body, fmt.Sprintf("Row %02d", i)).SetColor(th.Color.TextSecondary))
	}

	return controls.NewScrollViewer().SetChild(content)
}

// advancedTreeSample builds the small nested TreeView sample the Tree tab
// shows: two roots, one of them ("Pictures") two levels deep — built fresh
// on every buildAdvancedPage call (v0: this demo tree carries no state that
// needs to survive a rebuild the way textProp/itemList do), with "Pictures"
// and its "Vacation" child pre-expanded so the nesting is visible without
// any clicking, which matters for the Task 9 startup screenshot.
func advancedTreeSample() []*controls.TreeNode {
	docs := controls.NewTreeNode("Documents",
		controls.NewTreeNode("Report.docx"),
		controls.NewTreeNode("Notes.txt"),
	)
	vacation := controls.NewTreeNode("Vacation",
		controls.NewTreeNode("beach.jpg"),
		controls.NewTreeNode("hike.jpg"),
	)
	pictures := controls.NewTreeNode("Pictures", vacation, controls.NewTreeNode("Screenshot.png"))
	pictures.SetExpanded(true)
	vacation.SetExpanded(true)
	return []*controls.TreeNode{docs, pictures}
}

// buildAdvancedPage builds page 1's content: a MenuBar (File → New/Open/
// separator/Exit, Edit → a submenu demo) above a 3-tab TabControl —
// List/Tree/Data — exercising every Phase 7 control. host is threaded
// through so the Data tab's "Show dialog" Button can call ShowDialog
// directly (ShowDialog takes a *controls.OverlayHost, not something
// OverlayHostFor can resolve from a not-yet-attached button). selectedProp/
// dialogResultProp are owned by main and outlive every rebuild — like
// textProp/sliderProp/itemList on the Controls page — so the List tab's
// selection and the last dialog result both survive a theme toggle or a
// switch away to the Controls page and back; every ListView's items
// (the 30-row nav-irrelevant list) and the TreeView/Expander/DataGrid demo
// data are cheap to rebuild fresh each call and carry no state worth
// persisting. cancels collects every binder's cancel func AND every
// disposable control's Dispose (ListView, DataGrid — see the package doc
// comment above), exactly like buildControlsSection.
func buildAdvancedPage(th *theme.Theme, body *text.Face, host *controls.OverlayHost, selectedProp *core.Property[int], dialogResultProp *core.Property[string], cancels *[]func()) core.Widget {
	menuBar := controls.NewMenuBar(body)

	fileMenu := menuBar.AddMenu("File")
	fileMenu.Add("New", func() { log.Println("gallery: File > New") })
	fileMenu.Add("Open", func() { log.Println("gallery: File > Open") })
	fileMenu.AddSeparator()
	fileMenu.Add("Exit", func() {}) // no-op, per the brief

	editMenu := menuBar.AddMenu("Edit")
	editMenu.Add("Cut", func() { log.Println("gallery: Edit > Cut") })
	editMenu.Add("Copy", func() { log.Println("gallery: Edit > Copy") })
	editMenu.AddSub("Paste Special").
		Add("Values", func() { log.Println("gallery: Edit > Paste Special > Values") }).
		Add("Formulas", func() { log.Println("gallery: Edit > Paste Special > Formulas") })

	// Tab 1 "List": a 30-row ListView two-way bound to selectedProp (via
	// bind.ListSelected) plus a mirror TextBlock, one-way bound to the same
	// property, showing the selected index as plain text. The row data
	// itself ("Row 0".."Row 29") is rebuilt fresh every call — only the
	// SELECTION persists across rebuilds, via selectedProp.
	rows := make([]string, 30)
	for i := range rows {
		rows[i] = fmt.Sprintf("Row %d", i)
	}
	rowsList := bind.NewList(rows...)
	listView := controls.NewListView(body, rowsList)
	listView.SetWidth(160)
	listView.SetHeight(200)
	selectedText := controls.NewTextBlock(body, fmt.Sprintf("Selected: %d", selectedProp.Get())).
		SetColor(th.Color.TextSecondary)

	*cancels = append(*cancels,
		bind.ListSelected(selectedProp, listView),
		listView.Dispose,
		bind.OneWay(selectedProp, func(v int) { selectedText.SetText(fmt.Sprintf("Selected: %d", v)) }),
	)

	tab1 := controls.NewStackPanel(controls.Horizontal).SetGap(th.Metric.PaddingM).
		Add(listView, selectedText)

	// Tab 2 "Tree": a small nested TreeView beside an Expander("Details")
	// whose content TextBlock mirrors whichever node is currently selected
	// — plain field wiring (TreeView.OnChanged), not a binder, since
	// TreeView owns no external resource needing a cancel (see
	// ListView.Dispose's doc comment: ListView is fluo's ONE disposable
	// control in v0).
	detailText := controls.NewTextBlock(body, "Select a node to see its details.").
		SetColor(th.Color.TextSecondary)
	expander := controls.NewExpander(body, "Details").SetContent(detailText)
	expander.SetExpanded(true) // so Details is visible without a click, for the screenshot

	treeView := controls.NewTreeView(body, advancedTreeSample()...)
	treeView.OnChanged(func(n *controls.TreeNode) {
		if n != nil {
			detailText.SetText(fmt.Sprintf("Selected: %s", n.Label))
		}
	})

	tab2 := controls.NewStackPanel(controls.Horizontal).SetGap(th.Metric.PaddingM).
		Add(treeView, expander)

	// Tab 3 "Data": a 3-column (Px/Star/Px), 50-row DataGrid plus a "Show
	// dialog" Button firing ShowDialog, whose result is mirrored into a
	// TextBlock via bind.OneWay onto dialogResultProp (so the result
	// survives a rebuild exactly like selectedProp does above).
	grid := controls.NewDataGrid(body)
	grid.SetColumns(
		controls.Column{Title: "ID", Width: controls.Px(60), Value: func(row int) string {
			return fmt.Sprintf("%d", row)
		}},
		controls.Column{Title: "Name", Width: controls.Star(1), Value: func(row int) string {
			return fmt.Sprintf("Item %d", row)
		}},
		controls.Column{Title: "Qty", Width: controls.Px(60), Value: func(row int) string {
			return fmt.Sprintf("%d", (row*7)%100)
		}},
	)
	grid.SetRowCount(50)
	*cancels = append(*cancels, grid.Dispose)

	resultText := controls.NewTextBlock(body, dialogResultProp.Get()).SetColor(th.Color.TextSecondary)
	*cancels = append(*cancels, bind.OneWay(dialogResultProp, func(s string) { resultText.SetText(s) }))

	showDialogButton := controls.NewButton(body, "Show dialog").OnClick(func() {
		controls.ShowDialog(host, body, controls.DialogSpec{
			Title:     "Confirm",
			Body:      "Proceed with this action?",
			Primary:   "OK",
			Secondary: "Cancel",
			OnResult: func(r controls.DialogResult) {
				var s string
				switch r {
				case controls.DialogPrimary:
					s = "Primary"
				case controls.DialogSecondary:
					s = "Secondary"
				case controls.DialogDismissed:
					s = "Dismissed"
				}
				dialogResultProp.Set(fmt.Sprintf("Result: %s", s))
			},
		})
	})

	buttonRow := controls.NewStackPanel(controls.Horizontal).SetGap(th.Metric.PaddingM).
		Add(showDialogButton, resultText)

	tab3 := controls.NewStackPanel(controls.Vertical).SetGap(th.Metric.PaddingM).
		Add(grid, buttonRow)

	tabs := controls.NewTabControl(body).
		AddTab("List", tab1).
		AddTab("Tree", tab2).
		AddTab("Data", tab3)

	stack := controls.NewStackPanel(controls.Vertical).SetGap(th.Metric.PaddingM).
		Add(menuBar, tabs)

	return controls.NewScrollViewer().SetChild(stack)
}

// buildUI builds the gallery's entire widget tree from th's tokens — colors,
// paddings, radii, and type sizes all come from th, so the whole tree is a
// pure function of the active theme (see FLUO_THEME and the T-key toggle in
// main: re-theming means calling buildUI again and swapping roots, never
// mutating an existing tree in place) AND of pageProp's current value (see
// the package doc comment's Phase 7 paragraph) — a page switch rebuilds via
// the exact same mechanism a theme toggle does. counter/onToggle wire up the
// demo button's click count and the theme-toggle shortcut respectively; tq
// is forwarded to buildControlsSection (see its doc comment).
//
// Since Phase 5 Task 9, the returned root is a *controls.OverlayHost (rather
// than *galleryRoot directly): ComboBox's popup and ToolTipArea's tip (both
// used in the Controls section) need an OverlayHost ancestor to render into
// (OverlayHostFor walks up looking for one), so the host must sit above
// everything that uses either control — see main's SetRouter wiring, which
// an OverlayHost needs to drive its light-dismiss capture. Phase 7's
// Advanced page needs the SAME host explicitly (its Data tab's "Show
// dialog" Button calls controls.ShowDialog(host, ...) directly), so host is
// constructed first, here, and threaded into buildAdvancedPage before
// SetContent installs the finished tree onto it at the end.
//
// textProp/sliderProp/itemList are the Phase 6 binding models; pageProp/
// advSelectedProp/advDialogResultProp are Phase 7's (see buildAdvancedPage's
// doc comment) — all owned by main, outliving every rebuild. cancels
// collects every binder's cancel func AND every disposable control's
// Dispose created while building this tree — see buildControlsSection's and
// buildAdvancedPage's doc comments, and main's rebuild paths (T-key toggle,
// page-change), both of which cancel the OLD tree's bindings before calling
// buildUI again.
//
// Phase 8 replaces the old plain-Border title strip with a
// controls.TitleBar (see the package doc comment's Phase 8 paragraph):
// onMinimize/onMaximize/onClose are wired straight to the host's
// Ctx.Minimize/Ctx.ToggleMaximize/Ctx.Close (main captures those once, on
// the app's first frame, and passes the same three func values into every
// later rebuild — they never change across frames). buildUI now returns the
// fresh *controls.TitleBar alongside the host so main can keep a live
// reference for its per-frame DragRegion press-edge check (dragging itself
// is not this function's concern; see the package doc comment). The content
// pane is now a controls.AcrylicSurface rather than a flat WindowBackground
// Border, so the translucent backdrop shows through behind pageContent.
func buildUI(th *theme.Theme, font *text.Font, counter *int, onToggle func(), tq *timers.Queue, textProp *core.Property[string], sliderProp *core.Property[float32], itemList *bind.List[string], pageProp *core.Property[int], advSelectedProp *core.Property[int], advDialogResultProp *core.Property[string], onMinimize, onMaximize, onClose func(), cancels *[]func()) (*controls.OverlayHost, *controls.TitleBar) {
	title := text.NewFace(font, th.Type.SubtitleSize)
	body := text.NewFace(font, th.Type.BodySize)

	host := controls.NewOverlayHost()

	// nav: a 2-row ListView of page names ("Controls"/"Advanced"), two-way
	// bound to pageProp via bind.ListSelected — see the package doc
	// comment's Phase 7 paragraph for the full page-switch mechanism this
	// feeds into. navItems is rebuilt fresh every call (nothing ever
	// mutates it after construction, so there's no state worth persisting
	// beyond the SELECTION itself, which lives in pageProp).
	navItems := bind.NewList("Controls", "Advanced")
	navList := controls.NewListView(body, navItems)
	// No fixed width: let the list fill the sidebar column so its selection
	// band reaches the right edge too (the sidebar Border sets the width).
	navList.SetHeight(64)
	*cancels = append(*cancels,
		bind.ListSelected(pageProp, navList),
		navList.Dispose,
	)

	var pageContent core.Widget
	if pageProp.Get() == 0 {
		pageContent = buildControlsPage(th, body, counter, tq, textProp, sliderProp, itemList, cancels)
	} else {
		pageContent = buildAdvancedPage(th, body, host, advSelectedProp, advDialogResultProp, cancels)
	}

	titleBar := controls.NewTitleBar(title, "fluo gallery").
		OnMinimize(onMinimize).
		OnMaximize(onMaximize).
		OnClose(onClose)

	dock := controls.NewDockPanel().
		Add(titleBar, controls.DockTop).
		Add(func() core.Widget {
			// Fixed-width sidebar panel; vertical padding only so the nav
			// ListView fills the column width and its selection band reaches
			// both edges (row text stays inset by the ListView's own row
			// padding).
			b := controls.NewBorder().
				SetBackground(th.Color.LayerBackground).
				SetPadding(render.Thickness{Top: th.Metric.PaddingS, Bottom: th.Metric.PaddingS}).
				SetChild(navList)
			b.SetWidth(150)
			return b
		}(),
			controls.DockLeft).
		Add(controls.NewAcrylicSurface().
			SetRadius(th.Metric.CornerRadius).
			SetPadding(render.Uniform(th.Metric.PaddingL)).
			SetChild(pageContent),
			controls.DockLeft) // last child fills

	host.SetContent(newGalleryRoot(dock, onToggle))
	return host, titleBar
}

func main() {
	f, err := text.Load(goregular.TTF)
	if err != nil {
		log.Fatal(err)
	}

	// FLUO_THEME is a dev convenience for manual light/dark verification
	// (e.g. `FLUO_THEME=light go run ./cmd/fluo-gallery`), not a supported
	// runtime API — the real toggle is the T key.
	initial := theme.Dark()
	if os.Getenv("FLUO_THEME") == "light" {
		initial = theme.Light()
	}
	theme.SetActive(initial)

	var counter int
	var togglePending bool
	// timerQueue starts nil (no app.Ctx exists yet to supply one) and is
	// filled in from the first frame's c.Timers below — see buildControlsSection's
	// doc comment for why a nil queue is a safe, if less lively, fallback.
	var timerQueue *timers.Queue

	// winMinimize/winToggleMax/winClose are captured, once, from the app's
	// first frame's Ctx.Minimize/Ctx.ToggleMaximize/Ctx.Close — those three
	// Ctx fields wrap the SAME closures (minimizeFn/toggleMaximizeFn/
	// closeFn in app/window.go's Run) on every frame, so capturing them once
	// here and threading them into every build() call (rather than
	// re-reading them from Ctx on every rebuild) is safe and lets build's
	// signature stay a plain closure over local state, matching timerQueue's
	// own capture-once convention just above.
	var winMinimize, winToggleMax, winClose func()

	// Phase 6 binding models: constructed once here, so they outlive every
	// theme-toggle rebuild below (buildUI only ever builds fresh WIDGETS and
	// fresh BINDINGS onto these same three models — never fresh models).
	textProp := new(core.Property[string])
	textProp.Set("Hello, fluo!")
	sliderProp := new(core.Property[float32])
	sliderProp.Set(0.3)
	itemList := bind.NewList[string]()

	// Phase 7 binding models, same "constructed once, outlives every
	// rebuild" convention as the three above. pageProp drives buildUI's
	// page switch (see its own doc comment's Phase 7 paragraph); it starts
	// at 1 ("Advanced") per the Task 9 brief, so the gallery opens showing
	// the newest work. pagePending mirrors togglePending below, but is set
	// from a PERMANENT subscription on pageProp (installed once, right
	// here, never canceled — it isn't owned by any one built tree, unlike
	// the binder cancels buildUI/buildAdvancedPage collect into cancels)
	// rather than from a widget callback, since the page-switching ListView
	// itself is rebuilt on every page change and so cannot be the sole
	// owner of a flag that must survive across those rebuilds.
	// advSelectedProp/advDialogResultProp are the Advanced page's own
	// persistent models — see buildAdvancedPage's doc comment.
	pageProp := new(core.Property[int])
	pageProp.Set(1)
	var pagePending bool
	pageProp.OnChange(func(_, _ int) { pagePending = true })
	advSelectedProp := new(core.Property[int])
	advSelectedProp.Set(-1)
	advDialogResultProp := new(core.Property[string])
	advDialogResultProp.Set("Result: (none)")

	// cancels collects every binder's cancel func from the CURRENTLY BUILT
	// tree (buildUI/buildControlsSection append to it). The theme toggle
	// below calls each cancel and resets the slice before calling build()
	// again, so the OLD tree's OnChanged ownership and property
	// subscriptions are fully torn down before the new tree installs its own
	// — otherwise two trees' worth of binders would both be live, with the
	// older one silently fighting the newer one for OnChanged ownership.
	var cancels []func()
	build := func() (*controls.OverlayHost, *controls.TitleBar) {
		return buildUI(theme.Active(), f, &counter, func() { togglePending = true }, timerQueue, textProp, sliderProp, itemList, pageProp, advSelectedProp, advDialogResultProp, winMinimize, winToggleMax, winClose, &cancels)
	}

	var root *controls.OverlayHost
	var titleBar *controls.TitleBar
	var lastSize render.Size
	rootSet := false
	// mouseWasDown tracks last frame's Ctx.Mouse.Down so the titlebar-drag
	// check below fires BeginDrag exactly once per press (on the down-edge),
	// never on every frame a press stays held — see the package doc
	// comment's Phase 8 paragraph for why re-arming every frame would break
	// the drag entirely.
	var mouseWasDown bool

	err = app.Run(app.Config{Title: "fluo gallery", Width: 640, Height: 420, Undecorated: true}, func(c *app.Ctx) {
		if !rootSet {
			timerQueue = c.Timers
			winMinimize, winToggleMax, winClose = c.Minimize, c.ToggleMaximize, c.Close
			root, titleBar = build()
			c.Input.SetRoot(root)
			root.SetRouter(c.Input) // OverlayHost needs the router for light-dismiss capture
			rootSet = true
		}
		if togglePending {
			togglePending = false
			next := theme.Light()
			if theme.Active().Name == "classic-light" {
				next = theme.Dark()
			}
			theme.SetActive(next)
			// Cancel every binding on the OLD tree (about to be discarded)
			// before build() creates the new tree's own bindings on the same
			// models — see cancels' doc comment above.
			for _, cancel := range cancels {
				cancel()
			}
			cancels = cancels[:0]
			root, titleBar = build()
			c.Input.SetRoot(root) // SetRoot resets hover/capture/focus by design
			root.SetRouter(c.Input)
			lastSize = render.Size{}
			// This rebuild already reflects pageProp's current value (build()
			// reads it fresh via buildUI), so a page change coincidentally
			// pending on the SAME frame as a theme toggle needs no second
			// rebuild.
			pagePending = false
		}
		if pagePending {
			pagePending = false
			for _, cancel := range cancels {
				cancel()
			}
			cancels = cancels[:0]
			root, titleBar = build()
			c.Input.SetRoot(root)
			root.SetRouter(c.Input)
			lastSize = render.Size{}
		}
		if c.Size != lastSize || root.NeedsLayout() {
			lastSize = c.Size
			core.MeasureWidget(root, c.Size)
			core.ArrangeWidget(root, render.Rect{X: 0, Y: 0, W: c.Size.W, H: c.Size.H})
		}
		// Titlebar drag: see mouseWasDown's doc comment above. titleBar's
		// DragRegion excludes the three caption buttons on its own (see
		// TitleBar.DragRegion), so a press on Minimize/Maximize/Close never
		// reaches here as a drag start — it's delivered to the router like
		// any other click, via the ordinary glfw button callback in
		// app/window.go.
		pressed := c.Mouse.Down
		if pressed && !mouseWasDown && titleBar.DragRegion(c.Mouse.Pos) {
			c.BeginDrag()
		}
		mouseWasDown = pressed
		core.RenderWidget(root, c.R)
	})
	if err != nil {
		log.Fatal(err)
	}
}
