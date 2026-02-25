package tui

import (
	"sync"

	"github.com/charmbracelet/glamour"
)

var (
	rendererOnce sync.Once
	renderer     *glamour.TermRenderer
	rendererErr  error
)

// RenderMarkdown renders markdown content to terminal-formatted string.
// Falls back to raw content on any error.
func RenderMarkdown(content string, width int) string {
	rendererOnce.Do(func() {
		renderer, rendererErr = glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(width),
		)
	})
	return renderMarkdownWith(renderer, rendererErr, content)
}

// renderMarkdownWith renders content using the provided renderer and error state.
// Returns raw content as fallback when renderer is nil or an error occurred.
// Exported for test access within the package.
func renderMarkdownWith(r *glamour.TermRenderer, rerr error, content string) string {
	if rerr != nil || r == nil {
		return content
	}
	out, err := r.Render(content)
	if err != nil {
		return content
	}
	return out
}
