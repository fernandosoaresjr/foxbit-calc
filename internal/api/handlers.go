package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/fernandosoaresjr/foxbit-calc/internal/calculator"
	"github.com/fernandosoaresjr/foxbit-calc/internal/service"
)

// maxPrecision limita o parâmetro precision (espelha o contrato OpenAPI).
const maxPrecision = 15

// Server implementa a ServerInterface gerada (api.gen.go), delegando o cálculo
// ao service. Este arquivo é escrito à mão e NÃO é tocado pela regeneração.
type Server struct {
	service *service.Service
}

// NewServer cria um Server a partir do service de cálculo.
func NewServer(svc *service.Service) *Server {
	return &Server{service: svc}
}

// Garante em tempo de compilação que Server satisfaz a interface gerada.
var _ ServerInterface = (*Server)(nil)

func (s *Server) Sum(ctx echo.Context, params SumParams) error {
	return s.handle(ctx, calculator.Sum, params.TermOne, params.TermTwo, params.Precision)
}

func (s *Server) Subtract(ctx echo.Context, params SubtractParams) error {
	return s.handle(ctx, calculator.Sub, params.TermOne, params.TermTwo, params.Precision)
}

func (s *Server) Multiply(ctx echo.Context, params MultiplyParams) error {
	return s.handle(ctx, calculator.Mul, params.TermOne, params.TermTwo, params.Precision)
}

func (s *Server) Divide(ctx echo.Context, params DivideParams) error {
	return s.handle(ctx, calculator.Div, params.TermOne, params.TermTwo, params.Precision)
}

// handle centraliza a lógica comum aos quatro endpoints: validação de
// precision, execução via service e mapeamento de erros para HTTP.
func (s *Server) handle(ctx echo.Context, op calculator.Operation, termOne, termTwo float64, precision *int) error {
	if precision != nil && (*precision < 0 || *precision > maxPrecision) {
		return ctx.JSON(http.StatusBadRequest, Error{
			Message: "precision must be between 0 and 15",
		})
	}

	result, err := s.service.Compute(ctx.Request().Context(), service.Request{
		Operation: op,
		TermOne:   termOne,
		TermTwo:   termTwo,
		Precision: precision,
	})
	if err != nil {
		switch {
		case errors.Is(err, calculator.ErrDivisionByZero):
			return ctx.JSON(http.StatusBadRequest, Error{Message: "term_two must not be zero"})
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			// Cliente desistiu ou tempo esgotou: 503.
			return ctx.JSON(http.StatusServiceUnavailable, Error{Message: "request canceled"})
		default:
			return ctx.JSON(http.StatusInternalServerError, Error{Message: "internal error"})
		}
	}

	return ctx.JSON(http.StatusOK, CalculationResult{Result: result})
}
