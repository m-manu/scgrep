package main

type set[T comparable] map[T]struct{}

func newSet[T comparable](size int) set[T] {
	return make(set[T], size)
}

func (s set[T]) add(entry T) {
	s[entry] = struct{}{}
}

func (s set[T]) contains(entry T) bool {
	_, b := s[entry]
	return b
}

func (s set[T]) isEmpty() bool {
	return len(s) == 0
}
