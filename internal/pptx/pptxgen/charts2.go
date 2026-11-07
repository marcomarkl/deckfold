package pptxgen

import (
	"fmt"
	"strings"
)

// ChartRaw adds a chart frame whose c:chart element has the given content (title, view3D,
// plotArea, legend, in schema order).
func (s *Slide) ChartRaw(d *Deck, name string, b Box, inner string) int {
	id := s.ID()
	n := len(d.extra) + 1
	part := fmt.Sprintf("ppt/charts/chart%d.xml", n)
	rid := s.Rel("http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart", fmt.Sprintf("../charts/chart%d.xml", n), "")
	d.Part(part, "application/vnd.openxmlformats-officedocument.drawingml.chart+xml", xmlHead+
		`<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">`+
		`<c:chart>`+inner+`</c:chart></c:chartSpace>`)
	s.Shapes = append(s.Shapes, fmt.Sprintf(`<p:graphicFrame><p:nvGraphicFramePr><p:cNvPr id="%d" name="%s"/><p:cNvGraphicFramePr/><p:nvPr/></p:nvGraphicFramePr><p:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></p:xfrm>`+
		`<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" r:id="%s"/></a:graphicData></a:graphic></p:graphicFrame>`,
		id, Esc(name), Px(b.X), Px(b.Y), Px(b.W), Px(b.H), rid))
	return id
}

// chart XML pieces

func cTitle(t string) string {
	return `<c:title><c:tx><c:rich><a:bodyPr/><a:lstStyle/><a:p><a:r><a:t>` + Esc(t) + `</a:t></a:r></a:p></c:rich></c:tx><c:overlay val="0"/></c:title><c:autoTitleDeleted val="0"/>`
}

const cLegend = `<c:legend><c:legendPos val="b"/><c:overlay val="0"/></c:legend><c:plotVisOnly val="1"/>`

func cTx(name string, col int) string {
	return fmt.Sprintf(`<c:tx><c:strRef><c:f>Sheet1!$%c$1</c:f><c:strCache><c:ptCount val="1"/><c:pt idx="0"><c:v>%s</c:v></c:pt></c:strCache></c:strRef></c:tx>`, 'B'+col, Esc(name))
}

func cStr(tag string, vals []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<c:%s><c:strRef><c:f>Sheet1!$A$2:$A$%d</c:f><c:strCache><c:ptCount val="%d"/>`, tag, len(vals)+1, len(vals))
	for i, v := range vals {
		fmt.Fprintf(&b, `<c:pt idx="%d"><c:v>%s</c:v></c:pt>`, i, Esc(v))
	}
	fmt.Fprintf(&b, `</c:strCache></c:strRef></c:%s>`, tag)
	return b.String()
}

func cNum(tag string, col int, vals []float64) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<c:%s><c:numRef><c:f>Sheet1!$%c$2:$%c$%d</c:f><c:numCache><c:formatCode>General</c:formatCode><c:ptCount val="%d"/>`, tag, 'A'+col, 'A'+col, len(vals)+1, len(vals))
	for i, v := range vals {
		fmt.Fprintf(&b, `<c:pt idx="%d"><c:v>%g</c:v></c:pt>`, i, v)
	}
	fmt.Fprintf(&b, `</c:numCache></c:numRef></c:%s>`, tag)
	return b.String()
}

// cSer writes a series: inside (between c:tx and the data, in schema order) and data.
func cSer(i int, name, inside, data string) string {
	return fmt.Sprintf(`<c:ser><c:idx val="%d"/><c:order val="%d"/>`, i, i) + cTx(name, i) + inside + data + `</c:ser>`
}

func cAxes(cat bool, ids ...int) string {
	var b strings.Builder
	for _, id := range ids {
		fmt.Fprintf(&b, `<c:axId val="%d"/>`, id)
	}
	return b.String()
}

func catAx(id, cross int) string {
	return fmt.Sprintf(`<c:catAx><c:axId val="%d"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="0"/><c:axPos val="b"/><c:crossAx val="%d"/></c:catAx>`, id, cross)
}

func valAx(id, cross int, pos string, grid bool) string {
	g := ""
	if grid {
		g = `<c:majorGridlines/>`
	}
	return fmt.Sprintf(`<c:valAx><c:axId val="%d"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="0"/><c:axPos val="%s"/>%s<c:crossAx val="%d"/></c:valAx>`, id, pos, g, cross)
}

func serAx(id, cross int) string {
	return fmt.Sprintf(`<c:serAx><c:axId val="%d"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="0"/><c:axPos val="b"/><c:crossAx val="%d"/></c:serAx>`, id, cross)
}

const view3D = `<c:view3D><c:rotX val="15"/><c:rotY val="20"/><c:rAngAx val="0"/><c:perspective val="30"/></c:view3D>`

// Charts2 covers scatter, bubble, radar, stock, pie of pie, bar of pie and surface charts, and
// the 3D charts (column, pie, line, area, surface).
func Charts2() *Deck {
	d := New("Chart feature deck 2")
	box := Box{X: 140, Y: 60, W: 1000, H: 600}
	cats := []string{"North", "East", "South", "West", "Center"}
	a := []float64{4.3, 2.5, 3.5, 4.5, 3.1}
	bb := []float64{2.4, 4.4, 1.8, 2.8, 3.9}
	c := []float64{2, 2, 3, 5, 4}
	add := func(title, inner string) {
		s := d.Add(LayoutBlank)
		s.ChartRaw(d, title, box, inner)
	}
	xs := []float64{1, 2.5, 3, 4.2, 5.5, 7}
	add("Scatter", cTitle("Scatter")+`<c:plotArea><c:layout/><c:scatterChart><c:scatterStyle val="lineMarker"/><c:varyColors val="0"/>`+
		cSer(0, "Measured", `<c:spPr><a:ln w="19050"><a:noFill/></a:ln></c:spPr>`, cNum("xVal", 0, xs)+cNum("yVal", 1, []float64{2.1, 3.9, 3.2, 5.6, 6.1, 8.4}))+
		cSer(1, "Model", "", cNum("xVal", 0, xs)+cNum("yVal", 2, []float64{1.5, 3, 3.5, 4.7, 6, 7.5}))+
		cAxes(false, 2001, 2002)+`</c:scatterChart>`+valAx(2001, 2002, "b", false)+valAx(2002, 2001, "l", true)+`</c:plotArea>`+cLegend)
	add("Bubble", cTitle("Bubble")+`<c:plotArea><c:layout/><c:bubbleChart><c:varyColors val="0"/>`+
		cSer(0, "Markets", `<c:invertIfNegative val="0"/>`, cNum("xVal", 0, []float64{1, 2, 3, 4})+cNum("yVal", 1, []float64{3, 5, 2, 6})+cNum("bubbleSize", 2, []float64{4, 9, 16, 6})+`<c:bubble3D val="0"/>`)+
		`<c:bubbleScale val="100"/><c:showNegBubbles val="0"/>`+cAxes(false, 2101, 2102)+`</c:bubbleChart>`+valAx(2101, 2102, "b", false)+valAx(2102, 2101, "l", true)+`</c:plotArea>`+cLegend)
	add("Radar", cTitle("Radar")+`<c:plotArea><c:layout/><c:radarChart><c:radarStyle val="marker"/><c:varyColors val="0"/>`+
		cSer(0, "This year", "", cStr("cat", cats)+cNum("val", 1, a))+cSer(1, "Last year", "", cStr("cat", cats)+cNum("val", 2, bb))+
		cAxes(true, 2201, 2202)+`</c:radarChart>`+catAx(2201, 2202)+valAx(2202, 2201, "l", true)+`</c:plotArea>`+cLegend)
	days := []string{"Mon", "Tue", "Wed", "Thu", "Fri"}
	noLine := `<c:spPr><a:ln w="19050"><a:noFill/></a:ln></c:spPr><c:marker><c:symbol val="none"/></c:marker>`
	add("Stock", cTitle("Stock")+`<c:plotArea><c:layout/><c:stockChart>`+
		cSer(0, "High", noLine, cStr("cat", days)+cNum("val", 1, []float64{55, 57, 57, 58, 58}))+
		cSer(1, "Low", noLine, cStr("cat", days)+cNum("val", 2, []float64{11, 12, 13, 11, 25}))+
		cSer(2, "Close", noLine, cStr("cat", days)+cNum("val", 3, []float64{25, 38, 50, 35, 43}))+
		`<c:hiLowLines/>`+cAxes(true, 2301, 2302)+`</c:stockChart>`+catAx(2301, 2302)+valAx(2302, 2301, "l", true)+`</c:plotArea>`+cLegend)
	sales := []float64{40, 25, 15, 8, 7, 5}
	items := []string{"Alpha", "Beta", "Gamma", "Delta", "Epsilon", "Zeta"}
	for _, t := range []string{"pie", "bar"} {
		add(strings.ToUpper(t[:1])+t[1:]+" of pie", cTitle(strings.ToUpper(t[:1])+t[1:]+" of pie")+`<c:plotArea><c:layout/><c:ofPieChart><c:ofPieType val="`+t+`"/><c:varyColors val="1"/>`+
			cSer(0, "Share", "", cStr("cat", items)+cNum("val", 1, sales))+
			`<c:gapWidth val="100"/><c:splitType val="pos"/><c:splitPos val="3"/><c:secondPieSize val="75"/><c:serLines/></c:ofPieChart></c:plotArea>`+cLegend)
	}
	grid := [][]float64{{1, 2, 3, 2, 1}, {2, 4, 5, 4, 2}, {3, 5, 7, 5, 3}, {2, 4, 5, 4, 2}}
	surfaceSeries := func() string {
		var b strings.Builder
		for i, row := range grid {
			b.WriteString(cSer(i, fmt.Sprintf("Row %d", i+1), "", cStr("cat", cats)+cNum("val", i+1, row)))
		}
		return b.String()
	}
	add("Surface", cTitle("Surface")+view3D+`<c:plotArea><c:layout/><c:surfaceChart><c:wireframe val="0"/>`+surfaceSeries()+
		cAxes(true, 2401, 2402, 2403)+`</c:surfaceChart>`+catAx(2401, 2402)+valAx(2402, 2401, "l", true)+serAx(2403, 2402)+`</c:plotArea>`+cLegend)
	add("3D column", cTitle("3D column")+view3D+`<c:plotArea><c:layout/><c:bar3DChart><c:barDir val="col"/><c:grouping val="clustered"/><c:varyColors val="0"/>`+
		cSer(0, "This year", `<c:invertIfNegative val="0"/>`, cStr("cat", cats)+cNum("val", 1, a))+cSer(1, "Last year", `<c:invertIfNegative val="0"/>`, cStr("cat", cats)+cNum("val", 2, bb))+
		`<c:gapWidth val="150"/><c:shape val="box"/>`+cAxes(true, 2501, 2502)+`</c:bar3DChart>`+catAx(2501, 2502)+valAx(2502, 2501, "l", true)+`</c:plotArea>`+cLegend)
	add("3D pie", cTitle("3D pie")+`<c:view3D><c:rotX val="30"/><c:rotY val="0"/><c:rAngAx val="0"/></c:view3D><c:plotArea><c:layout/><c:pie3DChart><c:varyColors val="1"/>`+
		cSer(0, "Share", "", cStr("cat", items[:4])+cNum("val", 1, []float64{40, 30, 20, 10}))+`</c:pie3DChart></c:plotArea>`+cLegend)
	add("3D line", cTitle("3D line")+view3D+`<c:plotArea><c:layout/><c:line3DChart><c:grouping val="standard"/><c:varyColors val="0"/>`+
		cSer(0, "This year", "", cStr("cat", cats)+cNum("val", 1, a))+cSer(1, "Last year", "", cStr("cat", cats)+cNum("val", 2, bb))+
		cAxes(true, 2601, 2602, 2603)+`</c:line3DChart>`+catAx(2601, 2602)+valAx(2602, 2601, "l", true)+serAx(2603, 2602)+`</c:plotArea>`+cLegend)
	add("3D area", cTitle("3D area")+view3D+`<c:plotArea><c:layout/><c:area3DChart><c:grouping val="standard"/><c:varyColors val="0"/>`+
		cSer(0, "This year", "", cStr("cat", cats)+cNum("val", 1, a))+cSer(1, "Planned", "", cStr("cat", cats)+cNum("val", 2, c))+
		cAxes(true, 2701, 2702, 2703)+`</c:area3DChart>`+catAx(2701, 2702)+valAx(2702, 2701, "l", true)+serAx(2703, 2702)+`</c:plotArea>`+cLegend)
	add("3D surface", cTitle("3D surface")+view3D+`<c:plotArea><c:layout/><c:surface3DChart><c:wireframe val="0"/>`+surfaceSeries()+
		cAxes(true, 2801, 2802, 2803)+`</c:surface3DChart>`+catAx(2801, 2802)+valAx(2802, 2801, "l", true)+serAx(2803, 2802)+`</c:plotArea>`+cLegend)
	return d
}
