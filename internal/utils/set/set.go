package set

import "iter"

type Set[T comparable] map[T]struct{}

func New[T comparable]() Set[T] {
	return make(Set[T])
}

func FromSlice[T comparable](s []T) Set[T] {
	set := make(Set[T], len(s))
	for _, v := range s {
		set[v] = struct{}{}
	}
	return set
}

func FromSeq[T comparable](seq iter.Seq[T]) Set[T] {
	set := make(Set[T])
	for v := range seq {
		set[v] = struct{}{}
	}
	return set
}

func (s Set[T]) Add(elem T) {
	s[elem] = struct{}{}
}

func (s Set[T]) Remove(elem T) {
	delete(s, elem)
}

func (s Set[T]) Intersection(other Set[T]) Set[T] {
	out := make(Set[T])
	for v := range s {
		if _, ok := other[v]; ok {
			out[v] = struct{}{}
		}
	}
	return out
}

func (s Set[T]) Difference(other Set[T]) Set[T] {
	out := make(Set[T])
	for v := range s {
		if _, ok := other[v]; !ok {
			out[v] = struct{}{}
		}
	}
	return out
}

func (s Set[T]) Slice() []T {
	out := make([]T, 0, len(s))
	for v := range s {
		out = append(out, v)
	}
	return out
}
