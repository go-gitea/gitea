package builder

import "fmt"

// IsNull
type IsNull [1]string

var _ Cond = IsNull{""}

func (isNull IsNull) WriteTo(w Writer) error {
	_, err := fmt.Fprintf(w, "%s IS NULL", isNull[0])
	return err
}

func (isNull IsNull) And(conds ...Cond) Cond {
	return And(isNull, And(conds...))
}

func (isNull IsNull) Or(conds ...Cond) Cond {
	return Or(isNull, Or(conds...))
}

func (isNull IsNull) IsValid() bool {
	return len(isNull[0]) > 0
}

// NotNull
type NotNull [1]string

var _ Cond = NotNull{""}

func (notNull NotNull) WriteTo(w Writer) error {
	_, err := fmt.Fprintf(w, "%s IS NOT NULL", notNull[0])
	return err
}

func (notNull NotNull) And(conds ...Cond) Cond {
	return And(notNull, And(conds...))
}

func (notNull NotNull) Or(conds ...Cond) Cond {
	return Or(notNull, Or(conds...))
}

func (notNull NotNull) IsValid() bool {
	return len(notNull[0]) > 0
}
