package builder

import "fmt"

type expr struct {
	sql  string
	args []interface{}
}

var _ Cond = expr{}

func Expr(sql string, args ...interface{}) Cond {
	return expr{sql, args}
}

func (expr expr) WriteTo(w Writer) error {
	if _, err := fmt.Fprint(w, expr.sql); err != nil {
		return err
	}
	w.Append(expr.args...)
	return nil
}

func (expr expr) And(conds ...Cond) Cond {
	return And(expr, And(conds...))
}

func (expr expr) Or(conds ...Cond) Cond {
	return Or(expr, Or(conds...))
}

func (expr expr) IsValid() bool {
	return len(expr.sql) > 0
}
