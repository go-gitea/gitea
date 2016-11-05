package builder

import "fmt"

// WriteMap
func WriteMap(w Writer, data map[string]interface{}, op string) error {
	var args = make([]interface{}, 0, len(data))
	var i = 0
	for k, v := range data {
		switch v.(type) {
		case expr:
			if _, err := fmt.Fprintf(w, "%s%s(", k, op); err != nil {
				return err
			}

			if err := v.(expr).WriteTo(w); err != nil {
				return err
			}

			if _, err := fmt.Fprintf(w, ")"); err != nil {
				return err
			}
		case *Builder:
			if _, err := fmt.Fprintf(w, "%s%s(", k, op); err != nil {
				return err
			}

			if err := v.(*Builder).WriteTo(w); err != nil {
				return err
			}

			if _, err := fmt.Fprintf(w, ")"); err != nil {
				return err
			}
		default:
			if _, err := fmt.Fprintf(w, "%s%s?", k, op); err != nil {
				return err
			}
			args = append(args, v)
		}
		if i != len(data)-1 {
			if _, err := fmt.Fprint(w, " AND "); err != nil {
				return err
			}
		}
		i = i + 1
	}
	w.Append(args...)
	return nil
}

// Lt
type Lt map[string]interface{}

var _ Cond = Lt{}

func (lt Lt) WriteTo(w Writer) error {
	return WriteMap(w, lt, "<")
}

func (lt Lt) And(conds ...Cond) Cond {
	return condAnd{lt, And(conds...)}
}

func (lt Lt) Or(conds ...Cond) Cond {
	return condOr{lt, Or(conds...)}
}

func (lt Lt) IsValid() bool {
	return len(lt) > 0
}

// Lte
type Lte map[string]interface{}

var _ Cond = Lte{}

func (lte Lte) WriteTo(w Writer) error {
	return WriteMap(w, lte, "<=")
}

func (lte Lte) And(conds ...Cond) Cond {
	return And(lte, And(conds...))
}

func (lte Lte) Or(conds ...Cond) Cond {
	return Or(lte, Or(conds...))
}

func (lte Lte) IsValid() bool {
	return len(lte) > 0
}

// Gt
type Gt map[string]interface{}

var _ Cond = Gt{}

func (gt Gt) WriteTo(w Writer) error {
	return WriteMap(w, gt, ">")
}

func (gt Gt) And(conds ...Cond) Cond {
	return And(gt, And(conds...))
}

func (gt Gt) Or(conds ...Cond) Cond {
	return Or(gt, Or(conds...))
}

func (gt Gt) IsValid() bool {
	return len(gt) > 0
}

// Gte
type Gte map[string]interface{}

var _ Cond = Gte{}

func (gte Gte) WriteTo(w Writer) error {
	return WriteMap(w, gte, ">=")
}

func (gte Gte) And(conds ...Cond) Cond {
	return And(gte, And(conds...))
}

func (gte Gte) Or(conds ...Cond) Cond {
	return Or(gte, Or(conds...))
}

func (gte Gte) IsValid() bool {
	return len(gte) > 0
}
