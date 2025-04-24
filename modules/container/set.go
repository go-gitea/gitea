// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package container

import "maps"

type Set[T comparable] map[T]struct{}

// SetOf creates a set and adds the specified elements to it.
func SetOf[T comparable](values ...T) Set[T] {
	s := make(Set[T], len(values))
	s.AddMultiple(values...)
	return s
}

// Add adds the specified element to a set.
// Returns true if the element is added; false if the element is already present.
func (s Set[T]) Add(value T) bool {
	if _, has := s[value]; !has {
		s[value] = struct{}{}
		return true
	}
	return false
}

// AddMultiple adds the specified elements to a set.
func (s Set[T]) AddMultiple(values ...T) {
	for _, value := range values {
		s.Add(value)
	}
}

// Contains determines whether a set contains all these elements.
// Returns true if the set contains all these elements; otherwise, false.
func (s Set[T]) Contains(values ...T) bool {
	ret := true
	for _, value := range values {
		_, has := s[value]
		ret = ret && has
	}
	return ret
}

// Remove removes the specified element.
// Returns true if the element is successfully found and removed; otherwise, false.
func (s Set[T]) Remove(value T) bool {
	if _, has := s[value]; has {
		delete(s, value)
		return true
	}
	return false
}

// Values gets a list of all elements in the set.
func (s Set[T]) Values() []T {
	keys := make([]T, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	return keys
}

// Union constructs a new set that is the union of the provided sets
func (s Set[T]) Union(sets ...Set[T]) Set[T] {
	newSet := maps.Clone(s)
	for i := range sets {
		maps.Copy(newSet, sets[i])
	}
	return newSet
}
