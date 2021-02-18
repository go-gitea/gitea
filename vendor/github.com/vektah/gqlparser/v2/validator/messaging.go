package validator

import "bytes"

// Given [ A, B, C ] return '"A", "B", or "C"'.
func QuotedOrList(items ...string) string {
	itemsQuoted := make([]string, len(items))
	for i, item := range items {
		itemsQuoted[i] = `"` + item + `"`
	}
	return OrList(itemsQuoted...)
}

// Given [ A, B, C ] return 'A, B, or C'.
func OrList(items ...string) string {
	var buf bytes.Buffer

	if len(items) > 5 {
		items = items[:5]
	}
	if len(items) == 2 {
		buf.WriteString(items[0])
		buf.WriteString(" or ")
		buf.WriteString(items[1])
		return buf.String()
	}

	for i, item := range items {
		if i != 0 {
			if i == len(items)-1 {
				buf.WriteString(", or ")
			} else {
				buf.WriteString(", ")
			}
		}
		buf.WriteString(item)
	}
	return buf.String()
}
