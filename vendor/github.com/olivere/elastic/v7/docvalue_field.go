// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

// DocvalueField represents a docvalue field, its name and
// its format (optional).
type DocvalueField struct {
	Field  string
	Format string
}

// Source serializes the DocvalueField into JSON.
func (d DocvalueField) Source() (interface{}, error) {
	if d.Format == "" {
		return d.Field, nil
	}
	return map[string]interface{}{
		"field":  d.Field,
		"format": d.Format,
	}, nil
}

// DocvalueFields is a slice of DocvalueField instances.
type DocvalueFields []DocvalueField

// Source serializes the DocvalueFields into JSON.
func (d DocvalueFields) Source() (interface{}, error) {
	if d == nil {
		return nil, nil
	}
	v := make([]interface{}, 0)
	for _, f := range d {
		src, err := f.Source()
		if err != nil {
			return nil, err
		}
		v = append(v, src)
	}
	return v, nil
}
