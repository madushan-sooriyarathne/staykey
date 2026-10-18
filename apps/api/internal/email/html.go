package email

import (
	"html"
	"regexp"
	"strings"
)

var linkPattern = regexp.MustCompile(`https?://[^\s<>"]+[^\s<>".,;:!?)]`)

// Body returns the text and HTML parts of an email: text followed by a footer line. The HTML is
// the same words escaped, with paragraphs, line breaks and links, and no images or tracking.
func Body(text, footer string) (plain, htmlPart string) {
	plain = text
	if footer != "" {
		plain += "\n\n--\n" + footer
	}
	var b strings.Builder
	b.WriteString(`<!doctype html><html><body style="margin:0;padding:24px;background:#ffffff;">`)
	b.WriteString(`<div style="max-width:560px;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Helvetica,Arial,sans-serif;font-size:15px;line-height:1.55;color:#09090b;">`)
	for _, para := range strings.Split(text, "\n\n") {
		if strings.TrimSpace(para) == "" {
			continue
		}
		b.WriteString(`<p style="margin:0 0 16px;">`)
		b.WriteString(htmlLines(para))
		b.WriteString(`</p>`)
	}
	if footer != "" {
		b.WriteString(`<p style="margin:32px 0 0;font-size:13px;color:#71717a;">`)
		b.WriteString(htmlLines(footer))
		b.WriteString(`</p>`)
	}
	b.WriteString(`</div></body></html>`)
	return plain, b.String()
}

func htmlLines(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = linkify(l)
	}
	return strings.Join(lines, "<br>")
}

// linkify escapes a line and turns its URLs into links.
func linkify(line string) string {
	var b strings.Builder
	last := 0
	for _, m := range linkPattern.FindAllStringIndex(line, -1) {
		b.WriteString(html.EscapeString(line[last:m[0]]))
		u := html.EscapeString(line[m[0]:m[1]])
		b.WriteString(`<a href="` + u + `" style="color:#09090b;">` + u + `</a>`)
		last = m[1]
	}
	b.WriteString(html.EscapeString(line[last:]))
	return b.String()
}
