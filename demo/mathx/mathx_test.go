package mathx

import "testing"

func TestFib(t *testing.T) {
	cases := map[int]int{0: 0, 1: 1, 2: 1, 5: 5, 10: 55}
	for n, want := range cases {
		if got := Fib(n); got != want {
			t.Errorf("Fib(%d) = %d, want %d", n, got, want)
		}
	}
}
