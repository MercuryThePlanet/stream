package stream

import (
	op "github.com/MercuryThePlanet/optional"
	"runtime"
)

var (
	CORES = runtime.NumCPU()
)

type Spliterator interface {
	TryAdvance(Consumer) bool
	ForEachRemaining(Consumer)
	TrySplit() Spliterator
}

type (
	BinaryOp  func(op.T, op.T) op.T
	UnaryOp   func(op.T) op.T
	Supplier  func() op.T
	Consumer  func(op.T)
	Predicate func(op.T) bool
)

type Stream struct {
	pipeline     func(op.T, Consumer)
	spltr        Spliterator
	limitReached bool
}

type generator struct {
	supplier Supplier
}

func (g *generator) TryAdvance(c Consumer) bool {
	c(g.supplier())
	return true
}

func (g *generator) ForEachRemaining(c Consumer) {
	for {
		c(g.supplier())
	}
}

func (g *generator) TrySplit() Spliterator {
	return nil
}

func Of(s Spliterator) *Stream {
	return &Stream{func(t op.T, c Consumer) { c(t) }, s, false}
}

func Generate(supplier Supplier) *Stream {
	return Of(&generator{supplier})
}

type Collector interface {
	Collect(Spliterator)
}

type GeneralCollector struct {
	ts op.Ts
}

func NewGeneralCollector() *GeneralCollector {
	return &GeneralCollector{}
}

func (c *GeneralCollector) Collect(spltr Spliterator) {
	for spltr.TryAdvance(func(t op.T) {
		c.ts = append(c.ts, t)
	}) {
	}
}

func (c *GeneralCollector) Get() op.Ts {
	return c.ts
}

func (s *Stream) Collect(collector Collector) {
	collector.Collect(s.spltr)
}

func Iterate(seed op.T, m UnaryOp) *Stream {
	supplier := func() op.T {
		defer func() {
			seed = m(seed)
		}()
		return seed
	}
	return Of(&generator{supplier})
}

func (s *Stream) Peek(peek_consumer Consumer) *Stream {
	cur := s.pipeline
	s.pipeline = func(t op.T, c Consumer) {
		cur(t, func(t op.T) {
			peek_consumer(t)
			c(t)
		})
	}
	return s

}

func (s *Stream) Skip(num int) *Stream {
	cur := s.pipeline
	skipped := 0
	s.pipeline = func(t op.T, c Consumer) {
		cur(t, func(t op.T) {
			if skipped >= num {
				c(t)
			} else {
				skipped += 1
			}
		})
	}
	return s
}

func (s *Stream) Limit(size int) *Stream {
	cur := s.pipeline
	limit := 0
	s.pipeline = func(t op.T, c Consumer) {
		cur(t, func(t op.T) {
			c(t)
			limit += 1
			s.limitReached = limit >= size
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

func (s *Stream) Map(m UnaryOp) *Stream {
	cur := s.pipeline
	s.pipeline = func(t op.T, c Consumer) {
		cur(t, func(t op.T) {
			c(m(t))
		})
	}
	return s
}

func (s *Stream) NoneMatch(p Predicate) bool {
	return !s.AnyMatch(p)
}

func (s *Stream) AllMatch(p Predicate) (matches bool) {
	matches = true
	for s.spltr.TryAdvance(func(t op.T) {
		s.pipeline(t, func(t op.T) {
			matches = p(t)
		})
	}) && !s.limitReached {
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
		}) && !s.limitReached {
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
	}) && !s.limitReached {
		count++
	}
	return
}

func (s *Stream) ToSlice() (array op.Ts) {
	for s.spltr.TryAdvance(func(t op.T) {
		s.pipeline(t, func(t op.T) {
			array = append(array, t)
		})
	}) && !s.limitReached {
	}
	return
}

func (s *Stream) ForEach(c Consumer) {
	for s.spltr.TryAdvance(func(t op.T) {
		s.pipeline(t, c)
	}) && !s.limitReached {
	}
}

func (s *Stream) Reduce(reducer BinaryOp) *op.Optional {
	var rv op.T
	set := false
	for s.spltr.TryAdvance(func(t op.T) {
		s.pipeline(t, func(inner_t op.T) {
			rv = inner_t
			set = true
		})

	}) && !s.limitReached && !set {
	}
	for s.spltr.TryAdvance(func(t op.T) {
		s.pipeline(t, func(inner_t op.T) {
			rv = reducer(rv, inner_t)
		})
	}) && !s.limitReached {
	}

	return op.OfNilable(rv)
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
		}) && !s.limitReached {
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
