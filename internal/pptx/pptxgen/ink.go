package pptxgen

import "fmt"

// Ink adds ink strokes (an InkML part in a p14:contentPart, with a picture as the fallback for
// readers without ink). traces are InkML trace texts, color the brush color, width in cm.
func (s *Slide) Ink(d *Deck, name string, b Box, color string, width float64, traces ...string) int {
	id := s.ID()
	n := 1
	for _, e := range d.extra {
		if len(e.name) > 8 && e.name[:8] == "ppt/ink/" {
			n++
		}
	}
	part := fmt.Sprintf("ppt/ink/ink%d.xml", n)
	rid := s.Rel("http://schemas.openxmlformats.org/officeDocument/2006/relationships/customXml", fmt.Sprintf("../ink/ink%d.xml", n), "")
	img := s.Rel(relImage, "../media/"+d.Media("png", "image/png", TestPNG(40, 40)), "")
	ink := `<inkml:ink xmlns:inkml="http://www.w3.org/2003/InkML"><inkml:definitions><inkml:context xml:id="ctx0"><inkml:inkSource xml:id="inkSrc0"><inkml:traceFormat>` +
		`<inkml:channel name="X" type="integer" max="32767" units="cm"/><inkml:channel name="Y" type="integer" max="32767" units="cm"/></inkml:traceFormat>` +
		`<inkml:channelProperties><inkml:channelProperty channel="X" name="resolution" value="1000" units="1/cm"/><inkml:channelProperty channel="Y" name="resolution" value="1000" units="1/cm"/></inkml:channelProperties>` +
		`</inkml:inkSource><inkml:timestamp xml:id="ts0" timeString="2026-01-01T00:00:00.000"/></inkml:context>` +
		fmt.Sprintf(`<inkml:brush xml:id="br0"><inkml:brushProperty name="width" value="%g" units="cm"/><inkml:brushProperty name="height" value="%g" units="cm"/><inkml:brushProperty name="color" value="%s"/></inkml:brush>`, width, width, color) +
		`</inkml:definitions>`
	for _, t := range traces {
		ink += `<inkml:trace contextRef="#ctx0" brushRef="#br0">` + t + `</inkml:trace>`
	}
	ink += `</inkml:ink>`
	d.Part(part, "application/inkml+xml", xmlHead+ink)
	s.Shapes = append(s.Shapes, `<mc:AlternateContent xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006"><mc:Choice xmlns:p14="http://schemas.microsoft.com/office/powerpoint/2010/main" Requires="p14">`+
		fmt.Sprintf(`<p:contentPart p14:bwMode="auto" r:id="%s"><p14:nvContentPartPr><p14:cNvPr id="%d" name="%s"/><p14:cNvContentPartPr/><p14:nvPr/></p14:nvContentPartPr><p14:xfrm>%s</p14:xfrm></p:contentPart>`, rid, id, Esc(name), b.xfrmInner())+
		`</mc:Choice><mc:Fallback>`+
		fmt.Sprintf(`<p:pic><p:nvPicPr><p:cNvPr id="%d" name="%s"/><p:cNvPicPr/><p:nvPr/></p:nvPicPr><p:blipFill><a:blip r:embed="%s"/><a:stretch><a:fillRect/></a:stretch></p:blipFill><p:spPr>%s<a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr></p:pic>`, id, Esc(name), img, b.xfrm())+
		`</mc:Fallback></mc:AlternateContent>`)
	return id
}

// InkDeck has ink strokes: a wave written with first differences, a loop with second
// differences, and a straight stroke with explicit values.
func InkDeck() *Deck {
	d := New("Ink feature deck")
	s := d.Add(LayoutBlank)
	s.TextBox("Title", Box{X: 60, Y: 30, W: 1160, H: 60}, "", Para{Text: "Ink", RPr: `sz="3200"`})
	wave := "1000 3000"
	for i := 0; i < 20; i++ {
		dy := 400
		if i%4 >= 2 {
			dy = -400
		}
		wave += fmt.Sprintf(",'400'%d", dy)
	}
	loop := `2000 6000,'300'0,"0'-100,"-20"-40,"-20"-20,"-20"0,"-20"20,"-20"40`
	line := "1000 9000,9000 9000"
	s.Ink(d, "Ink 1", Box{X: 140, Y: 140, W: 1000, H: 520}, "#E71224", 0.1, wave, loop, line)
	return d
}
