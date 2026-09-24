package calculator

import (
	"math"
	"strings"
	"testing"
)

func TestEvaluate(t *testing.T) {
	tests := []struct {
		expression string
		want       float64
	}{
		{"2 + 3 * 4", 14},
		{"(2 + 3) * 4", 20},
		{"-2^2", -4},
		{"2^3^2", 512},
		{"2^-2", 0.25},
		{"sqrt(81) + abs(-2)", 11},
		{"sin(pi / 2)", 1},
		{"cos(0) + tan(0)", 1},
		{"ln(e)", 1},
		{"log(1000)", 3},
		{"floor(3.9) + ceil(1.1)", 6},
		{"9 % 4", 1},
	}
	for _, test := range tests {
		t.Run(test.expression, func(t *testing.T) {
			got, err := Evaluate(test.expression)
			if err != nil {
				t.Fatal(err)
			}
			if math.Abs(got-test.want) > 1e-10*(1+math.Abs(test.want)) {
				t.Fatalf("got %v, want %v", got, test.want)
			}
		})
	}
}

func TestEvaluateRejectsInvalidExpressions(t *testing.T) {
	tests := []string{
		"", "1 / 0", "5 % 0", "sqrt(-1)", "ln(0)", "2 +", "(2 + 3",
		"2 & 3", "unknown(2)", strings.Repeat("1", MaxExpressionLength+1), "10^10000",
	}
	for _, expression := range tests {
		t.Run(expression, func(t *testing.T) {
			if _, err := Evaluate(expression); err == nil {
				t.Fatalf("Evaluate(%q) unexpectedly succeeded", expression)
			}
		})
	}
}

func TestCalculateSupportsPowerAndRemainder(t *testing.T) {
	if got, err := Calculate(2, 8, "^"); err != nil || got != 256 {
		t.Fatalf("power = %v, %v; want 256", got, err)
	}
	if got, err := Calculate(10, 4, "%"); err != nil || got != 2 {
		t.Fatalf("remainder = %v, %v; want 2", got, err)
	}
	if _, err := Calculate(1e308, 1e308, "*"); err == nil {
		t.Fatal("overflow unexpectedly succeeded")
	}
}
