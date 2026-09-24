package calculator

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

const MaxExpressionLength = 256

func Calculate(a, b float64, op string) (float64, error) {
	if !finite(a) || !finite(b) {
		return 0, fmt.Errorf("numbers must be finite")
	}
	var result float64
	switch op {
	case "+":
		result = a + b
	case "-":
		result = a - b
	case "*":
		result = a * b
	case "/":
		if b == 0 {
			return 0, fmt.Errorf("division by zero")
		}
		result = a / b
	case "%":
		if b == 0 {
			return 0, fmt.Errorf("remainder by zero")
		}
		result = math.Mod(a, b)
	case "^":
		result = math.Pow(a, b)
	default:
		return 0, fmt.Errorf("unsupported operator %q", op)
	}
	if !finite(result) {
		return 0, fmt.Errorf("result is outside the supported numeric range")
	}
	return result, nil
}

// Evaluate parses arithmetic expressions without executing user-provided code.
// Trigonometric functions use radians.
func Evaluate(expression string) (float64, error) {
	if len(expression) > MaxExpressionLength {
		return 0, fmt.Errorf("expression cannot exceed %d bytes", MaxExpressionLength)
	}
	p := parser{input: expression}
	result, err := p.additive()
	if err != nil {
		return 0, err
	}
	p.space()
	if p.pos != len(p.input) {
		return 0, fmt.Errorf("unexpected character %q", p.input[p.pos])
	}
	if !finite(result) {
		return 0, fmt.Errorf("result is outside the supported numeric range")
	}
	return result, nil
}

type parser struct {
	input string
	pos   int
}

func (p *parser) additive() (float64, error) {
	left, err := p.multiplicative()
	if err != nil {
		return 0, err
	}
	for {
		op := byte(0)
		if p.take('+') {
			op = '+'
		} else if p.take('-') {
			op = '-'
		} else {
			return left, nil
		}
		right, err := p.multiplicative()
		if err != nil {
			return 0, err
		}
		left, err = Calculate(left, right, string(op))
		if err != nil {
			return 0, err
		}
	}
}

func (p *parser) multiplicative() (float64, error) {
	left, err := p.unary()
	if err != nil {
		return 0, err
	}
	for {
		op := byte(0)
		switch {
		case p.take('*'):
			op = '*'
		case p.take('/'):
			op = '/'
		case p.take('%'):
			op = '%'
		default:
			return left, nil
		}
		right, err := p.unary()
		if err != nil {
			return 0, err
		}
		left, err = Calculate(left, right, string(op))
		if err != nil {
			return 0, err
		}
	}
}

// Unary signs bind less tightly than powers: -2^2 is -(2^2), and 2^-2 works.
func (p *parser) unary() (float64, error) {
	if p.take('+') {
		return p.unary()
	}
	if p.take('-') {
		value, err := p.unary()
		return -value, err
	}
	return p.power()
}

func (p *parser) power() (float64, error) {
	left, err := p.primary()
	if err != nil {
		return 0, err
	}
	if p.take('^') {
		right, err := p.unary()
		if err != nil {
			return 0, err
		}
		return Calculate(left, right, "^")
	}
	return left, nil
}

func (p *parser) primary() (float64, error) {
	p.space()
	if p.pos >= len(p.input) {
		return 0, fmt.Errorf("expected a number or expression")
	}
	current := p.input[p.pos]
	if current == '(' {
		p.pos++
		value, err := p.additive()
		if err != nil {
			return 0, err
		}
		if !p.take(')') {
			return 0, fmt.Errorf("missing closing parenthesis")
		}
		return value, nil
	}
	if digit(current) || current == '.' {
		return p.number()
	}
	if letter(current) {
		start := p.pos
		for p.pos < len(p.input) && letter(p.input[p.pos]) {
			p.pos++
		}
		name := strings.ToLower(p.input[start:p.pos])
		if p.take('(') {
			value, err := p.additive()
			if err != nil {
				return 0, err
			}
			if !p.take(')') {
				return 0, fmt.Errorf("missing closing parenthesis after %s", name)
			}
			return applyFunction(name, value)
		}
		switch name {
		case "pi":
			return math.Pi, nil
		case "e":
			return math.E, nil
		default:
			return 0, fmt.Errorf("unknown constant or function %q", name)
		}
	}
	return 0, fmt.Errorf("unexpected character %q", current)
}

func (p *parser) number() (float64, error) {
	start, digits := p.pos, 0
	for p.pos < len(p.input) && isDigit(p.input[p.pos]) {
		p.pos++
		digits++
	}
	if p.pos < len(p.input) && p.input[p.pos] == '.' {
		p.pos++
		for p.pos < len(p.input) && isDigit(p.input[p.pos]) {
			p.pos++
			digits++
		}
	}
	if digits == 0 {
		return 0, fmt.Errorf("invalid number")
	}
	if p.pos < len(p.input) && (p.input[p.pos] == 'e' || p.input[p.pos] == 'E') {
		p.pos++
		if p.pos < len(p.input) && (p.input[p.pos] == '+' || p.input[p.pos] == '-') {
			p.pos++
		}
		exponentStart := p.pos
		for p.pos < len(p.input) && isDigit(p.input[p.pos]) {
			p.pos++
		}
		if p.pos == exponentStart {
			return 0, fmt.Errorf("invalid number exponent")
		}
	}
	value, err := strconv.ParseFloat(p.input[start:p.pos], 64)
	if err != nil || !finite(value) {
		return 0, fmt.Errorf("number is outside the supported numeric range")
	}
	return value, nil
}

func (p *parser) take(ch byte) bool {
	p.space()
	if p.pos < len(p.input) && p.input[p.pos] == ch {
		p.pos++
		return true
	}
	return false
}

func (p *parser) space() {
	for p.pos < len(p.input) {
		switch p.input[p.pos] {
		case ' ', '\t', '\n', '\r':
			p.pos++
		default:
			return
		}
	}
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func letter(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func applyFunction(name string, value float64) (float64, error) {
	var result float64
	switch name {
	case "sqrt":
		result = math.Sqrt(value)
	case "abs":
		result = math.Abs(value)
	case "sin":
		result = math.Sin(value)
	case "cos":
		result = math.Cos(value)
	case "tan":
		result = math.Tan(value)
	case "ln":
		if value <= 0 {
			return 0, fmt.Errorf("ln requires a value greater than zero")
		}
		result = math.Log(value)
	case "log":
		if value <= 0 {
			return 0, fmt.Errorf("log requires a value greater than zero")
		}
		result = math.Log10(value)
	case "floor":
		result = math.Floor(value)
	case "ceil":
		result = math.Ceil(value)
	default:
		return 0, fmt.Errorf("unknown function %q", name)
	}
	if !finite(result) {
		return 0, fmt.Errorf("%s is undefined for this value", name)
	}
	return result, nil
}
