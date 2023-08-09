package iter

import (
	"context"

	"gopkg.microglot.org/compiler.go/internal/idl"
	"gopkg.microglot.org/compiler.go/internal/optional"
)

// NewSlice converts a slice of values into an Iterator implementation.
func NewSlice[T any](vs []T) idl.Iterator[T] {
	return &iteratorSlice[T]{slice: vs, offset: -1}
}

type iteratorSlice[T any] struct {
	slice  []T
	offset int
}

func (it *iteratorSlice[T]) Next(ctx context.Context) optional.Optional[T] {
	it.offset = it.offset + 1
	if it.offset >= len(it.slice) {
		return optional.None[T]()
	}
	return optional.Some(it.slice[it.offset])
}

func (it *iteratorSlice[T]) Close(ctx context.Context) error {
	return nil
}

// NewIteratorFilter wraps an iterator with a filter so that only values that
// pass the filter are returned.
func NewIteratorFilter[T any](it idl.Iterator[T], f idl.Filter[T]) idl.Iterator[T] {
	return &iteratorFilter[T]{
		iter:   it,
		filter: f,
	}
}

type iteratorFilter[T any] struct {
	iter   idl.Iterator[T]
	filter idl.Filter[T]
}

func (it *iteratorFilter[T]) Next(ctx context.Context) optional.Optional[T] {
	for {
		v := it.iter.Next(ctx)
		if !v.IsPresent() {
			return v
		}
		if it.filter.Keep(ctx, v.Value()) {
			return v
		}
	}
}

func (it *iteratorFilter[T]) Close(ctx context.Context) error {
	return it.iter.Close(ctx)
}

// NewLookahead wraps an iterator in a Lookahead implementation to enable
// peeking at the next n values.
func NewLookahead[T any](it idl.Iterator[T], n uint8) idl.Lookahead[T] {
	return &lookahead[T]{
		iter: it,
		n:    n,
	}
}

type lookahead[T any] struct {
	iter  idl.Iterator[T]
	n     uint8
	peeks []optional.Optional[T]
}

func (look *lookahead[T]) init(ctx context.Context) {
	if look.peeks == nil {
		look.peeks = make([]optional.Optional[T], look.n+1)
		for x := 0; x <= int(look.n); x = x + 1 {
			look.peeks[x] = look.iter.Next(ctx)
		}
	}
}

func (look *lookahead[T]) Next(ctx context.Context) optional.Optional[T] {
	if look.peeks == nil {
		look.init(ctx)
		return look.peeks[0]
	}
	copy(look.peeks, look.peeks[1:])
	look.peeks[len(look.peeks)-1] = look.iter.Next(ctx)
	return look.peeks[0]
}
func (look *lookahead[T]) Close(ctx context.Context) error {
	return look.iter.Close(ctx)
}
func (look *lookahead[T]) Lookahead(ctx context.Context, n uint8) optional.Optional[T] {
	if look.peeks == nil {
		look.init(ctx)
	}
	if n > look.n {
		return optional.None[T]()
	}
	return look.peeks[n]
}

// FilterFunc is an adaptor for simple filter functions that makes them
// compatible with the Filter interface. Use like:
//
//	FilterFunc[T](func(ctx context.Context, val T) bool { return true })
//
// Note that this type should never be referenced directly in any signature.
// Always use Filter as an input or output type.
type FilterFunc[T any] func(ctx context.Context, val T) bool

func (f FilterFunc[T]) Keep(ctx context.Context, val T) bool {
	return f(ctx, val)
}
