package matrix

import (
	"math"
	"testing"
)

func TestValidate(t *testing.T) {
	const maxDim = 256

	cases := []struct {
		name     string
		input    Matrix
		maxDim   int
		wantCode ErrorCode // vacío significa que se espera que la matriz sea válida
	}{
		{
			name:   "matriz cuadrada válida",
			input:  Matrix{{1, 2}, {3, 4}},
			maxDim: maxDim,
		},
		{
			name:   "matriz rectangular válida",
			input:  Matrix{{1, 2, 3}, {4, 5, 6}},
			maxDim: maxDim,
		},
		{
			name:   "matriz de un solo elemento",
			input:  Matrix{{42}},
			maxDim: maxDim,
		},
		{
			name:     "sin filas",
			input:    Matrix{},
			maxDim:   maxDim,
			wantCode: CodeEmptyMatrix,
		},
		{
			name:     "fila sin columnas",
			input:    Matrix{{}},
			maxDim:   maxDim,
			wantCode: CodeEmptyMatrix,
		},
		{
			name:     "filas de distinto largo",
			input:    Matrix{{1, 2, 3}, {4, 5}},
			maxDim:   maxDim,
			wantCode: CodeRaggedRows,
		},
		{
			name:     "fila vacía intercalada",
			input:    Matrix{{1, 2}, {}},
			maxDim:   maxDim,
			wantCode: CodeRaggedRows,
		},
		{
			name:     "contiene NaN",
			input:    Matrix{{1, 2}, {3, math.NaN()}},
			maxDim:   maxDim,
			wantCode: CodeNonFiniteValue,
		},
		{
			name:     "contiene infinito positivo",
			input:    Matrix{{math.Inf(1), 2}},
			maxDim:   maxDim,
			wantCode: CodeNonFiniteValue,
		},
		{
			name:     "contiene infinito negativo",
			input:    Matrix{{1, math.Inf(-1)}},
			maxDim:   maxDim,
			wantCode: CodeNonFiniteValue,
		},
		{
			name:     "demasiadas filas",
			input:    New(5, 2),
			maxDim:   4,
			wantCode: CodeMatrixTooLarge,
		},
		{
			name:     "demasiadas columnas",
			input:    New(2, 5),
			maxDim:   4,
			wantCode: CodeMatrixTooLarge,
		},
		{
			name:   "justo en el límite",
			input:  New(4, 4),
			maxDim: 4,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.input, tc.maxDim)

			if tc.wantCode == "" {
				if err != nil {
					t.Fatalf("se esperaba una matriz válida, se obtuvo %s: %s", err.Code, err.Message)
				}
				return
			}

			if err == nil {
				t.Fatalf("se esperaba el error %s, la matriz fue aceptada", tc.wantCode)
			}
			if err.Code != tc.wantCode {
				t.Errorf("código = %s, se esperaba %s (mensaje: %s)", err.Code, tc.wantCode, err.Message)
			}
			if err.Message == "" {
				t.Error("el error no trae mensaje legible")
			}
		})
	}
}

// TestValidateErrorDetails comprueba que el error posicional sea accionable:
// debe indicar qué fila rompe el rectángulo, no solo que algo falló.
func TestValidateErrorDetails(t *testing.T) {
	err := Validate(Matrix{{1, 2, 3}, {4, 5, 6}, {7, 8}}, 256)
	if err == nil {
		t.Fatal("se esperaba el error RAGGED_ROWS")
	}
	if got := err.Details["rowIndex"]; got != 2 {
		t.Errorf("rowIndex = %v, se esperaba 2", got)
	}
	if got := err.Details["expectedCols"]; got != 3 {
		t.Errorf("expectedCols = %v, se esperaba 3", got)
	}
	if got := err.Details["actualCols"]; got != 2 {
		t.Errorf("actualCols = %v, se esperaba 2", got)
	}
}
