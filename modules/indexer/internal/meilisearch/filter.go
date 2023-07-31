// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package meilisearch

import (
	"fmt"
	"strings"
)

// Filter represents a filter for meilisearch queries.
// It's just a simple wrapper around a string.
// DO NOT assume that it is a complete implementation.
type Filter interface {
	Statement() string
}

type FilterAnd struct {
	filters []Filter
}

func (f *FilterAnd) Statement() string {
	var statements []string
	for _, filter := range f.filters {
		if s := filter.Statement(); s != "" {
			statements = append(statements, fmt.Sprintf("(%s)", s))
		}
	}
	return strings.Join(statements, " AND ")
}

func (f *FilterAnd) And(filter Filter) *FilterAnd {
	f.filters = append(f.filters, filter)
	return f
}

type FilterOr struct {
	filters []Filter
}

func (f *FilterOr) Statement() string {
	var statements []string
	for _, filter := range f.filters {
		if s := filter.Statement(); s != "" {
			statements = append(statements, fmt.Sprintf("(%s)", s))
		}
	}
	return strings.Join(statements, " OR ")
}

func (f *FilterOr) Or(filter Filter) *FilterOr {
	f.filters = append(f.filters, filter)
	return f
}

type FilterIn string

// NewFilterIn creates a new FilterIn.
// It supports int64 only, to avoid extra works to handle strings with special characters.
func NewFilterIn[T int64](field string, values ...T) FilterIn {
	if len(values) == 0 {
		return ""
	}
	vs := make([]string, len(values))
	for i, v := range values {
		vs[i] = fmt.Sprintf("%v", v)
	}
	return FilterIn(fmt.Sprintf("%s IN [%v]", field, strings.Join(vs, ", ")))
}

func (f FilterIn) Statement() string {
	return string(f)
}

type FilterEq string

// NewFilterEq creates a new FilterEq.
// It supports int64 and bool only, to avoid extra works to handle strings with special characters.
func NewFilterEq[T bool | int64](field string, value T) FilterEq {
	return FilterEq(fmt.Sprintf("%s = %v", field, value))
}

func (f FilterEq) Statement() string {
	return string(f)
}

type FilterNot string

func NewFilterNot(filter Filter) FilterNot {
	return FilterNot(fmt.Sprintf("NOT (%s)", filter.Statement()))
}

func (f FilterNot) Statement() string {
	return string(f)
}

type FilterGte string

// NewFilterGte creates a new FilterGte.
// It supports int64 only, to avoid extra works to handle strings with special characters.
func NewFilterGte[T int64](field string, value T) FilterGte {
	return FilterGte(fmt.Sprintf("%s >= %v", field, value))
}

func (f FilterGte) Statement() string {
	return string(f)
}

type FilterLte string

// NewFilterLte creates a new FilterLte.
// It supports int64 only, to avoid extra works to handle strings with special characters.
func NewFilterLte[T int64](field string, value T) FilterLte {
	return FilterLte(fmt.Sprintf("%s <= %v", field, value))
}

func (f FilterLte) Statement() string {
	return string(f)
}
