package matrix

import (
	"math"
	"testing"
)

// tolerance usada al comparar resultados numéricos. Es varios órdenes de
// magnitud mayor que el épsilon de máquina (2.2e-16) para absorber el error
// acumulado de las operaciones, pero lo bastante estricta como para detectar
// un algoritmo realmente incorrecto.
const tolerance = 1e-10

// assertDimensions falla si la matriz no tiene el tamaño esperado.
func assertDimensions(t *testing.T, name string, m Matrix, rows, cols int) {
	t.Helper()
	if m.Rows() != rows || m.Cols() != cols {
		t.Errorf("%s: se esperaba %d×%d, se obtuvo %d×%d", name, rows, cols, m.Rows(), m.Cols())
	}
}

// assertOrthonormalColumns verifica que QᵀQ = I, es decir, que las columnas de
// Q sean ortogonales entre sí y de norma 1. Es la propiedad que define a Q y la
// que se degrada primero cuando el algoritmo es numéricamente inestable.
func assertOrthonormalColumns(t *testing.T, q Matrix) {
	t.Helper()
	gram := q.Transpose().Mul(q)
	for i := range gram {
		for j := range gram[i] {
			expected := 0.0
			if i == j {
				expected = 1.0
			}
			if math.Abs(gram[i][j]-expected) > tolerance {
				t.Errorf("QᵀQ[%d][%d] = %g, se esperaba %g: Q no es ortogonal", i, j, gram[i][j], expected)
			}
		}
	}
}

// assertReconstructs verifica que Q·R devuelva la matriz original.
func assertReconstructs(t *testing.T, a, q, r Matrix) {
	t.Helper()
	product := q.Mul(r)
	if product == nil {
		t.Fatalf("Q(%d×%d)·R(%d×%d): dimensiones incompatibles", q.Rows(), q.Cols(), r.Rows(), r.Cols())
	}
	assertDimensions(t, "Q·R", product, a.Rows(), a.Cols())

	// La comparación es relativa a la escala de la matriz: un error absoluto de
	// 1e-6 es irrelevante en una matriz de valores ~1e150 y catastrófico en una
	// de valores ~1e-150.
	scale := math.Max(a.MaxAbs(), 1)
	for i := range a {
		for j := range a[i] {
			if math.Abs(product[i][j]-a[i][j]) > tolerance*scale {
				t.Errorf("Q·R[%d][%d] = %g, se esperaba %g", i, j, product[i][j], a[i][j])
			}
		}
	}
}
