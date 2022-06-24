package main

import (
	"testing"
)

func TestDivideBy100(t *testing.T) {
	cases := []struct {
		input    float64
		expected float64
	}{
		{
			15.11,
			.1511,
		},
		{
			0.1511,
			0.001511,
		},
		{
			0.1,
			0.001,
		},
		{
			0.01,
			0.0001,
		},
		{
			0.0003432,
			0.000003432,
		},
		{
			0.000003432,
			0.00000003432,
		},
		{
			129392.493093,
			1293.92493093,
		},
		{
			999.9,
			9.999,
		},
		{
			9999999999.99,
			99999999.9999,
		},
		{
			99999999999,
			999999999.99,
		},
	}

	for _, v := range cases {
		res := DivideBy100(v.input)
		if res != v.expected {
			t.Logf("Failed got:%v expected:%v", res, v.expected)
		}
	}
}
func BenchmarkDivideBy100(b *testing.B) {
	// run the Fib function b.N times
	for n := 0; n < b.N; n++ {
		DivideBy100(15.11)
	}
}

func BenchmarkDivide(b *testing.B) {
	// run the Fib function b.N times
	for n := 0; n < b.N; n++ {
		_ = 15.11 / 100
	}
}
