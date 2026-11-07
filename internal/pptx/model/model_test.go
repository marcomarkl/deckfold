package model

import (
	"testing"

	"deckfold/internal/pptx/opc"
	"deckfold/internal/pptx/pptxgen"
)

func read(t *testing.T, d *pptxgen.Deck) *Presentation {
	t.Helper()
	b, err := d.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := opc.OpenBytes(b)
	if err != nil {
		t.Fatal(err)
	}
	p, err := Read(pkg)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestInheritance(t *testing.T) {
	p := read(t, pptxgen.Basic())
	if len(p.Slides) != 4 || len(p.Visible()) != 3 || !p.Slides[2].Hidden {
		t.Fatalf("slides %d, visible %d", len(p.Slides), len(p.Visible()))
	}
	s := p.Slides[0]
	// the centered title of the title slide inherits from the layout (ctrTitle) and the master (title)
	var title, sub = s.Root.Path("cSld/spTree").Children("sp")[0], s.Root.Path("cSld/spTree").Children("sp")[1]
	l, m := s.Inherited(Ph(title))
	if PhType(Ph(l)) != "ctrTitle" || PhType(Ph(m)) != "title" {
		t.Errorf("title inherits from %s / %s", PhType(Ph(l)), PhType(Ph(m)))
	}
	// the subtitle (idx 1) inherits from the layout subtitle and the master body
	l, m = s.Inherited(Ph(sub))
	if PhType(Ph(l)) != "subTitle" || PhType(Ph(m)) != "body" {
		t.Errorf("subtitle inherits from %s / %s", PhType(Ph(l)), PhType(Ph(m)))
	}
	if s.Layout.Master.TextStyle("ctrTitle") != s.Layout.Master.Title || s.Layout.Master.TextStyle("subTitle") != s.Layout.Master.Body {
		t.Error("master text styles")
	}
	if s.Layout.Master.Theme.MinorLatin != "Carlito" || s.Layout.Master.Theme.Colors["accent1"] != "156082" {
		t.Errorf("theme: %q %v", s.Layout.Master.Theme.MinorLatin, s.Layout.Master.Theme.Colors)
	}
	if s.Notes != "Notes of the first slide.\nA second paragraph of notes." {
		t.Errorf("notes = %q", s.Notes)
	}
}
