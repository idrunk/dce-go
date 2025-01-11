package util

import (
	"iter"
	"maps"
	"slices"
)

func NewSeq[V any](seq iter.Seq[V]) Sequence[V, V, int8] {
	return NewMapSeq[V, V](seq)
}

func SeqFrom[V any](seq []V) Sequence[V, V, int8] {
	return MapSeqFrom[V, V](seq)
}

func NewMapSeq[V, T any](seq iter.Seq[V]) Sequence[V, T, int8] {
	return Sequence[V, T, int8]{seq: seq}
}

func MapSeqFrom[V, T any](seq []V) Sequence[V, T, int8] {
	return Sequence[V, T, int8]{seq: slices.Values(seq)}
}

func NewSeq2[K comparable, V any](seq2 iter.Seq2[K, V]) Sequence[V, V, K] {
	return NewMapSeq2[K, V, V](seq2)
}

func Seq2From[K comparable, V any](seq2 map[K]V) Sequence[V, V, K] {
	return MapSeq2From[K, V, V](seq2)
}

func NewMapSeq2[K comparable, V, T any](seq2 iter.Seq2[K, V]) Sequence[V, T, K] {
	return Sequence[V, T, K]{seq2: seq2}
}

func MapSeq2From[K comparable, V, T any](seq2 map[K]V) Sequence[V, T, K] {
	return Sequence[V, T, K]{seq2: maps.All(seq2)}
}

type Sequence[V, T any, K comparable] struct {
	seq  iter.Seq[V]
	seq2 iter.Seq2[K, V]
}

func (s Sequence[V, T, K]) Filter(filter func(V) bool) Sequence[V, T, K] {
	return Sequence[V, T, K]{
		seq: func(yield func(V) bool) {
			for v := range s.seq {
				if filter(v) && !yield(v) {
					return
				}
			}
		},
	}
}

func (s Sequence[V, T, K]) Flat() Sequence[T, T, K] {
	return Sequence[T, T, K]{
		seq: func(yield func(T) bool) {
			for v := range s.seq {
				if vv, ok := any(v).([]T); ok {
					for i := 0; i < len(vv); i++ {
						if !yield(vv[i]) {
							return
						}
					}
				}
			}
		},
	}
}

func (s Sequence[V, T, K]) FlatMap(mapper func(V) []T) Sequence[T, T, K] {
	return Sequence[T, T, K]{
		seq: func(yield func(T) bool) {
			for v := range s.seq {
				vv := mapper(v)
				for i := 0; i < len(vv); i++ {
					if !yield(vv[i]) {
						return
					}
				}
			}
		},
	}
}

func (s Sequence[V, T, K]) Map(mapper func(V) T) Sequence[T, T, K] {
	return Sequence[T, T, K]{
		seq: func(yield func(T) bool) {
			for v := range s.seq {
				if !yield(mapper(v)) {
					return
				}
			}
		},
	}
}

func (s Sequence[V, T, K]) Filter2(filter func(K, V) bool) Sequence[V, T, K] {
	return Sequence[V, T, K]{
		seq2: func(yield func(K, V) bool) {
			for k, v := range s.seq2 {
				if filter(k, v) && !yield(k, v) {
					return
				}
			}
		},
	}
}

func (s Sequence[V, T, K]) Map2(mapper func(K, V) T) Sequence[T, T, K] {
	return Sequence[T, T, K]{
		seq: func(yield func(T) bool) {
			for k, v := range s.seq2 {
				if !yield(mapper(k, v)) {
					return
				}
			}
		},
	}
}

func (s Sequence[V, T, K]) Seq() iter.Seq[V] {
	return s.seq
}

func (s Sequence[V, T, K]) Seq2() iter.Seq2[K, V] {
	return s.seq2
}

func (s Sequence[V, T, K]) Contains(t any, equal func(one, other any) bool) bool {
	for v := range s.seq {
		if equal(v, t) {
			return true
		}
	}
	return false
}

func (s Sequence[V, T, K]) Find(finder func(V) bool) (V, bool) {
	for v := range s.seq {
		if finder(v) {
			return v, true
		}
	}
	return NewStruct[V](), false
}

func (s Sequence[V, T, K]) Unique(contains func(set []V, v V) bool) []V {
	var set []V
	for v := range s.seq {
		if !contains(set, v) {
			set = append(set, v)
		}
	}
	return set
}

func (s Sequence[V, T, K]) Collect() []V {
	return slices.Collect(s.seq)
}

func (s Sequence[V, T, K]) Collect2() map[K]V {
	return maps.Collect(s.seq2)
}

func Equal(one, other any) bool {
	return one == other
}

type UniVec[T comparable] []T

func (s *UniVec[T]) Append(items ...T) *UniVec[T] {
	for i := 0; i < len(items); i++ {
		vec := *s
		if !slices.Contains(vec, items[i]) {
			*s = append(vec, items[i])
		}
	}
	return s
}

func Set[T comparable](items ...T) UniVec[T] {
	uv := UniVec[T]{}
	uv.Append(items...)
	return uv
}

func SliceDeleteGet[S ~[]E, E any](s *S, i int, j int) (S, error) {
	slice := *s
	if len(slice) < j {
		return nil, Closed0("Delete index out of range")
	} else if i > j {
		return nil, Closed0("Delete index i cannot be greater than j")
	}
	deleted := slices.Clone(slice[i:j])
	*s = slices.Delete(slice, i, j)
	return deleted, nil
}
