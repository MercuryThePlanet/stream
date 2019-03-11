package stream_test

import (
	op "github.com/MercuryThePlanet/optional"
	"github.com/MercuryThePlanet/stream"
	"testing"
)

func TestStream(t *testing.T) {
	a := stream.NewArray([]interface{}{1, 2, 3, 4, 5})

	stream.Of(a).Map(func(t op.T) op.T {
		return t.(int) * 2
	}).Filter(func(t op.T) bool {
		return t.(int) > 5
	}).ForEach(func(t op.T) {
		println(t.(int))
	})
}
