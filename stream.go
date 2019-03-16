package stream

import (
	"fmt"
	op "github.com/MercuryThePlanet/optional"
	"runtime"
)

var (
	CORES = runtime.NumCPU()
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

func (s *Stream) Limit(size int) *Stream {
	cur := s.pipeline
	s.pipeline = func(t op.T, c Consumer) {
		cur(t, func(t op.T) {
			c(t)
		})
	}
	return s
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

func (s *Stream) AllMatch(p Predicate) (matches bool) {
	matches = true
	for s.spltr.TryAdvance(func(t op.T) {
		s.pipeline(t, func(t op.T) {
			matches = p(t)
		})
	}) {
		if !matches {
			break
		}
	}
	return
}

func (s *Stream) AnyMatch(p Predicate) bool {
	rv_chan := make(chan bool)

	find := func(spltr Spliterator) {
		found := false
		for spltr.TryAdvance(func(t op.T) {
			s.pipeline(t, func(t op.T) {
				found = true
			})
		}) {
			if found {
				break
			}
		}
		rv_chan <- found
	}

	splits := split(s.spltr, CORES, find)

	for i := 0; i < splits; i++ {
		if <-rv_chan {
			return true
		}
	}

	return false
}

func (s *Stream) Count() (count int) {
	for s.spltr.TryAdvance(func(t op.T) {
		s.pipeline(t, func(t op.T) {})
	}) {
		count++
	}
	return
}

func (s *Stream) ForEach(c Consumer) {
	for s.spltr.TryAdvance(func(t op.T) {
		s.pipeline(t, c)
	}) {
	}
}

func (s *Stream) FindAny() *op.Optional {
	rv_chan := make(chan *op.Optional)

	find := func(spltr Spliterator) {
		found := false
		for spltr.TryAdvance(func(t op.T) {
			s.pipeline(t, func(t op.T) {
				rv_chan <- op.Of(t)
				found = true
			})
		}) {
			if found {
				break
			}
		}
		if !found {
			rv_chan <- op.Empty()
		}
	}

	splits := split(s.spltr, CORES, find)

	for i := 0; i < splits; i++ {
		o := <-rv_chan
		if o.IsPresent() {
			return o
		}
	}

	return op.Empty()
}

func split(spltr Spliterator, cores_left int, find func(Spliterator)) int {
	if cores_left > 1 {
		var rem int = cores_left / 2
		s := spltr.TrySplit()
		if s != nil {
			return split(s, rem, find) + split(spltr, rem, find)
		} else {
			go find(spltr)
			return 1
		}
	} else {
		go find(spltr)
		return 1
	}
}
