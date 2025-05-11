package util

type Tuple2[A, B any] struct {
	A A
	B B
}

func (t *Tuple2[A, B]) Values() (A, B) {
	return t.A, t.B
}

func NewTuple2[A, B any](a A, b B) *Tuple2[A, B] {
	return &Tuple2[A, B]{a, b}
}

type Tuple3[A, B, C any] struct {
	A A
	B B
	C C
}

func (t *Tuple3[A, B, C]) Values() (A, B, C) {
	return t.A, t.B, t.C
}

func NewTuple3[A, B, C any](a A, b B, c C) *Tuple3[A, B, C] {
	return &Tuple3[A, B, C]{a, b, c}
}

type Tuple4[A, B, C, D any] struct {
	A A
	B B
	C C
	D D
}

func (t *Tuple4[A, B, C, D]) Values() (A, B, C, D) {
	return t.A, t.B, t.C, t.D
}

func NewTuple4[A, B, C, D any](a A, b B, c C, d D) *Tuple4[A, B, C, D] {
	return &Tuple4[A, B, C, D]{a, b, c, d}
}

type Tuple5[A, B, C, D, E any] struct {
	A A
	B B
	C C
	D D
	E E
}

func (t *Tuple5[A, B, C, D, E]) Values() (A, B, C, D, E) {
	return t.A, t.B, t.C, t.D, t.E
}

func NewTuple5[A, B, C, D, E any](a A, b B, c C, d D, e E) *Tuple5[A, B, C, D, E] {
	return &Tuple5[A, B, C, D, E]{a, b, c, d, e}
}
