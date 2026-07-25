package text

import (
	"testing"

	"golang.org/x/image/font/gofont/goregular"

	"github.com/0xdreadnaught/fluo/render"
)

func TestAtlasPacking(t *testing.T) {
	f, _ := Load(goregular.TTF)
	a := NewAtlas(f)
	seen := map[render.Rect]bool{}
	for _, r := range "AbgQ" {
		gi, _ := f.glyphIndex(r)
		e, err := a.glyph(gi)
		if err != nil {
			t.Fatal(err)
		}
		if e.uv.W <= 0 || e.uv.Right() > 1 || e.uv.Bottom() > 1 {
			t.Errorf("%c: bad uv %v", r, e.uv)
		}
		if seen[e.uv] {
			t.Errorf("%c: uv reused", r)
		}
		seen[e.uv] = true
	}
	gi, _ := f.glyphIndex('A')
	e1, _ := a.glyph(gi)
	e2, _ := a.glyph(gi)
	if e1 != e2 {
		t.Error("glyph not cached")
	}
}
