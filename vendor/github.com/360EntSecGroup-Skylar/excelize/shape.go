package excelize

import (
	"encoding/json"
	"encoding/xml"
	"strconv"
	"strings"
)

// parseFormatShapeSet provides function to parse the format settings of the
// shape with default value.
func parseFormatShapeSet(formatSet string) (*formatShape, error) {
	format := formatShape{
		Width:  160,
		Height: 160,
		Format: formatPicture{
			FPrintsWithSheet: true,
			FLocksWithSheet:  false,
			NoChangeAspect:   false,
			OffsetX:          0,
			OffsetY:          0,
			XScale:           1.0,
			YScale:           1.0,
		},
	}
	err := json.Unmarshal([]byte(formatSet), &format)
	return &format, err
}

// AddShape provides the method to add shape in a sheet by given worksheet
// index, shape format set (such as offset, scale, aspect ratio setting and
// print settings) and properties set. For example, add text box (rect shape) in
// Sheet1:
//
//    xlsx.AddShape("Sheet1", "G6", `{"type":"rect","color":{"line":"#4286F4","fill":"#8eb9ff"},"paragraph":[{"text":"Rectangle Shape","font":{"bold":true,"italic":true,"family":"Berlin Sans FB Demi","size":36,"color":"#777777","underline":"sng"}}],"width":180,"height": 90}`)
//
// The following shows the type of shape supported by excelize:
//
//    accentBorderCallout1 (Callout 1 with Border and Accent Shape)
//    accentBorderCallout2 (Callout 2 with Border and Accent Shape)
//    accentBorderCallout3 (Callout 3 with Border and Accent Shape)
//    accentCallout1 (Callout 1 Shape)
//    accentCallout2 (Callout 2 Shape)
//    accentCallout3 (Callout 3 Shape)
//    actionButtonBackPrevious (Back or Previous Button Shape)
//    actionButtonBeginning (Beginning Button Shape)
//    actionButtonBlank (Blank Button Shape)
//    actionButtonDocument (Document Button Shape)
//    actionButtonEnd (End Button Shape)
//    actionButtonForwardNext (Forward or Next Button Shape)
//    actionButtonHelp (Help Button Shape)
//    actionButtonHome (Home Button Shape)
//    actionButtonInformation (Information Button Shape)
//    actionButtonMovie (Movie Button Shape)
//    actionButtonReturn (Return Button Shape)
//    actionButtonSound (Sound Button Shape)
//    arc (Curved Arc Shape)
//    bentArrow (Bent Arrow Shape)
//    bentConnector2 (Bent Connector 2 Shape)
//    bentConnector3 (Bent Connector 3 Shape)
//    bentConnector4 (Bent Connector 4 Shape)
//    bentConnector5 (Bent Connector 5 Shape)
//    bentUpArrow (Bent Up Arrow Shape)
//    bevel (Bevel Shape)
//    blockArc (Block Arc Shape)
//    borderCallout1 (Callout 1 with Border Shape)
//    borderCallout2 (Callout 2 with Border Shape)
//    borderCallout3 (Callout 3 with Border Shape)
//    bracePair (Brace Pair Shape)
//    bracketPair (Bracket Pair Shape)
//    callout1 (Callout 1 Shape)
//    callout2 (Callout 2 Shape)
//    callout3 (Callout 3 Shape)
//    can (Can Shape)
//    chartPlus (Chart Plus Shape)
//    chartStar (Chart Star Shape)
//    chartX (Chart X Shape)
//    chevron (Chevron Shape)
//    chord (Chord Shape)
//    circularArrow (Circular Arrow Shape)
//    cloud (Cloud Shape)
//    cloudCallout (Callout Cloud Shape)
//    corner (Corner Shape)
//    cornerTabs (Corner Tabs Shape)
//    cube (Cube Shape)
//    curvedConnector2 (Curved Connector 2 Shape)
//    curvedConnector3 (Curved Connector 3 Shape)
//    curvedConnector4 (Curved Connector 4 Shape)
//    curvedConnector5 (Curved Connector 5 Shape)
//    curvedDownArrow (Curved Down Arrow Shape)
//    curvedLeftArrow (Curved Left Arrow Shape)
//    curvedRightArrow (Curved Right Arrow Shape)
//    curvedUpArrow (Curved Up Arrow Shape)
//    decagon (Decagon Shape)
//    diagStripe (Diagonal Stripe Shape)
//    diamond (Diamond Shape)
//    dodecagon (Dodecagon Shape)
//    donut (Donut Shape)
//    doubleWave (Double Wave Shape)
//    downArrow (Down Arrow Shape)
//    downArrowCallout (Callout Down Arrow Shape)
//    ellipse (Ellipse Shape)
//    ellipseRibbon (Ellipse Ribbon Shape)
//    ellipseRibbon2 (Ellipse Ribbon 2 Shape)
//    flowChartAlternateProcess (Alternate Process Flow Shape)
//    flowChartCollate (Collate Flow Shape)
//    flowChartConnector (Connector Flow Shape)
//    flowChartDecision (Decision Flow Shape)
//    flowChartDelay (Delay Flow Shape)
//    flowChartDisplay (Display Flow Shape)
//    flowChartDocument (Document Flow Shape)
//    flowChartExtract (Extract Flow Shape)
//    flowChartInputOutput (Input Output Flow Shape)
//    flowChartInternalStorage (Internal Storage Flow Shape)
//    flowChartMagneticDisk (Magnetic Disk Flow Shape)
//    flowChartMagneticDrum (Magnetic Drum Flow Shape)
//    flowChartMagneticTape (Magnetic Tape Flow Shape)
//    flowChartManualInput (Manual Input Flow Shape)
//    flowChartManualOperation (Manual Operation Flow Shape)
//    flowChartMerge (Merge Flow Shape)
//    flowChartMultidocument (Multi-Document Flow Shape)
//    flowChartOfflineStorage (Offline Storage Flow Shape)
//    flowChartOffpageConnector (Off-Page Connector Flow Shape)
//    flowChartOnlineStorage (Online Storage Flow Shape)
//    flowChartOr (Or Flow Shape)
//    flowChartPredefinedProcess (Predefined Process Flow Shape)
//    flowChartPreparation (Preparation Flow Shape)
//    flowChartProcess (Process Flow Shape)
//    flowChartPunchedCard (Punched Card Flow Shape)
//    flowChartPunchedTape (Punched Tape Flow Shape)
//    flowChartSort (Sort Flow Shape)
//    flowChartSummingJunction (Summing Junction Flow Shape)
//    flowChartTerminator (Terminator Flow Shape)
//    foldedCorner (Folded Corner Shape)
//    frame (Frame Shape)
//    funnel (Funnel Shape)
//    gear6 (Gear 6 Shape)
//    gear9 (Gear 9 Shape)
//    halfFrame (Half Frame Shape)
//    heart (Heart Shape)
//    heptagon (Heptagon Shape)
//    hexagon (Hexagon Shape)
//    homePlate (Home Plate Shape)
//    horizontalScroll (Horizontal Scroll Shape)
//    irregularSeal1 (Irregular Seal 1 Shape)
//    irregularSeal2 (Irregular Seal 2 Shape)
//    leftArrow (Left Arrow Shape)
//    leftArrowCallout (Callout Left Arrow Shape)
//    leftBrace (Left Brace Shape)
//    leftBracket (Left Bracket Shape)
//    leftCircularArrow (Left Circular Arrow Shape)
//    leftRightArrow (Left Right Arrow Shape)
//    leftRightArrowCallout (Callout Left Right Arrow Shape)
//    leftRightCircularArrow (Left Right Circular Arrow Shape)
//    leftRightRibbon (Left Right Ribbon Shape)
//    leftRightUpArrow (Left Right Up Arrow Shape)
//    leftUpArrow (Left Up Arrow Shape)
//    lightningBolt (Lightning Bolt Shape)
//    line (Line Shape)
//    lineInv (Line Inverse Shape)
//    mathDivide (Divide Math Shape)
//    mathEqual (Equal Math Shape)
//    mathMinus (Minus Math Shape)
//    mathMultiply (Multiply Math Shape)
//    mathNotEqual (Not Equal Math Shape)
//    mathPlus (Plus Math Shape)
//    moon (Moon Shape)
//    nonIsoscelesTrapezoid (Non-Isosceles Trapezoid Shape)
//    noSmoking (No Smoking Shape)
//    notchedRightArrow (Notched Right Arrow Shape)
//    octagon (Octagon Shape)
//    parallelogram (Parallelogram Shape)
//    pentagon (Pentagon Shape)
//    pie (Pie Shape)
//    pieWedge (Pie Wedge Shape)
//    plaque (Plaque Shape)
//    plaqueTabs (Plaque Tabs Shape)
//    plus (Plus Shape)
//    quadArrow (Quad-Arrow Shape)
//    quadArrowCallout (Callout Quad-Arrow Shape)
//    rect (Rectangle Shape)
//    ribbon (Ribbon Shape)
//    ribbon2 (Ribbon 2 Shape)
//    rightArrow (Right Arrow Shape)
//    rightArrowCallout (Callout Right Arrow Shape)
//    rightBrace (Right Brace Shape)
//    rightBracket (Right Bracket Shape)
//    round1Rect (One Round Corner Rectangle Shape)
//    round2DiagRect (Two Diagonal Round Corner Rectangle Shape)
//    round2SameRect (Two Same-side Round Corner Rectangle Shape)
//    roundRect (Round Corner Rectangle Shape)
//    rtTriangle (Right Triangle Shape)
//    smileyFace (Smiley Face Shape)
//    snip1Rect (One Snip Corner Rectangle Shape)
//    snip2DiagRect (Two Diagonal Snip Corner Rectangle Shape)
//    snip2SameRect (Two Same-side Snip Corner Rectangle Shape)
//    snipRoundRect (One Snip One Round Corner Rectangle Shape)
//    squareTabs (Square Tabs Shape)
//    star10 (Ten Pointed Star Shape)
//    star12 (Twelve Pointed Star Shape)
//    star16 (Sixteen Pointed Star Shape)
//    star24 (Twenty Four Pointed Star Shape)
//    star32 (Thirty Two Pointed Star Shape)
//    star4 (Four Pointed Star Shape)
//    star5 (Five Pointed Star Shape)
//    star6 (Six Pointed Star Shape)
//    star7 (Seven Pointed Star Shape)
//    star8 (Eight Pointed Star Shape)
//    straightConnector1 (Straight Connector 1 Shape)
//    stripedRightArrow (Striped Right Arrow Shape)
//    sun (Sun Shape)
//    swooshArrow (Swoosh Arrow Shape)
//    teardrop (Teardrop Shape)
//    trapezoid (Trapezoid Shape)
//    triangle (Triangle Shape)
//    upArrow (Up Arrow Shape)
//    upArrowCallout (Callout Up Arrow Shape)
//    upDownArrow (Up Down Arrow Shape)
//    upDownArrowCallout (Callout Up Down Arrow Shape)
//    uturnArrow (U-Turn Arrow Shape)
//    verticalScroll (Vertical Scroll Shape)
//    wave (Wave Shape)
//    wedgeEllipseCallout (Callout Wedge Ellipse Shape)
//    wedgeRectCallout (Callout Wedge Rectangle Shape)
//    wedgeRoundRectCallout (Callout Wedge Round Rectangle Shape)
//
// The following shows the type of text underline supported by excelize:
//
//    none
//    words
//    sng
//    dbl
//    heavy
//    dotted
//    dottedHeavy
//    dash
//    dashHeavy
//    dashLong
//    dashLongHeavy
//    dotDash
//    dotDashHeavy
//    dotDotDash
//    dotDotDashHeavy
//    wavy
//    wavyHeavy
//    wavyDbl
//
func (f *File) AddShape(sheet, cell, format string) error {
	formatSet, err := parseFormatShapeSet(format)
	if err != nil {
		return err
	}
	// Read sheet data.
	xlsx := f.workSheetReader(sheet)
	// Add first shape for given sheet, create xl/drawings/ and xl/drawings/_rels/ folder.
	drawingID := f.countDrawings() + 1
	drawingXML := "xl/drawings/drawing" + strconv.Itoa(drawingID) + ".xml"
	sheetRelationshipsDrawingXML := "../drawings/drawing" + strconv.Itoa(drawingID) + ".xml"

	if xlsx.Drawing != nil {
		// The worksheet already has a shape or chart relationships, use the relationships drawing ../drawings/drawing%d.xml.
		sheetRelationshipsDrawingXML = f.getSheetRelationshipsTargetByID(sheet, xlsx.Drawing.RID)
		drawingID, _ = strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(sheetRelationshipsDrawingXML, "../drawings/drawing"), ".xml"))
		drawingXML = strings.Replace(sheetRelationshipsDrawingXML, "..", "xl", -1)
	} else {
		// Add first shape for given sheet.
		rID := f.addSheetRelationships(sheet, SourceRelationshipDrawingML, sheetRelationshipsDrawingXML, "")
		f.addSheetDrawing(sheet, rID)
	}
	f.addDrawingShape(sheet, drawingXML, cell, formatSet)
	f.addContentTypePart(drawingID, "drawings")
	return err
}

// addDrawingShape provides function to add preset geometry by given sheet,
// drawingXMLand format sets.
func (f *File) addDrawingShape(sheet, drawingXML, cell string, formatSet *formatShape) {
	textUnderlineType := map[string]bool{"none": true, "words": true, "sng": true, "dbl": true, "heavy": true, "dotted": true, "dottedHeavy": true, "dash": true, "dashHeavy": true, "dashLong": true, "dashLongHeavy": true, "dotDash": true, "dotDashHeavy": true, "dotDotDash": true, "dotDotDashHeavy": true, "wavy": true, "wavyHeavy": true, "wavyDbl": true}
	cell = strings.ToUpper(cell)
	fromCol := string(strings.Map(letterOnlyMapF, cell))
	fromRow, _ := strconv.Atoi(strings.Map(intOnlyMapF, cell))
	row := fromRow - 1
	col := TitleToNumber(fromCol)
	width := int(float64(formatSet.Width) * formatSet.Format.XScale)
	height := int(float64(formatSet.Height) * formatSet.Format.YScale)
	colStart, rowStart, _, _, colEnd, rowEnd, x2, y2 := f.positionObjectPixels(sheet, col, row, formatSet.Format.OffsetX, formatSet.Format.OffsetY, width, height)
	content := xlsxWsDr{}
	content.A = NameSpaceDrawingML
	content.Xdr = NameSpaceDrawingMLSpreadSheet
	cNvPrID := f.drawingParser(drawingXML, &content)
	twoCellAnchor := xdrCellAnchor{}
	twoCellAnchor.EditAs = formatSet.Format.Positioning
	from := xlsxFrom{}
	from.Col = colStart
	from.ColOff = formatSet.Format.OffsetX * EMU
	from.Row = rowStart
	from.RowOff = formatSet.Format.OffsetY * EMU
	to := xlsxTo{}
	to.Col = colEnd
	to.ColOff = x2 * EMU
	to.Row = rowEnd
	to.RowOff = y2 * EMU
	twoCellAnchor.From = &from
	twoCellAnchor.To = &to
	shape := xdrSp{
		NvSpPr: &xdrNvSpPr{
			CNvPr: &xlsxCNvPr{
				ID:   cNvPrID,
				Name: "Shape " + strconv.Itoa(cNvPrID),
			},
			CNvSpPr: &xdrCNvSpPr{
				TxBox: true,
			},
		},
		SpPr: &xlsxSpPr{
			PrstGeom: xlsxPrstGeom{
				Prst: formatSet.Type,
			},
		},
		Style: &xdrStyle{
			LnRef:     setShapeRef(formatSet.Color.Line, 2),
			FillRef:   setShapeRef(formatSet.Color.Fill, 1),
			EffectRef: setShapeRef(formatSet.Color.Effect, 0),
			FontRef: &aFontRef{
				Idx: "minor",
				SchemeClr: &attrValString{
					Val: "tx1",
				},
			},
		},
		TxBody: &xdrTxBody{
			BodyPr: &aBodyPr{
				VertOverflow: "clip",
				HorzOverflow: "clip",
				Wrap:         "none",
				RtlCol:       false,
				Anchor:       "t",
			},
		},
	}
	if len(formatSet.Paragraph) < 1 {
		formatSet.Paragraph = []formatShapeParagraph{
			{
				Font: formatFont{
					Bold:      false,
					Italic:    false,
					Underline: "none",
					Family:    "Calibri",
					Size:      11,
					Color:     "#000000",
				},
				Text: " ",
			},
		}
	}
	for _, p := range formatSet.Paragraph {
		u := p.Font.Underline
		_, ok := textUnderlineType[u]
		if !ok {
			u = "none"
		}
		text := p.Text
		if text == "" {
			text = " "
		}
		paragraph := &aP{
			R: &aR{
				RPr: aRPr{
					I:       p.Font.Italic,
					B:       p.Font.Bold,
					Lang:    "en-US",
					AltLang: "en-US",
					U:       u,
					Sz:      p.Font.Size * 100,
					Latin:   &aLatin{Typeface: p.Font.Family},
					SolidFill: &aSolidFill{
						SrgbClr: &attrValString{
							Val: strings.Replace(strings.ToUpper(p.Font.Color), "#", "", -1),
						},
					},
				},
				T: text,
			},
			EndParaRPr: &aEndParaRPr{
				Lang: "en-US",
			},
		}
		shape.TxBody.P = append(shape.TxBody.P, paragraph)
	}
	twoCellAnchor.Sp = &shape
	twoCellAnchor.ClientData = &xdrClientData{
		FLocksWithSheet:  formatSet.Format.FLocksWithSheet,
		FPrintsWithSheet: formatSet.Format.FPrintsWithSheet,
	}
	content.TwoCellAnchor = append(content.TwoCellAnchor, &twoCellAnchor)
	output, _ := xml.Marshal(content)
	f.saveFileList(drawingXML, output)
}

// setShapeRef provides function to set color with hex model by given actual
// color value.
func setShapeRef(color string, i int) *aRef {
	if color == "" {
		return &aRef{
			Idx: 0,
			ScrgbClr: &aScrgbClr{
				R: 0,
				G: 0,
				B: 0,
			},
		}
	}
	return &aRef{
		Idx: i,
		SrgbClr: &attrValString{
			Val: strings.Replace(strings.ToUpper(color), "#", "", -1),
		},
	}
}
