package stream

import (
	"fmt"
	op "github.com/MercuryThePlanet/optional"
)

func T() {
	fmt.Println("vim-go")
}

type Spliterator interface {
	TryAdvance(op.Consumer) bool
	ForEachRemaining(op.Consumer)
	TrySplit() Spliterator
}

type Array struct {
	ts  op.Ts
	idx int
}

func NewArray(ts op.Ts) *Array {
	return &Array{ts, 0}
}

func (a *Array) TryAdvance(c op.Consumer) bool {
	if a.idx >= len(a.ts) {
		return false
	}
	c(a.ts[a.idx])
	a.idx++
	return true
}

func (a *Array) ForEachRemaining(c op.Consumer) {
	for a.TryAdvance(c) {
	}
}

func (a *Array) TrySplit() (s Spliterator) {
	remain := len(a.ts) - a.idx
	if remain > 1 {
		var split int = (remain) / 2
		s = &Array{ts: a.ts[split:], idx: 0}
		a.ts = a.ts[:split]
		return
	}
	return nil
}

type (
	Mapper    func(op.T) op.T
	Consumer  func(op.T)
	Predicate func(op.T) bool
)

type Stream struct {
	pipeline func(op.T, Consumer)
	spltr    Spliterator
}

func Of(s Spliterator) *Stream {
	return &Stream{func(t op.T, c Consumer) { c(t) }, s}
}

func (s *Stream) Filter(p Predicate) *Stream {
	cur := s.pipeline
	s.pipeline = func(t op.T, c Consumer) {
		cur(t, func(t op.T) {
			if p(t) {
				c(t)
			}
		})
	}
	return s
}

func (s *Stream) Map(m Mapper) *Stream {
	cur := s.pipeline
	s.pipeline = func(t op.T, c Consumer) {
		cur(t, func(t op.T) {
			c(m(t))
		})
	}
	return s
}

func (s *Stream) ForEach(c Consumer) {
	for s.spltr.TryAdvance(func(t op.T) {
		s.pipeline(t, c)
	}) {
	}
}
