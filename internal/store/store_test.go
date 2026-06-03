package store

import "testing"

func TestVecLiteral(t *testing.T) {
	cases := []struct {
		in   []float32
		want string
	}{
		{nil, "[]"},
		{[]float32{}, "[]"},
		{[]float32{1}, "[1]"},
		{[]float32{1, 2.5, -0.3}, "[1,2.5,-0.3]"},
	}
	for _, c := range cases {
		if got := vecLiteral(c.in); got != c.want {
			t.Errorf("vecLiteral(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}
