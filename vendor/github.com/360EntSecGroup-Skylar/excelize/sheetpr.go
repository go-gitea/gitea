package excelize

// SheetPrOption is an option of a view of a worksheet. See SetSheetPrOptions().
type SheetPrOption interface {
	setSheetPrOption(view *xlsxSheetPr)
}

// SheetPrOptionPtr is a writable SheetPrOption. See GetSheetPrOptions().
type SheetPrOptionPtr interface {
	SheetPrOption
	getSheetPrOption(view *xlsxSheetPr)
}

type (
	// CodeName is a SheetPrOption
	CodeName string
	// EnableFormatConditionsCalculation is a SheetPrOption
	EnableFormatConditionsCalculation bool
	// Published is a SheetPrOption
	Published bool
	// FitToPage is a SheetPrOption
	FitToPage bool
	// AutoPageBreaks is a SheetPrOption
	AutoPageBreaks bool
)

func (o CodeName) setSheetPrOption(pr *xlsxSheetPr) {
	pr.CodeName = string(o)
}

func (o *CodeName) getSheetPrOption(pr *xlsxSheetPr) {
	if pr == nil {
		*o = ""
		return
	}
	*o = CodeName(pr.CodeName)
}

func (o EnableFormatConditionsCalculation) setSheetPrOption(pr *xlsxSheetPr) {
	pr.EnableFormatConditionsCalculation = boolPtr(bool(o))
}

func (o *EnableFormatConditionsCalculation) getSheetPrOption(pr *xlsxSheetPr) {
	if pr == nil {
		*o = true
		return
	}
	*o = EnableFormatConditionsCalculation(defaultTrue(pr.EnableFormatConditionsCalculation))
}

func (o Published) setSheetPrOption(pr *xlsxSheetPr) {
	pr.Published = boolPtr(bool(o))
}

func (o *Published) getSheetPrOption(pr *xlsxSheetPr) {
	if pr == nil {
		*o = true
		return
	}
	*o = Published(defaultTrue(pr.Published))
}

func (o FitToPage) setSheetPrOption(pr *xlsxSheetPr) {
	if pr.PageSetUpPr == nil {
		if !o {
			return
		}
		pr.PageSetUpPr = new(xlsxPageSetUpPr)
	}
	pr.PageSetUpPr.FitToPage = bool(o)
}

func (o *FitToPage) getSheetPrOption(pr *xlsxSheetPr) {
	// Excel default: false
	if pr == nil || pr.PageSetUpPr == nil {
		*o = false
		return
	}
	*o = FitToPage(pr.PageSetUpPr.FitToPage)
}

func (o AutoPageBreaks) setSheetPrOption(pr *xlsxSheetPr) {
	if pr.PageSetUpPr == nil {
		if !o {
			return
		}
		pr.PageSetUpPr = new(xlsxPageSetUpPr)
	}
	pr.PageSetUpPr.AutoPageBreaks = bool(o)
}

func (o *AutoPageBreaks) getSheetPrOption(pr *xlsxSheetPr) {
	// Excel default: false
	if pr == nil || pr.PageSetUpPr == nil {
		*o = false
		return
	}
	*o = AutoPageBreaks(pr.PageSetUpPr.AutoPageBreaks)
}

// SetSheetPrOptions provides function to sets worksheet properties.
//
// Available options:
//   CodeName(string)
//   EnableFormatConditionsCalculation(bool)
//   Published(bool)
//   FitToPage(bool)
//   AutoPageBreaks(bool)
func (f *File) SetSheetPrOptions(name string, opts ...SheetPrOption) error {
	sheet := f.workSheetReader(name)
	pr := sheet.SheetPr
	if pr == nil {
		pr = new(xlsxSheetPr)
		sheet.SheetPr = pr
	}

	for _, opt := range opts {
		opt.setSheetPrOption(pr)
	}
	return nil
}

// GetSheetPrOptions provides function to gets worksheet properties.
//
// Available options:
//   CodeName(string)
//   EnableFormatConditionsCalculation(bool)
//   Published(bool)
//   FitToPage(bool)
//   AutoPageBreaks(bool)
func (f *File) GetSheetPrOptions(name string, opts ...SheetPrOptionPtr) error {
	sheet := f.workSheetReader(name)
	pr := sheet.SheetPr

	for _, opt := range opts {
		opt.getSheetPrOption(pr)
	}
	return nil
}
