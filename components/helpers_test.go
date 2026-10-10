package components

import (
	"context"
	"html"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/test/testutil"
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

func tagAttr(tag, name string) (value string, ok bool) {
	rest := strings.TrimPrefix(tag, "<")
	at := strings.IndexAny(rest, " \t\r\n>/")
	if at < 0 {
		return "", false
	}
	rest = rest[at:]
	for {
		rest = strings.TrimLeft(rest, " \t\r\n/")
		if rest == "" || rest[0] == '>' {
			return "", false
		}
		end := strings.IndexAny(rest, " \t\r\n=>/")
		if end < 0 {
			end = len(rest)
		}
		attr, current := rest[:end], ""
		rest = strings.TrimLeft(rest[end:], " \t\r\n")
		if strings.HasPrefix(rest, "=") {
			rest = strings.TrimLeft(rest[1:], " \t\r\n")
			if rest != "" && (rest[0] == '"' || rest[0] == '\'') {
				closing := strings.IndexByte(rest[1:], rest[0])
				if closing < 0 {
					return "", false
				}
				current, rest = rest[1:1+closing], rest[closing+2:]
			} else {
				end := strings.IndexAny(rest, " \t\r\n>")
				if end < 0 {
					end = len(rest)
				}
				current, rest = rest[:end], rest[end:]
			}
		}
		if attr == name {
			return html.UnescapeString(current), true
		}
	}
}

func innerHTML(page, id string) (inner string, ok bool) {
	tag := testutil.TagByID(page, id)
	if tag == "" {
		return "", false
	}
	name := strings.TrimPrefix(tag, "<")
	name = name[:strings.IndexAny(name, " \t\r\n>/")]
	start := strings.Index(page, tag) + len(tag)
	endsName := func(s string) bool { return s != "" && strings.IndexByte("> \t\r\n/", s[0]) >= 0 }
	depth := 1
	for at := start; at < len(page); {
		next := strings.IndexByte(page[at:], '<')
		if next < 0 {
			return "", false
		}
		at += next
		switch rest := page[at:]; {
		case strings.HasPrefix(rest, "</"+name) && endsName(rest[len(name)+2:]):
			if depth--; depth == 0 {
				return page[start:at], true
			}
		case strings.HasPrefix(rest, "<"+name) && endsName(rest[len(name)+1:]):
			depth++
		}
		at++
	}
	return "", false
}

func openTags(page, element string) []string {
	var tags []string
	open := "<" + element
	for at := 0; ; {
		next := strings.Index(page[at:], open)
		if next < 0 {
			return tags
		}
		start := at + next
		after := start + len(open)
		if after >= len(page) || strings.IndexByte(" \t\r\n>/", page[after]) < 0 {
			at = after
			continue
		}
		var quote byte
		end := -1
		for i := after; i < len(page) && end < 0; i++ {
			switch c := page[i]; {
			case quote != 0:
				if c == quote {
					quote = 0
				}
			case c == '"' || c == '\'':
				quote = c
			case c == '>':
				end = i
			}
		}
		if end < 0 {
			return tags
		}
		tags = append(tags, page[start:end+1])
		at = end + 1
	}
}
