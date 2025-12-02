package markdown

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

type Markup struct {
	Tag         string
	Prefix      string
	Suffix      string
	Replacement string
}

type ExtendedMarkup struct {
	Tag        string
	Attributes map[string]string
}

var (
	headerPattern *regexp.Regexp = regexp.MustCompile(`(?m)^#(\s+)`)
	markupTags                   = map[string]Markup{
		"h1":     {Tag: "h1", Prefix: "\n# ", Suffix: "\n\n"},
		"h2":     {Tag: "h2", Prefix: "\n## ", Suffix: "\n\n"},
		"h3":     {Tag: "h3", Prefix: "\n### ", Suffix: "\n\n"},
		"h4":     {Tag: "h4", Prefix: "\n#### ", Suffix: "\n\n"},
		"h5":     {Tag: "h5", Prefix: "\n##### ", Suffix: "\n\n"},
		"h6":     {Tag: "h6", Prefix: "\n###### ", Suffix: "\n\n"},
		"p":      {Tag: "p", Prefix: "", Suffix: "\n\n"},
		"code":   {Tag: "code", Prefix: "`", Suffix: "`"},
		"strong": {Tag: "strong", Prefix: "**", Suffix: "**"},
		"em":     {Tag: "em", Prefix: "_", Suffix: "_"},
		"i":      {Tag: "i", Prefix: "_", Suffix: "_"},
		"del":    {Tag: "del", Prefix: "~~", Suffix: "~~"},
		"strike": {Tag: "strike", Prefix: "~~", Suffix: "~~"},
		"s":      {Tag: "s", Prefix: "~~", Suffix: "~~"},
		"br":     {Tag: "br", Replacement: "  \n"},
		"hr":     {Tag: "hr", Replacement: "\n---\n\n"},
	}
	extensionTags = map[string]ExtendedMarkup{
		"abbr":  {Tag: "abbr", Attributes: map[string]string{"kind": "abbr"}},
		"kbd":   {Tag: "kbd", Attributes: map[string]string{"kind": "kbd"}},
		"mark":  {Tag: "mark", Attributes: map[string]string{"kind": "mark"}},
		"q":     {Tag: "q", Attributes: map[string]string{"kind": "q"}},
		"small": {Tag: "small", Attributes: map[string]string{"kind": "small"}},
		"sub":   {Tag: "sub", Attributes: map[string]string{"kind": "sub"}},
		"sup":   {Tag: "sup", Attributes: map[string]string{"kind": "sup"}},
	}
	preservedTags = []string{
		"address",
		"cite",
		"dfn",
		"ins",
		"samp",
	}
	wrapperTags = []string{
		"html", "head", "body", "header", "nav", "main", "section", "article", "footer",
		"div", "span",
		"figure",
	}
	skippedTags = []string{
		"style", "script",
	}
)

func Convert(htmlInput string) (out string, err error) {
	var node *html.Node
	var buf bytes.Buffer
	if node, err = html.Parse(strings.NewReader(htmlInput)); err != nil {
		return "", err
	}
	walk(&buf, node, 0, true)
	return buf.String(), nil
}

func walk(buf *bytes.Buffer, n *html.Node, indent int, extended bool) {
	switch n.Type {

	case html.TextNode:
		var prefix, suffix string

		// Normalize CR/LF/Tabs
		text := n.Data
		text = strings.ReplaceAll(text, "\r", "")
		text = strings.ReplaceAll(text, "\t", " ")
		text = strings.ReplaceAll(text, "\n", " ")
		if strings.HasPrefix(text, " ") {
			prefix = " "
		}
		if strings.HasSuffix(text, " ") {
			suffix = " "
		}
		text = fmt.Sprintf("%s%s%s", prefix, strings.Join(strings.Fields(text), " "), suffix)
		if strings.TrimSpace(text) == "" {
			return
		}
		buf.WriteString(escape(text))
		return
	case html.ElementNode:
		// skipped tags
		if slices.Contains(skippedTags, n.Data) {
			return
		}

		// markup tags (<strong>, <em>, <p>, <code>, etc.)
		if markup, ok := markupTags[n.Data]; ok {
			if markup.Replacement != "" {
				buf.WriteString(markup.Replacement)
				return
			}
			if markup.Prefix != "" {
				buf.WriteString(markup.Prefix)
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(buf, c, indent, extended)
			}
			if markup.Suffix != "" {
				buf.WriteString(markup.Suffix)
			}
			return
		}

		// extension tags (<small>, etc.)
		if extension, ok := extensionTags[n.Data]; ok {
			var attributes []string
			if extended {
				buf.WriteString("^[")
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(buf, c, indent, false) // NOTE: nested extensions are not supported
			}
			if extended {
				buf.WriteString("](")
				for attr, val := range extension.Attributes {
					attributes = append(attributes, fmt.Sprintf("%s: \"%s\"", attr, val))
				}
				buf.WriteString(fmt.Sprintf("%s)", strings.Join(attributes, ", ")))
			}
			return
		}

		// wrapper tags (div, section, etc.)
		if slices.Contains(wrapperTags, n.Data) {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(buf, c, indent, extended)
			}
			return
		}

		// special cases
		switch n.Data {
		case "blockquote":
			blockquote(buf, n, extended)
			return
		case "ul":
			ul(buf, n, indent, extended)
			return
		case "ol":
			ol(buf, n, indent, extended)
			return
		case "pre":
			pre(buf, n)
			return
		case "a":
			a(buf, n, extended)
			return
		case "img":
			img(buf, n, extended)
			return
		}

		// preserved literal HTML
		if slices.Contains(preservedTags, n.Data) {
			buf.WriteString("<" + n.Data + ">")
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(buf, c, indent, extended)
			}
			buf.WriteString("</" + n.Data + ">")
			return
		}

		// unknown tags
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			// fmt.Printf("tag %q unknown\n", n.Data)
			walk(buf, c, indent, extended)
		}
		return

	case html.CommentNode:
		buf.WriteString("<!-- " + n.Data + " -->")
		return
	}

	// fallback
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(buf, c, indent, extended)
	}
}

func blockquote(buf *bytes.Buffer, n *html.Node, extended bool) {
	var quote bytes.Buffer
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(&quote, c, 0, extended)
	}
	for _, line := range strings.Split(strings.TrimSpace(quote.String()), "\n") {
		buf.WriteString("> " + strings.TrimSpace(line) + "\n")
	}
	buf.WriteString("\n")
}

func ul(buf *bytes.Buffer, n *html.Node, depth int, extended bool) {
	indent := strings.Repeat("  ", depth)
	bullet := "* "

	for li := n.FirstChild; li != nil; li = li.NextSibling {
		if li.Type != html.ElementNode || li.Data != "li" {
			continue
		}

		var item bytes.Buffer
		item.WriteString(indent + bullet)

		// write only inline content first
		for c := li.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && (c.Data == "ul" || c.Data == "ol") {
				continue
			}
			walk(&item, c, depth, extended)
		}

		// collapse internal whitespace
		line := strings.TrimSpace(strings.Join(strings.Fields(item.String()), " "))
		buf.WriteString(line + "\n")

		// now render nested lists
		for c := li.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && c.Data == "ul" {
				ul(buf, c, depth+1, extended)
			} else if c.Data == "ol" {
				ol(buf, c, depth+1, extended)
			}
		}
	}

	buf.WriteString("\n")
}

func ol(buf *bytes.Buffer, n *html.Node, depth int, extended bool) {
	indent := strings.Repeat("  ", depth)
	i := 1

	for li := n.FirstChild; li != nil; li = li.NextSibling {
		if li.Type != html.ElementNode || li.Data != "li" {
			continue
		}

		var item bytes.Buffer
		item.WriteString(indent + strconv.Itoa(i) + ". ")

		for c := li.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && (c.Data == "ul" || c.Data == "ol") {
				continue
			}
			walk(&item, c, depth, extended)
		}

		line := strings.TrimSpace(strings.Join(strings.Fields(item.String()), " "))
		buf.WriteString(line + "\n")

		for c := li.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && c.Data == "ul" {
				ul(buf, c, depth+1, extended)
			} else if c.Type == html.ElementNode && c.Data == "ol" {
				ol(buf, c, depth+1, extended)
			}
		}

		i++
	}

	buf.WriteString("\n")
}

func pre(buf *bytes.Buffer, n *html.Node) {
	if !newline(buf) {
		buf.WriteString("\n")
	}
	buf.WriteString("^[\n")
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		preformatted(buf, c) // ignore extensions
	}
	buf.WriteString("\n](kind: \"pre\")\n\n")
}

func a(buf *bytes.Buffer, n *html.Node, extended bool) {
	var href string = ""
	var inner bytes.Buffer
	for _, a := range n.Attr {
		if a.Key == "href" {
			href = a.Val
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(&inner, c, 0, extended)
	}
	text := strings.TrimSpace(inner.String())
	if text == "" && href != "" {
		// if no link text, use the domain as text? – Codex
		href = strings.TrimPrefix(href, "https://")
		href = strings.TrimPrefix(href, "http://")
		parts := strings.Split(href, "/")
		text = parts[0]
	}
	if !whitespaced(buf) {
		buf.WriteString(" ")
	}
	if href == "" {
		buf.WriteString(text)
		return
	}
	buf.WriteString("[" + text + "](" + href + ")")
}

func img(buf *bytes.Buffer, n *html.Node, extended bool) {
	var alt string
	var src string
	var srcset string
	var attributes []string = []string{"kind: \"img\""}
	for _, a := range n.Attr {
		if a.Key == "alt" {
			alt = a.Val
		}
		if a.Key == "src" {
			src = a.Val
			attributes = append(attributes, fmt.Sprintf("%s: \"%s\"", "src", a.Val))
		}
		if a.Key == "srcset" {
			srcset = a.Val
			attributes = append(attributes, fmt.Sprintf("%s: \"%s\"", "srcset", a.Val))
			if src == "" {
				for _, s := range strings.Split(srcset, ",") {
					s = strings.Join(strings.Fields(strings.TrimSpace(s)), " ") // normalize spaces
					parts := strings.SplitN(s, " ", 2)
					partSrc := parts[0]
					partWidth, _ := strconv.Atoi(strings.TrimSuffix(strings.Join(parts[1:], ""), "w"))
					if len(parts) > 0 && strings.HasSuffix(strings.Join(parts[1:], ""), "w") && partWidth > 1 {
						src = partSrc
						break
					}
				}
			}
		}
	}
	if !newline(buf) {
		buf.WriteString("\n")
	}
	if extended {
		buf.WriteString(fmt.Sprintf("\n^[%s](%s)\n\n", alt, strings.Join(attributes, ", ")))
	} else {
		buf.WriteString(fmt.Sprintf("\n![%s](%s)\n\n", alt, src))
	}
}

func escape(s string) (r string) {
	r = strings.ReplaceAll(s, "`", "\\`")
	r = strings.ReplaceAll(r, "*", "\\*")
	r = strings.ReplaceAll(r, "_", "\\_")
	r = strings.ReplaceAll(r, "[", "\\[")
	r = strings.ReplaceAll(r, "]", "\\]")
	r = headerPattern.ReplaceAllString(r, `\\#$1`)
	r = strings.ReplaceAll(r, ">", "\\>")
	return r
}

// newline checks if the last character in the buffer is a newline
func newline(buf *bytes.Buffer) bool {
	bs := buf.Bytes()
	if len(bs) == 0 {
		return true
	}
	return bs[len(bs)-1] == '\n'
}

// whitespace checks if the last character in the buffer is whitespace
func whitespaced(buf *bytes.Buffer) bool {
	bs := buf.Bytes()
	if len(bs) == 0 {
		return true
	}
	return bs[len(bs)-1] == ' ' || bs[len(bs)-1] == '\n' || bs[len(bs)-1] == '\t'
}


func preformatted(buf *bytes.Buffer, n *html.Node) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		switch {
		case c.Type == html.TextNode:
			buf.WriteString(c.Data)
		case c.Type == html.ElementNode && c.Data == "br":
			buf.WriteString("\n")
		case c.Type == html.ElementNode:
			preformatted(buf, c)
		default:
			continue
		}
	}
}