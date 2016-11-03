package builder

import "fmt"

type condOr []Cond

var _ Cond = condOr{}

func Or(conds ...Cond) Cond {
	var result = make(condOr, 0, len(conds))
	for _, cond := range conds {
		if cond == nil || !cond.IsValid() {
			continue
		}
		result = append(result, cond)
	}
	return result
}

func (or condOr) WriteTo(w Writer) error {
	for i, cond := range or {
		var needQuote bool
		switch cond.(type) {
		case condAnd:
			needQuote = true
		case Eq:
			needQuote = (len(cond.(Eq)) > 1)
		}

		if needQuote {
			fmt.Fprint(w, "(")
		}

		err := cond.WriteTo(w)
		if err != nil {
			return err
		}

		if needQuote {
			fmt.Fprint(w, ")")
		}

		if i != len(or)-1 {
			fmt.Fprint(w, " OR ")
		}
	}

	return nil
}

func (o condOr) And(conds ...Cond) Cond {
	return And(o, And(conds...))
}

func (o condOr) Or(conds ...Cond) Cond {
	return Or(o, Or(conds...))
}

func (o condOr) IsValid() bool {
	return len(o) > 0
}
