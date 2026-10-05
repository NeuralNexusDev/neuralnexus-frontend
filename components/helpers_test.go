package components

import (
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

func renderString(t testing.TB, c templ.Component) string {
	t.Helper()
	var out strings.Builder
	if err := c.Render(context.Background(), &out); err != nil {
		t.Fatalf("render: %v", err)
	}
	return out.String()
}

func renderWithChild(t testing.TB, c templ.Component, child string) string {
	t.Helper()
	var out strings.Builder
	ctx := templ.WithChildren(context.Background(), templ.Raw(child))
	if err := c.Render(ctx, &out); err != nil {
		t.Fatalf("render: %v", err)
	}
	return out.String()
}

func assertInOrder(t testing.TB, html string, parts ...string) {
	t.Helper()
	from := 0
	for _, part := range parts {
		at := strings.Index(html[from:], part)
		if at < 0 {
			t.Errorf("%q is missing, or comes before an earlier part, in:\n%s", part, html)
			return
		}
		from += at + len(part)
	}
}
