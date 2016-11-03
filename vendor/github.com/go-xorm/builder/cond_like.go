package builder

import "fmt"

type Like [2]string

var _ Cond = Like{"", ""}

func (like Like) WriteTo(w Writer) error {
	if _, err := fmt.Fprintf(w, "%s LIKE ?", like[0]); err != nil {
		return err
	}
	w.Append("%" + like[1] + "%")
	return nil
}

func (like Like) And(conds ...Cond) Cond {
	return And(like, And(conds...))
}

func (like Like) Or(conds ...Cond) Cond {
	return Or(like, Or(conds...))
}

func (like Like) IsValid() bool {
	return len(like[0]) > 0 && len(like[1]) > 0
}
