package builder

import "fmt"

type Incr int

type Decr int

type Eq map[string]interface{}

var _ Cond = Eq{}

func (eq Eq) opWriteTo(op string, w Writer) error {
	var i = 0
	for k, v := range eq {
		switch v.(type) {
		case []int, []int64, []string, []int32, []int16, []int8, []uint, []uint64, []uint32, []uint16, []interface{}:
			if err := In(k, v).WriteTo(w); err != nil {
				return err
			}
		case expr:
			if _, err := fmt.Fprintf(w, "%s=(", k); err != nil {
				return err
			}

			if err := v.(expr).WriteTo(w); err != nil {
				return err
			}

			if _, err := fmt.Fprintf(w, ")"); err != nil {
				return err
			}
		case *Builder:
			if _, err := fmt.Fprintf(w, "%s=(", k); err != nil {
				return err
			}

			if err := v.(*Builder).WriteTo(w); err != nil {
				return err
			}

			if _, err := fmt.Fprintf(w, ")"); err != nil {
				return err
			}
		case Incr:
			if _, err := fmt.Fprintf(w, "%s=%s+?", k, k); err != nil {
				return err
			}
			w.Append(int(v.(Incr)))
		case Decr:
			if _, err := fmt.Fprintf(w, "%s=%s-?", k, k); err != nil {
				return err
			}
			w.Append(int(v.(Decr)))
		default:
			if _, err := fmt.Fprintf(w, "%s=?", k); err != nil {
				return err
			}
			w.Append(v)
		}
		if i != len(eq)-1 {
			if _, err := fmt.Fprint(w, op); err != nil {
				return err
			}
		}
		i = i + 1
	}
	return nil
}

func (eq Eq) WriteTo(w Writer) error {
	return eq.opWriteTo(" AND ", w)
}

func (eq Eq) And(conds ...Cond) Cond {
	return And(eq, And(conds...))
}

func (eq Eq) Or(conds ...Cond) Cond {
	return Or(eq, Or(conds...))
}

func (eq Eq) IsValid() bool {
	return len(eq) > 0
}
