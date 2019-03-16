package stream_test

import (
	op "github.com/MercuryThePlanet/optional"
	"github.com/MercuryThePlanet/stream"
	"testing"
)

func TestStream(t *testing.T) {
	gen := func() *stream.Array {
		return stream.NewArray(makeRange(1, 10))
	}

	stream.Of(gen()).Map(func(t op.T) op.T {
		return t.(int) * 2
	}).Filter(func(t op.T) bool {
		return t.(int) > 1
	}).FindAny()

	matches := stream.Of(gen()).AnyMatch(func(t op.T) bool {
		return t.(int) > 1
	})
	println(matches)
}

func makeRange(min, max int) []interface{} {
	a := make([]interface{}, max-min+1)
	for i := range a {
		a[i] = min + i
	}
	return a
}
