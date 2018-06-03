package excelize

import "fmt"

// SheetViewOption is an option of a view of a worksheet. See SetSheetViewOptions().
type SheetViewOption interface {
	setSheetViewOption(view *xlsxSheetView)
}

// SheetViewOptionPtr is a writable SheetViewOption. See GetSheetViewOptions().
type SheetViewOptionPtr interface {
	SheetViewOption
	getSheetViewOption(view *xlsxSheetView)
}

type (
	// DefaultGridColor is a SheetViewOption.
	DefaultGridColor bool
	// RightToLeft is a SheetViewOption.
	RightToLeft bool
	// ShowFormulas is a SheetViewOption.
	ShowFormulas bool
	// ShowGridLines is a SheetViewOption.
	ShowGridLines bool
	// ShowRowColHeaders is a SheetViewOption.
	ShowRowColHeaders bool
	// ZoomScale is a SheetViewOption.
	ZoomScale float64
	/* TODO
	// ShowWhiteSpace is a SheetViewOption.
	ShowWhiteSpace bool
	// ShowZeros is a SheetViewOption.
	ShowZeros bool
	// WindowProtection is a SheetViewOption.
	WindowProtection bool
	*/
)

// Defaults for each option are described in XML schema for CT_SheetView

func (o DefaultGridColor) setSheetViewOption(view *xlsxSheetView) {
	view.DefaultGridColor = boolPtr(bool(o))
}

func (o *DefaultGridColor) getSheetViewOption(view *xlsxSheetView) {
	*o = DefaultGridColor(defaultTrue(view.DefaultGridColor)) // Excel default: true
}

func (o RightToLeft) setSheetViewOption(view *xlsxSheetView) {
	view.RightToLeft = bool(o) // Excel default: false
}

func (o *RightToLeft) getSheetViewOption(view *xlsxSheetView) {
	*o = RightToLeft(view.RightToLeft)
}

func (o ShowFormulas) setSheetViewOption(view *xlsxSheetView) {
	view.ShowFormulas = bool(o) // Excel default: false
}

func (o *ShowFormulas) getSheetViewOption(view *xlsxSheetView) {
	*o = ShowFormulas(view.ShowFormulas) // Excel default: false
}

func (o ShowGridLines) setSheetViewOption(view *xlsxSheetView) {
	view.ShowGridLines = boolPtr(bool(o))
}

func (o *ShowGridLines) getSheetViewOption(view *xlsxSheetView) {
	*o = ShowGridLines(defaultTrue(view.ShowGridLines)) // Excel default: true
}

func (o ShowRowColHeaders) setSheetViewOption(view *xlsxSheetView) {
	view.ShowRowColHeaders = boolPtr(bool(o))
}

func (o *ShowRowColHeaders) getSheetViewOption(view *xlsxSheetView) {
	*o = ShowRowColHeaders(defaultTrue(view.ShowRowColHeaders)) // Excel default: true
}

func (o ZoomScale) setSheetViewOption(view *xlsxSheetView) {
	//This attribute is restricted to values ranging from 10 to 400.
	if float64(o) >= 10 && float64(o) <= 400 {
		view.ZoomScale = float64(o)
	}
}

func (o *ZoomScale) getSheetViewOption(view *xlsxSheetView) {
	*o = ZoomScale(view.ZoomScale)
}

// getSheetView returns the SheetView object
func (f *File) getSheetView(sheetName string, viewIndex int) (*xlsxSheetView, error) {
	xlsx := f.workSheetReader(sheetName)
	if viewIndex < 0 {
		if viewIndex < -len(xlsx.SheetViews.SheetView) {
			return nil, fmt.Errorf("view index %d out of range", viewIndex)
		}
		viewIndex = len(xlsx.SheetViews.SheetView) + viewIndex
	} else if viewIndex >= len(xlsx.SheetViews.SheetView) {
		return nil, fmt.Errorf("view index %d out of range", viewIndex)
	}

	return &(xlsx.SheetViews.SheetView[viewIndex]), nil
}

// SetSheetViewOptions sets sheet view options.
// The viewIndex may be negative and if so is counted backward (-1 is the last view).
//
// Available options:
//    DefaultGridColor(bool)
//    RightToLeft(bool)
//    ShowFormulas(bool)
//    ShowGridLines(bool)
//    ShowRowColHeaders(bool)
// Example:
//    err = f.SetSheetViewOptions("Sheet1", -1, ShowGridLines(false))
func (f *File) SetSheetViewOptions(name string, viewIndex int, opts ...SheetViewOption) error {
	view, err := f.getSheetView(name, viewIndex)
	if err != nil {
		return err
	}

	for _, opt := range opts {
		opt.setSheetViewOption(view)
	}
	return nil
}

// GetSheetViewOptions gets the value of sheet view options.
// The viewIndex may be negative and if so is counted backward (-1 is the last view).
//
// Available options:
//    DefaultGridColor(bool)
//    RightToLeft(bool)
//    ShowFormulas(bool)
//    ShowGridLines(bool)
//    ShowRowColHeaders(bool)
// Example:
//    var showGridLines excelize.ShowGridLines
//    err = f.GetSheetViewOptions("Sheet1", -1, &showGridLines)
func (f *File) GetSheetViewOptions(name string, viewIndex int, opts ...SheetViewOptionPtr) error {
	view, err := f.getSheetView(name, viewIndex)
	if err != nil {
		return err
	}

	for _, opt := range opts {
		opt.getSheetViewOption(view)
	}
	return nil
}
