package content

import (
	"errors"
	"strings"
	"testing"

	"deckfold/internal/adapter"
)

func file(meta, content string) []byte {
	s := `<!doctype html><html><head><meta charset="utf-8"></head><body><script type="application/json" id="df-meta">` + meta + `</script>`
	if content != "" {
		s += `<script type="application/json" id="df-content">` + content + `</script>`
	}
	return []byte(s + `</body></html>`)
}

func TestRead(t *testing.T) {
	meta := `{"deck":{"format":"pptx","title":"Deck","width":1280,"height":720,"slides":[{"label":"One","notes":"n1","steps":2},{"steps":1}]},"shim":{}}`
	d, err := Read(file(meta, `{"version":1,"slides":[{"title":"One","blocks":[{"role":"title","text":"One","step":0},{"role":"item","text":"a","step":1,"level":1}]},{"blocks":[]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if d.Format != "pptx" || d.Title != "Deck" || d.Width != 1280 || len(d.Slides) != 2 || d.Slides[0].Notes != "n1" || d.Slides[0].Blocks[1].Level != 1 || d.Slides[1].Blocks == nil {
		t.Errorf("read %+v", d)
	}
	for _, c := range []struct {
		name, meta, content string
		want                error
	}{
		{"no model", meta, "", ErrNoModel},
		{"generic", `{"deck":{"format":"generic","slides":[]},"shim":{}}`, "", ErrGeneric},
		{"newer", meta, `{"version":99,"slides":[{"blocks":[]},{"blocks":[]}]}`, ErrNewer},
	} {
		if _, err := Read(file(c.meta, c.content)); !errors.Is(err, c.want) {
			t.Errorf("%s: error %v, want %v", c.name, err, c.want)
		}
	}
	if _, err := Read(file(meta, `{"version":1,"slides":[]}`)); err == nil {
		t.Error("a model with another number of slides than the metadata must be an error")
	}
}

func TestMarkdown(t *testing.T) {
	d := &Deck{Title: "A deck", Slides: []Slide{
		{Title: "First", Notes: "Say hello.\n\n2. not a list", Blocks: []adapter.Block{
			{Role: "title", Text: "First"}, {Role: "subtitle", Text: "Below the title"},
			{Role: "item", Text: "one"}, {Role: "item", Text: "nested", Level: 1}, {Role: "item", Text: "two"},
			{Role: "text", Text: "A paragraph"}, {Role: "alt", Text: "A picture"}}},
		{Blocks: []adapter.Block{{Role: "text", Text: "- not a list"}, {Role: "text", Text: "1. not numbered"},
			{Role: "text", Text: "# no heading [no](link) <b>no html</b> *no* _emphasis_ `no code` &amp; | ~~no~~ ![x](y)"}}},
	}}
	want := `# A deck

## 1. First

Below the title

- one
  - nested
- two

A paragraph

Image: A picture

> Notes
>
> Say hello.
>
> 2\. not a list

## Slide 2

\- not a list

1\. not numbered

\# no heading \[no\](link) \<b\>no html\</b\> \*no\* \_emphasis\_ \` + "`" + `no code\` + "`" + ` \&amp; \| \~\~no\~\~ \!\[x\](y)
`
	if got := string(d.Markdown()); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	// no line of slide text starts a heading, a quote, a list or HTML
	for _, line := range strings.Split(string((&Deck{Slides: []Slide{{Blocks: []adapter.Block{{Role: "text", Text: "> quote"}, {Role: "text", Text: "<script>x</script>"}, {Role: "item", Text: "## h"}}}}}).Markdown()), "\n") {
		if strings.HasPrefix(line, ">") || strings.HasPrefix(line, "<") || strings.HasPrefix(line, "- #") {
			t.Errorf("line %q", line)
		}
	}
}
