package calculator

import (
	"errors"
	"testing"
)

func TestArithmeticOperations(t *testing.T) {
	tests := []struct {
		name string
		fn   func(a, b float64) float64
		a, b float64
		want float64
	}{
		{"add positives", Add, 4, 1, 5},
		{"add negatives", Add, -4, -1, -5},
		{"add float", Add, 1.5, 2.25, 3.75},
		{"subtract", Subtract, 4, 1, 3},
		{"subtract negative result", Subtract, 1, 4, -3},
		{"multiply", Multiply, 4, 3, 12},
		{"multiply by zero", Multiply, 4, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.fn(tt.a, tt.b); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDivide(t *testing.T) {
	got, err := Divide(10, 4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 2.5 {
		t.Errorf("got %v, want 2.5", got)
	}
}

func TestDivideByZero(t *testing.T) {
	_, err := Divide(1, 0)
	if !errors.Is(err, ErrDivisionByZero) {
		t.Errorf("got %v, want ErrDivisionByZero", err)
	}
}

func TestCalculate(t *testing.T) {
	tests := []struct {
		op      Operation
		a, b    float64
		want    float64
		wantErr bool
	}{
		{Sum, 4, 1, 5, false},
		{Sub, 4, 1, 3, false},
		{Mul, 4, 3, 12, false},
		{Div, 12, 4, 3, false},
		{Div, 1, 0, 0, true},
		{Operation("pow"), 2, 3, 0, true},
	}
	for _, tt := range tests {
		t.Run(string(tt.op), func(t *testing.T) {
			got, err := Calculate(tt.op, tt.a, tt.b)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name      string
		value     float64
		precision int
		want      float64
	}{
		{"to integer truncates down", 1.6666, 0, 1},
		{"to integer no rounding", 1.9999, 0, 1},
		{"two decimals", 1.3399, 2, 1.33},
		{"two decimals no rounding", 1.6666, 2, 1.66},
		{"negative truncates toward zero", -1.6666, 0, -1},
		{"negative two decimals", -1.6666, 2, -1.66},
		{"already exact", 5, 2, 5},
		{"negative precision treated as zero", 1.99, -3, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Truncate(tt.value, tt.precision); got != tt.want {
				t.Errorf("Truncate(%v, %d) = %v, want %v", tt.value, tt.precision, got, tt.want)
			}
		})
	}
}
