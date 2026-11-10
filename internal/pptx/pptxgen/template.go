package pptxgen

import "fmt"

// theme returns an Office-like theme with the given fonts.
func theme(head, body string) string {
	return xmlHead + `<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="deckfold test theme"><a:themeElements>` +
		`<a:clrScheme name="deckfold"><a:dk1><a:sysClr val="windowText" lastClr="000000"/></a:dk1><a:lt1><a:sysClr val="window" lastClr="FFFFFF"/></a:lt1>` +
		`<a:dk2><a:srgbClr val="0E2841"/></a:dk2><a:lt2><a:srgbClr val="E8E8E8"/></a:lt2>` +
		`<a:accent1><a:srgbClr val="156082"/></a:accent1><a:accent2><a:srgbClr val="E97132"/></a:accent2><a:accent3><a:srgbClr val="196B24"/></a:accent3>` +
		`<a:accent4><a:srgbClr val="0F9ED5"/></a:accent4><a:accent5><a:srgbClr val="A02B93"/></a:accent5><a:accent6><a:srgbClr val="4EA72E"/></a:accent6>` +
		`<a:hlink><a:srgbClr val="467886"/></a:hlink><a:folHlink><a:srgbClr val="96607D"/></a:folHlink></a:clrScheme>` +
		`<a:fontScheme name="deckfold"><a:majorFont><a:latin typeface="` + Esc(head) + `"/><a:ea typeface=""/><a:cs typeface=""/></a:majorFont>` +
		`<a:minorFont><a:latin typeface="` + Esc(body) + `"/><a:ea typeface=""/><a:cs typeface=""/></a:minorFont></a:fontScheme>` +
		`<a:fmtScheme name="deckfold"><a:fillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill>` +
		`<a:gradFill rotWithShape="1"><a:gsLst><a:gs pos="0"><a:schemeClr val="phClr"><a:lumMod val="110000"/><a:satMod val="105000"/><a:tint val="67000"/></a:schemeClr></a:gs><a:gs pos="100000"><a:schemeClr val="phClr"><a:lumMod val="105000"/><a:satMod val="109000"/><a:tint val="81000"/></a:schemeClr></a:gs></a:gsLst><a:lin ang="5400000" scaled="0"/></a:gradFill>` +
		`<a:gradFill rotWithShape="1"><a:gsLst><a:gs pos="0"><a:schemeClr val="phClr"><a:satMod val="103000"/><a:lumMod val="102000"/><a:tint val="94000"/></a:schemeClr></a:gs><a:gs pos="100000"><a:schemeClr val="phClr"><a:lumMod val="99000"/><a:satMod val="120000"/><a:shade val="78000"/></a:schemeClr></a:gs></a:gsLst><a:lin ang="5400000" scaled="0"/></a:gradFill></a:fillStyleLst>` +
		`<a:lnStyleLst><a:ln w="12700" cap="flat" cmpd="sng" algn="ctr"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:prstDash val="solid"/><a:miter lim="800000"/></a:ln>` +
		`<a:ln w="19050" cap="flat" cmpd="sng" algn="ctr"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:prstDash val="solid"/><a:miter lim="800000"/></a:ln>` +
		`<a:ln w="25400" cap="flat" cmpd="sng" algn="ctr"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:prstDash val="solid"/><a:miter lim="800000"/></a:ln></a:lnStyleLst>` +
		`<a:effectStyleLst><a:effectStyle><a:effectLst/></a:effectStyle><a:effectStyle><a:effectLst/></a:effectStyle><a:effectStyle><a:effectLst><a:outerShdw blurRad="57150" dist="19050" dir="5400000" algn="ctr" rotWithShape="0"><a:srgbClr val="000000"><a:alpha val="63000"/></a:srgbClr></a:outerShdw></a:effectLst></a:effectStyle></a:effectStyleLst>` +
		`<a:bgFillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"><a:tint val="95000"/><a:satMod val="170000"/></a:schemeClr></a:solidFill>` +
		`<a:gradFill rotWithShape="1"><a:gsLst><a:gs pos="0"><a:schemeClr val="phClr"><a:tint val="93000"/></a:schemeClr></a:gs><a:gs pos="100000"><a:schemeClr val="phClr"><a:shade val="98000"/></a:schemeClr></a:gs></a:gsLst><a:lin ang="5400000" scaled="0"/></a:gradFill></a:bgFillStyleLst></a:fmtScheme>` +
		`</a:themeElements><a:objectDefaults/><a:extraClrSchemeLst/></a:theme>`
}

func ph(id int, name, phXML string, x, y, w, h int64, body string) string {
	return fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="%s"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr>%s</p:nvPr></p:nvSpPr><p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr>%s</p:sp>`,
		id, name, phXML, x, y, w, h, body)
}

func emptyTx(bodyPr, lst string) string {
	return `<p:txBody>` + bodyPr + `<a:lstStyle>` + lst + `</a:lstStyle><a:p><a:endParaRPr lang="en-US"/></a:p></p:txBody>`
}

const spTreeHead = `<p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>`

const clrMap = `<p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/>`

// master returns the slide master with title, body, date, footer and slide number placeholders.
func master(d *Deck) string {
	W, H := d.Width, d.Height
	mx := W * 838200 / 12192000
	ftr := `<a:fld id="{B6F15528-21DE-4FAA-801E-634DDDAF4B2B}" type="slidenum"><a:rPr lang="en-US"/><a:t>&lt;#&gt;</a:t></a:fld>`
	tree := spTreeHead +
		ph(2, "Title Placeholder 1", `<p:ph type="title"/>`, mx, H*365125/6858000, W-2*mx, H*1325563/6858000,
			emptyTx(`<a:bodyPr vert="horz" lIns="91440" tIns="45720" rIns="91440" bIns="45720" rtlCol="0" anchor="ctr"><a:normAutofit/></a:bodyPr>`, "")) +
		ph(3, "Text Placeholder 2", `<p:ph type="body" idx="1"/>`, mx, H*1825625/6858000, W-2*mx, H*4351338/6858000,
			emptyTx(`<a:bodyPr vert="horz" lIns="91440" tIns="45720" rIns="91440" bIns="45720" rtlCol="0"><a:normAutofit/></a:bodyPr>`, "")) +
		ph(4, "Date Placeholder 3", `<p:ph type="dt" sz="half" idx="10"/>`, mx, H*6356350/6858000, W*2743200/12192000, H*365125/6858000,
			emptyTx(`<a:bodyPr anchor="ctr"/>`, `<a:lvl1pPr algn="l"><a:defRPr sz="1200"><a:solidFill><a:schemeClr val="tx1"><a:tint val="75000"/></a:schemeClr></a:solidFill></a:defRPr></a:lvl1pPr>`)) +
		ph(5, "Footer Placeholder 4", `<p:ph type="ftr" sz="quarter" idx="11"/>`, W*4038600/12192000, H*6356350/6858000, W*4114800/12192000, H*365125/6858000,
			emptyTx(`<a:bodyPr anchor="ctr"/>`, `<a:lvl1pPr algn="ctr"><a:defRPr sz="1200"><a:solidFill><a:schemeClr val="tx1"><a:tint val="75000"/></a:schemeClr></a:solidFill></a:defRPr></a:lvl1pPr>`)) +
		ph(6, "Slide Number Placeholder 5", `<p:ph type="sldNum" sz="quarter" idx="12"/>`, W-mx-W*2743200/12192000, H*6356350/6858000, W*2743200/12192000, H*365125/6858000,
			`<p:txBody><a:bodyPr anchor="ctr"/><a:lstStyle><a:lvl1pPr algn="r"><a:defRPr sz="1200"><a:solidFill><a:schemeClr val="tx1"><a:tint val="75000"/></a:schemeClr></a:solidFill></a:defRPr></a:lvl1pPr></a:lstStyle><a:p>`+ftr+`<a:endParaRPr lang="en-US"/></a:p></p:txBody>`) +
		`</p:spTree>`
	var layoutIDs string
	for i := range layouts {
		layoutIDs += fmt.Sprintf(`<p:sldLayoutId id="%d" r:id="rId%d"/>`, 2147483649+i, i+1)
	}
	body := `<p:bodyStyle>`
	sizes := []int{2800, 2400, 2000, 1800, 1800, 1800, 1800, 1800, 1800}
	for l := 1; l <= 9; l++ {
		body += fmt.Sprintf(`<a:lvl%dpPr marL="%d" indent="-228600" algn="l" defTabSz="914400" rtl="0" eaLnBrk="1" latinLnBrk="0" hangingPunct="1"><a:lnSpc><a:spcPct val="90000"/></a:lnSpc><a:spcBef><a:spcPts val="%d"/></a:spcBef><a:buFont typeface="Arial" panose="020B0604020202020204" pitchFamily="34" charset="0"/><a:buChar char="&#8226;"/><a:defRPr sz="%d"`+d.styleKern()+`><a:solidFill><a:schemeClr val="tx1"/></a:solidFill><a:latin typeface="+mn-lt"/><a:ea typeface="+mn-ea"/><a:cs typeface="+mn-cs"/></a:defRPr></a:lvl%dpPr>`,
			l, 228600+(l-1)*457200, map[bool]int{true: 1000, false: 500}[l == 1], sizes[l-1], l)
	}
	body += `</p:bodyStyle>`
	return xmlHead + `<p:sldMaster ` + nsDecl + `><p:cSld><p:bg><p:bgRef idx="1001"><a:schemeClr val="bg1"/></p:bgRef></p:bg>` + tree + `</p:cSld>` + clrMap +
		`<p:sldLayoutIdLst>` + layoutIDs + `</p:sldLayoutIdLst>` +
		`<p:txStyles><p:titleStyle><a:lvl1pPr algn="l" defTabSz="914400" rtl="0" eaLnBrk="1" latinLnBrk="0" hangingPunct="1"><a:lnSpc><a:spcPct val="90000"/></a:lnSpc><a:spcBef><a:spcPct val="0"/></a:spcBef><a:buNone/><a:defRPr sz="4400"` + d.styleKern() + `><a:solidFill><a:schemeClr val="tx1"/></a:solidFill><a:latin typeface="+mj-lt"/><a:ea typeface="+mj-ea"/><a:cs typeface="+mj-cs"/></a:defRPr></a:lvl1pPr></p:titleStyle>` +
		body + `<p:otherStyle>` + d.levels("", 1800) + `</p:otherStyle></p:txStyles></p:sldMaster>`
}

// layouts in the order of LayoutTitle, LayoutContent, LayoutBlank.
var layouts = []func(d *Deck) string{titleLayout, contentLayout, blankLayout}

func layoutDoc(name, typ, tree string) string {
	return xmlHead + `<p:sldLayout ` + nsDecl + ` type="` + typ + `" preserve="1"><p:cSld name="` + name + `">` + spTreeHead + tree + `</p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sldLayout>`
}

func footers(d *Deck) string {
	W, H := d.Width, d.Height
	mx := W * 838200 / 12192000
	return ph(4, "Date Placeholder 3", `<p:ph type="dt" sz="half" idx="10"/>`, mx, H*6356350/6858000, W*2743200/12192000, H*365125/6858000, emptyTx(`<a:bodyPr/>`, "")) +
		ph(5, "Footer Placeholder 4", `<p:ph type="ftr" sz="quarter" idx="11"/>`, W*4038600/12192000, H*6356350/6858000, W*4114800/12192000, H*365125/6858000, emptyTx(`<a:bodyPr/>`, "")) +
		ph(6, "Slide Number Placeholder 5", `<p:ph type="sldNum" sz="quarter" idx="12"/>`, W-mx-W*2743200/12192000, H*6356350/6858000, W*2743200/12192000, H*365125/6858000,
			`<p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:fld id="{B6F15528-21DE-4FAA-801E-634DDDAF4B2B}" type="slidenum"><a:rPr lang="en-US"/><a:t>&lt;#&gt;</a:t></a:fld><a:endParaRPr lang="en-US"/></a:p></p:txBody>`)
}

func titleLayout(d *Deck) string {
	W, H := d.Width, d.Height
	x := W * 1524000 / 12192000
	return layoutDoc("Title Slide", "title",
		ph(2, "Title 1", `<p:ph type="ctrTitle"/>`, x, H*1122363/6858000, W-2*x, H*2387600/6858000,
			emptyTx(`<a:bodyPr anchor="b"/>`, `<a:lvl1pPr algn="ctr"><a:defRPr sz="6000"/></a:lvl1pPr>`))+
			ph(3, "Subtitle 2", `<p:ph type="subTitle" idx="1"/>`, x, H*3602038/6858000, W-2*x, H*1655762/6858000,
				emptyTx(`<a:bodyPr/>`, `<a:lvl1pPr marL="0" indent="0" algn="ctr"><a:buNone/><a:defRPr sz="2400"/></a:lvl1pPr>`))+
			footers(d))
}

func contentLayout(d *Deck) string {
	W, H := d.Width, d.Height
	mx := W * 838200 / 12192000
	return layoutDoc("Title and Content", "obj",
		ph(2, "Title 1", `<p:ph type="title"/>`, mx, H*365125/6858000, W-2*mx, H*1325563/6858000, emptyTx(`<a:bodyPr/>`, ""))+
			ph(3, "Content Placeholder 2", `<p:ph idx="1"/>`, mx, H*1825625/6858000, W-2*mx, H*4351338/6858000, emptyTx(`<a:bodyPr/>`, ""))+
			footers(d))
}

func blankLayout(d *Deck) string {
	return layoutDoc("Blank", "blank", footers(d))
}

func notesMaster(d *Deck) string {
	return xmlHead + `<p:notesMaster ` + nsDecl + `><p:cSld><p:bg><p:bgRef idx="1001"><a:schemeClr val="bg1"/></p:bgRef></p:bg>` + spTreeHead +
		ph(2, "Slide Image Placeholder 1", `<p:ph type="sldImg" idx="2"/>`, 685800, 1143000, 5486400, 3086100, "") +
		ph(3, "Notes Placeholder 2", `<p:ph type="body" sz="quarter" idx="3"/>`, 685800, 4400550, 5486400, 3600450, emptyTx(`<a:bodyPr/>`, "")) +
		`</p:spTree></p:cSld>` + clrMap + `<p:notesStyle>` + d.levels("", 1200) + `</p:notesStyle></p:notesMaster>`
}
