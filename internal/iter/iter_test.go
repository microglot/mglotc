package iter

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"gopkg.microglot.org/compiler.go/internal/idl"
)

type elem struct {
	value int
}

func TestLookahead(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	numValues := 10

	for x := 0; x < numValues; x = x + 1 {
		t.Run(fmt.Sprintf("LA(%d)", x), func(t *testing.T) {
			elems := make([]*elem, 0, numValues)
			for y := 0; y < numValues; y = y + 1 {
				elems = append(elems, &elem{value: y})
			}
			iter := NewSlice(elems)
			look := NewLookahead(iter, uint8(x))
			for y := 0; y < numValues; y = y + 1 {
				val := look.Next(ctx)
				require.NotNil(t, val)
				require.True(t, val.IsPresent())
				expected := y
				require.Equal(t, expected, val.Value().value)

				expectedPeek := y + x
				expectedPeekOK := expectedPeek < numValues
				peek := look.Lookahead(ctx, uint8(x))
				if expectedPeekOK {
					require.True(t, peek.IsPresent())
					require.Equal(t, expectedPeek, peek.Value().value)
				} else {
					require.False(t, peek.IsPresent())
				}
			}
			require.Nil(t, look.Close(ctx))
		})
	}
}

func TestLookaheadFilter(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	numValues := 10
	filter := idl.Filter[*elem](FilterFunc[*elem](func(ctx context.Context, val *elem) bool {
		return val.value%2 == 0
	}))
	for x := 0; x < numValues/2; x = x + 1 {
		t.Run(fmt.Sprintf("LA(%d)", x), func(t *testing.T) {
			elems := make([]*elem, 0, numValues)
			for y := 0; y < numValues; y = y + 1 {
				elems = append(elems, &elem{value: y})
			}
			iter := NewSlice(elems)
			iter = NewIteratorFilter(iter, filter)
			look := NewLookahead(iter, uint8(x))
			for y := 0; y < numValues/2; y = y + 2 {
				val := look.Next(ctx)
				require.NotNil(t, val)
				require.True(t, val.IsPresent())
				expected := y
				require.Equal(t, expected, val.Value().value)

				expectedPeek := y + (x * 2)
				expectedPeekOK := expectedPeek < numValues
				peek := look.Lookahead(ctx, uint8(x))
				if expectedPeekOK {
					require.True(t, peek.IsPresent())
					require.Equal(t, expectedPeek, peek.Value().value)
				} else {
					require.False(t, peek.IsPresent())
				}
			}
			require.Nil(t, look.Close(ctx))
		})
	}
}

var benchEscapeValue *elem
var benchEscapeValuePeek *elem

func BenchmarkLookahead(b *testing.B) {
	ctx := context.Background()
	sliceSize := 1000
	slice := make([]*elem, sliceSize)
	for x := 0; x < sliceSize; x = x + 1 {
		slice[x] = &elem{value: x}
	}
	iter := NewSlice(slice)
	look := NewLookahead(iter, 1)

	var loopEscapeValue *elem
	var loopEscapeValuePeek *elem
	b.ResetTimer()
	for n := 0; n < b.N; n = n + 1 {
		for x := 0; x < sliceSize; x = x + 1 {
			loopEscapeValue = look.Next(ctx).Value()
			loopEscapeValuePeek = look.Lookahead(ctx, 1).Value()
		}
	}
	benchEscapeValue = loopEscapeValue
	benchEscapeValuePeek = loopEscapeValuePeek
}
