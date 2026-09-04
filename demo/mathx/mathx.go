// Package mathx is a tiny testable package for the in-tab `go test` demo: the
// toolchain compiles it AND its test, runs the test binary, and reports ok —
// something the Playground (which compiles a single main.go server-side and
// only runs it) does not offer.
package mathx

// Fib returns the nth Fibonacci number (Fib(0)=0, Fib(1)=1).
func Fib(n int) int {
	a, b := 0, 1
	for i := 0; i < n; i++ {
		a, b = b, a+b
	}
	return a
}
