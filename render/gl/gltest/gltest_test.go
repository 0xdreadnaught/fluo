package gltest_test

import (
	"testing"

	"github.com/go-gl/gl/v3.3-core/gl"

	"github.com/0xdreadnaught/fluo/render/gl/gltest"
)

func TestClearGolden(t *testing.T) {
	gltest.Run(t, 64, 64, func(fb *gltest.Framebuffer) {
		gl.ClearColor(1, 0.5, 0, 1)
		gl.Clear(gl.COLOR_BUFFER_BIT)
		gltest.CheckGolden(t, "clear", fb.Image())
	})
}
