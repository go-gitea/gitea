package builder

import "fmt"

type Not [1]Cond

var _ Cond = Not{}

func (not Not) WriteTo(w Writer) error {
	if _, err := fmt.Fprint(w, "NOT "); err != nil {
		return err
	}
	switch not[0].(type) {
	case condAnd, condOr:
		if _, err := fmt.Fprint(w, "("); err != nil {
			return err
		}
	}

	if err := not[0].WriteTo(w); err != nil {
		return err
	}

	switch not[0].(type) {
	case condAnd, condOr:
		if _, err := fmt.Fprint(w, ")"); err != nil {
			return err
		}
	}

	return nil
}

func (not Not) And(conds ...Cond) Cond {
	return And(not, And(conds...))
}

func (not Not) Or(conds ...Cond) Cond {
	return Or(not, Or(conds...))
}

func (not Not) IsValid() bool {
	return not[0] != nil && not[0].IsValid()
}
