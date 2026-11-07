package opc

import (
	"archive/zip"
	"bytes"
	"errors"
	"strings"
	"testing"
)

func zipOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for n, c := range files {
		w, err := z.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(c))
	}
	z.Close()
	return b.Bytes()
}

const types = `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="xml" ContentType="application/xml"/><Override PartName="/ppt/presentation.xml" ContentType="x-main"/></Types>`

func TestRejects(t *testing.T) {
	cfb := append([]byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}, make([]byte, 512)...)
	if _, err := OpenBytes(cfb); !errors.Is(err, ErrCFB) {
		t.Errorf("legacy or encrypted file: %v", err)
	}
	if _, err := OpenBytes([]byte("hello")); !errors.Is(err, ErrNotZip) {
		t.Errorf("no zip: %v", err)
	}
	if _, err := OpenBytes([]byte("PK\x03\x04broken")); err == nil || !strings.Contains(err.Error(), "damaged") {
		t.Errorf("damaged zip: %v", err)
	}
	if _, err := OpenBytes(zipOf(t, map[string]string{"[Content_Types].xml": types, "../evil.xml": "x"})); err == nil {
		t.Error("a part name with .. is accepted")
	}
	if _, err := OpenBytes(zipOf(t, map[string]string{"a.xml": "x"})); err == nil {
		t.Error("a package without content types is accepted")
	}
}

func TestRelsAndTypes(t *testing.T) {
	p, err := OpenBytes(zipOf(t, map[string]string{
		"[Content_Types].xml":             types,
		"ppt/presentation.xml":            "<p/>",
		"ppt/_rels/presentation.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="x/slide" Target="slides/slide1.xml"/><Relationship Id="rId2" Type="x/hyperlink" Target="https://example.com/" TargetMode="External"/><Relationship Id="rId3" Type="x/image" Target="../media/../ppt/media/a.png"/></Relationships>`,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if p.ContentType("ppt/presentation.xml") != "x-main" || p.ContentType("ppt/other.xml") != "application/xml" {
		t.Error("content types")
	}
	rels, err := p.Rels("ppt/presentation.xml")
	if err != nil {
		t.Fatal(err)
	}
	if rels["rId1"].Target != "ppt/slides/slide1.xml" || !rels["rId2"].External || rels["rId3"].Target != "ppt/media/a.png" {
		t.Errorf("rels = %+v", rels)
	}
	if got := ByType(rels, "slide"); len(got) != 1 || got[0].ID != "rId1" {
		t.Errorf("ByType = %+v", got)
	}
}
