package matrix

import (
	"fmt"
	"math"
)

// ErrorCode identifica de forma estable el motivo por el que una matriz fue
// rechazada. El cliente puede ramificar sobre este código sin tener que parsear
// el mensaje, que está pensado para humanos y puede cambiar.
type ErrorCode string

const (
	// CodeEmptyMatrix: la matriz no tiene filas, o tiene filas sin columnas.
	CodeEmptyMatrix ErrorCode = "EMPTY_MATRIX"
	// CodeRaggedRows: las filas no tienen todas el mismo largo, por lo que la
	// entrada no representa una matriz rectangular.
	CodeRaggedRows ErrorCode = "RAGGED_ROWS"
	// CodeNonFiniteValue: la matriz contiene NaN o ±Inf. Ambos contaminarían
	// todo el resultado de la factorización, así que se rechazan en la entrada.
	CodeNonFiniteValue ErrorCode = "NON_FINITE_VALUE"
	// CodeMatrixTooLarge: la matriz supera el límite configurado de dimensión.
	CodeMatrixTooLarge ErrorCode = "MATRIX_TOO_LARGE"
)

// ValidationError describe una entrada inválida. Details lleva la información
// posicional necesaria para que el usuario corrija el problema sin adivinar.
type ValidationError struct {
	Code    ErrorCode      `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

func (e *ValidationError) Error() string { return e.Message }

// Validate comprueba que la entrada sea una matriz rectangular utilizable.
//
// maxDimension acota tanto filas como columnas. Es una defensa deliberada: el
// coste de la factorización crece como O(m·n²), de modo que sin un límite un
// único request podría monopolizar la CPU del servicio.
func Validate(m Matrix, maxDimension int) *ValidationError {
	rows := len(m)
	if rows == 0 {
		return &ValidationError{
			Code:    CodeEmptyMatrix,
			Message: "la matriz debe tener al menos una fila",
		}
	}

	cols := len(m[0])
	if cols == 0 {
		return &ValidationError{
			Code:    CodeEmptyMatrix,
			Message: "la matriz debe tener al menos una columna",
		}
	}

	if rows > maxDimension || cols > maxDimension {
		return &ValidationError{
			Code: CodeMatrixTooLarge,
			Message: fmt.Sprintf(
				"la matriz de %d×%d supera el límite de %d por dimensión",
				rows, cols, maxDimension,
			),
			Details: map[string]any{
				"rows": rows, "cols": cols, "maxDimension": maxDimension,
			},
		}
	}

	for i, row := range m {
		if len(row) != cols {
			return &ValidationError{
				Code: CodeRaggedRows,
				Message: fmt.Sprintf(
					"todas las filas deben tener el mismo largo: la fila 0 tiene %d columnas y la fila %d tiene %d",
					cols, i, len(row),
				),
				Details: map[string]any{
					"expectedCols": cols, "rowIndex": i, "actualCols": len(row),
				},
			}
		}
		for j, v := range row {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return &ValidationError{
					Code: CodeNonFiniteValue,
					Message: fmt.Sprintf(
						"la posición [%d][%d] contiene un valor no finito (NaN o infinito)",
						i, j,
					),
					Details: map[string]any{"rowIndex": i, "colIndex": j},
				}
			}
		}
	}

	return nil
}
