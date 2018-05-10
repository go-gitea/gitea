package rst

/*
Elements are not allocated one at a time, but in rows of
elemHeap.RowSize elements. After N elements have been
requested, a row is exhausted, and the next one will
be allocated. Previously allocated rows are tracked in
elemHeap.rows.

The Reset() method allows to reset the current position (row, and
position within the row), which allows reusing elements. Whether
elements can be reused, depends on the value of the hasGlobals
field.
*/

type elemHeap struct {
	rows [][]element
	heapPos
	rowSize int

	base       heapPos
	hasGlobals bool
}

type heapPos struct {
	iRow int
	row  []element
}

func (h *elemHeap) nextRow() []element {
	h.iRow++
	if h.iRow == len(h.rows) {
		h.rows = append(h.rows, make([]element, h.rowSize))
	}
	h.row = h.rows[h.iRow]
	return h.row
}

func (h *elemHeap) init(size int) {
	h.rowSize = size
	h.rows = [][]element{make([]element, size)}
	h.row = h.rows[h.iRow]
	h.base = h.heapPos
}

func (h *elemHeap) Reset() {
	if !h.hasGlobals {
		h.heapPos = h.base
	} else {
		/* Don't restore saved position in case elements added
		 * after the previous Reset call are needed in
		 * global context, like notes.
		 */
		h.hasGlobals = false
		h.base = h.heapPos
	}
}
