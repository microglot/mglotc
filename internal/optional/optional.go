package optional

type Optional[T any] struct {
	present bool
	value   T
}

func (self Optional[T]) IsPresent() bool {
	return self.present
}

func (self Optional[T]) Value() T {
	return self.value
}

func Some[T any](v T) Optional[T] {
	return Optional[T]{
		present: true,
		value:   v,
	}
}

func None[T any]() Optional[T] {
	return Optional[T]{}
}
