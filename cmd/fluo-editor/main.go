// Command fluo-editor is a standalone code-editor demo built on fluo's
// multi-line TextBox: a monospace editor with a line-number gutter, word-wrap
// OFF (long lines scroll horizontally), tab-to-indent, mouse-wheel scrolling,
// and a live Ln/Col status. It is the sandbox for working out editor features
// in fluo before handing them to consumers.
package main

import (
	"fmt"
	"log"

	"github.com/0xdreadnaught/fluo/app"
	"github.com/0xdreadnaught/fluo/controls"
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/text"
	"github.com/0xdreadnaught/fluo/theme"

	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/goregular"
)

// sample is a short seed document — tab-indented, with one line long enough to
// exercise word-wrap-off horizontal scrolling.
const sample = `package main

import "fmt"

func main() {
	msg := "hello from the fluo code editor — a long line to test horizontal scrolling"
	for i := 0; i < 3; i++ {
		fmt.Println(i, msg)
	}
}
`

// lineCol converts a rune-index caret to a 1-based line/column, matching what
// the editor draws.
func lineCol(s string, caret int) (line, col int) {
	runes := []rune(s)
	if caret > len(runes) {
		caret = len(runes)
	}
	if caret < 0 {
		caret = 0
	}
	line, col = 1, 1
	for i := 0; i < caret; i++ {
		if runes[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return line, col
}

func countLines(s string) int {
	n := 1
	for _, r := range s {
		if r == '\n' {
			n++
		}
	}
	return n
}

func main() {
	uiFont, err := text.Load(goregular.TTF)
	if err != nil {
		log.Fatal(err)
	}
	monoFont, err := text.Load(gomono.TTF)
	if err != nil {
		log.Fatal(err)
	}
	th := theme.Active()
	ui := text.NewFace(uiFont, th.Type.BodySize)
	mono := text.NewFace(monoFont, th.Type.BodySize)

	// The editor: multi-line, line numbers on, word-wrap OFF (horizontal
	// scroll), tab inserts a tab. SetCaret(0) after SetText opens it at the
	// TOP — SetText parks the caret at the end, which would otherwise open the
	// box scrolled to the last line.
	editor := controls.NewTextBox(mono).SetMultiline(true)
	editor.SetLineNumbers(true)
	editor.SetTabInserts(true)
	editor.SetText(sample)
	editor.SetCaret(0)

	title := controls.NewTextBlock(ui, "fluo — code editor demo").SetColor(th.Color.WindowText)
	status := controls.NewTextBlock(ui, "").SetColor(th.Color.GrayText)

	newBtn := controls.NewButton(ui, "New").OnClick(func() {
		editor.SetText("")
		editor.SetCaret(0)
	})
	sampleBtn := controls.NewButton(ui, "Sample").OnClick(func() {
		editor.SetText(sample)
		editor.SetCaret(0)
	})

	wrap := false
	wrapBtn := controls.NewButton(ui, "Toggle wrap").OnClick(func() {
		wrap = !wrap
		editor.SetWordWrap(wrap)
	})
	gutter := true
	gutterBtn := controls.NewButton(ui, "Toggle line #").OnClick(func() {
		gutter = !gutter
		editor.SetLineNumbers(gutter)
	})

	header := controls.NewStackPanel(controls.Horizontal).
		SetGap(th.Metric.PaddingM).
		Add(title, newBtn, sampleBtn, wrapBtn, gutterBtn, status)

	grid := controls.NewGrid().Rows(controls.AutoTrack(), controls.Star(1))
	grid.Add(header, 0, 0)
	grid.Add(editor, 1, 0)

	root := controls.NewBorder().
		SetPadding(render.Uniform(th.Metric.PaddingL)).
		SetChild(grid)

	var lastSize render.Size
	var lastStatus string
	rootSet := false

	err = app.Run(app.Config{Title: "fluo code editor", Width: 900, Height: 620}, func(c *app.Ctx) {
		if !rootSet {
			c.Input.SetRoot(root)
			rootSet = true
		}

		// Live status (caret moves via click/arrows without firing OnChanged,
		// so recompute each frame; SetText is gated on change to avoid churn).
		txt := editor.Text()
		ln, col := lineCol(txt, editor.Caret())
		wrapS, gutS := "off", "off"
		if wrap {
			wrapS = "on"
		}
		if gutter {
			gutS = "on"
		}
		s := fmt.Sprintf("Ln %d, Col %d  ·  %d lines  ·  wrap:%s  ·  gutter:%s", ln, col, countLines(txt), wrapS, gutS)
		if s != lastStatus {
			status.SetText(s)
			lastStatus = s
		}

		if c.Size != lastSize || root.NeedsLayout() {
			lastSize = c.Size
			core.MeasureWidget(root, c.Size)
			core.ArrangeWidget(root, render.Rect{X: 0, Y: 0, W: c.Size.W, H: c.Size.H})
		}
		core.RenderWidget(root, c.R)
	})
	if err != nil {
		log.Fatal(err)
	}
}
