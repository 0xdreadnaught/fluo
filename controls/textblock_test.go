package controls

import (
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/text"

	"golang.org/x/image/font/gofont/goregular"
)

func TestTextBlockMeasuresText(t *testing.T) {
	f, err := text.Load(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	face := text.NewFace(f, 14)
	tb := NewTextBlock(face, "Hello")
	core.MeasureWidget(tb, render.Size{W: 500, H: 500})
	want := face.Measure("Hello")
	if got := tb.DesiredSize(); got != want {
		t.Fatalf("desired=%v want %v", got, want)
	}
}

func TestTextBlockSetTextInvalidates(t *testing.T) {
	f, _ := text.Load(goregular.TTF)
	face := text.NewFace(f, 14)
	tb := NewTextBlock(face, "a")
	core.MeasureWidget(tb, render.Size{W: 500, H: 500})
	core.ArrangeWidget(tb, render.Rect{X: 0, Y: 0, W: 200, H: 30})
	if tb.NeedsLayout() {
		t.Fatal("clean after measure+arrange")
	}
	tb.SetText("wider text")
	if !tb.NeedsLayout() {
		t.Fatal("SetText must invalidate measure")
	}
	if tb.Text() != "wider text" {
		t.Fatalf("Text()=%q", tb.Text())
	}
	tb.SetText("wider text") // no-op set must not panic or re-invalidate spuriously
}

func TestTextBlockNilFace(t *testing.T) {
	tb := NewTextBlock(nil, "x")
	core.MeasureWidget(tb, render.Size{W: 100, H: 100})
	if got := tb.DesiredSize(); got != (render.Size{}) {
		t.Fatalf("desired=%v", got)
	}
}
