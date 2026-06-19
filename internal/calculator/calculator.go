// Package calculator implementa as quatro operações básicas da matemática como
// funções puras, além de um utilitário de truncamento de precisão. Não tem
// dependências de framework, cache ou I/O — é o núcleo testável da aplicação.
package calculator

import (
	"errors"
	"fmt"
	"math"
)

// ErrDivisionByZero é retornado por Div (e Calculate) quando o divisor é zero.
var ErrDivisionByZero = errors.New("division by zero")

// Operation identifica uma das quatro operações suportadas. O valor textual
// coincide com o sufixo de rota (/api/<op>) e é usado em chaves de cache,
// métricas e logs.
type Operation string

const (
	Sum Operation = "sum"
	Sub Operation = "sub"
	Mul Operation = "mul"
	Div Operation = "div"
)

// Add retorna a + b.
func Add(a, b float64) float64 { return a + b }

// Subtract retorna a - b.
func Subtract(a, b float64) float64 { return a - b }

// Multiply retorna a * b.
func Multiply(a, b float64) float64 { return a * b }

// Divide retorna a / b, ou ErrDivisionByZero quando b == 0.
func Divide(a, b float64) (float64, error) {
	if b == 0 {
		return 0, ErrDivisionByZero
	}
	return a / b, nil
}

// Calculate despacha a operação op sobre os termos a e b. Retorna erro para
// operação desconhecida ou divisão por zero.
func Calculate(op Operation, a, b float64) (float64, error) {
	switch op {
	case Sum:
		return Add(a, b), nil
	case Sub:
		return Subtract(a, b), nil
	case Mul:
		return Multiply(a, b), nil
	case Div:
		return Divide(a, b)
	default:
		return 0, fmt.Errorf("unknown operation: %q", op)
	}
}

// Truncate trunca value para precision casas decimais, sem arredondar e em
// direção a zero. Com precision 0, trunca para inteiro. Valores negativos de
// precision são tratados como 0.
func Truncate(value float64, precision int) float64 {
	if precision < 0 {
		precision = 0
	}
	factor := math.Pow(10, float64(precision))
	return math.Trunc(value*factor) / factor
}
