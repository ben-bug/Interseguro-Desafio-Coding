package matrix

import (
	"math"
	"testing"
)

// TestQRFullProperties verifica las tres propiedades que definen la
// factorización, en lugar de comparar contra valores precalculados.
//
// Esto es deliberado: A = Q·R no es única (invertir el signo de una columna de
// Q y de la fila correspondiente de R produce otra factorización igualmente
// válida), así que fijar valores esperados ataría el test a un detalle de
// implementación en vez de al contrato matemático.
func TestQRFullProperties(t *testing.T) {
	cases := []struct {
		name string
		a    Matrix
	}{
		{
			name: "cuadrada bien condicionada",
			a:    Matrix{{12, -51, 4}, {6, 167, -68}, {-4, 24, -41}},
		},
		{
			name: "más filas que columnas (m > n)",
			a:    Matrix{{1, 2}, {3, 4}, {5, 6}, {7, 8}},
		},
		{
			name: "más columnas que filas (m < n)",
			a:    Matrix{{1, 2, 3, 4}, {5, 6, 7, 8}},
		},
		{
			name: "vector columna",
			a:    Matrix{{3}, {4}, {12}},
		},
		{
			name: "vector fila",
			a:    Matrix{{3, 4, 12}},
		},
		{
			name: "escalar 1×1",
			a:    Matrix{{7}},
		},
		{
			name: "escalar 1×1 negativo",
			a:    Matrix{{-7}},
		},
		{
			name: "identidad",
			a:    Identity(4),
		},
		{
			// Rango 1: cada fila es múltiplo de la primera. Gram-Schmidt
			// clásico se degrada notoriamente aquí; Householder no.
			name: "columnas linealmente dependientes",
			a:    Matrix{{1, 2, 3}, {2, 4, 6}, {3, 6, 9}},
		},
		{
			name: "matriz nula",
			a:    New(3, 3),
		},
		{
			// Primera columna ya alineada con e₁: ejercita la rama en que la
			// subcolumna no requiere reflexión.
			name: "primera columna canónica",
			a:    Matrix{{5, 1, 2}, {0, 3, 4}, {0, 0, 6}},
		},
		{
			// Verifica el escalado de norm2: elevar 1e150 al cuadrado
			// desbordaría a +Inf sin él.
			name: "valores muy grandes",
			a:    Matrix{{1e150, 2e150}, {3e150, 4e150}},
		},
		{
			// El extremo opuesto: sin escalado, el cuadrado se anula por
			// underflow y la norma daría 0.
			name: "valores muy pequeños",
			a:    Matrix{{1e-150, 2e-150}, {3e-150, 4e-150}},
		},
		{
			name: "valores mixtos con ceros intercalados",
			a:    Matrix{{0, 1, 0}, {2, 0, 3}, {0, 4, 0}, {5, 0, 0}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, n := tc.a.Rows(), tc.a.Cols()
			d := QR(tc.a, ModeFull)

			assertDimensions(t, "Q", d.Q, m, m)
			assertDimensions(t, "R", d.R, m, n)

			assertReconstructs(t, tc.a, d.Q, d.R)
			assertOrthonormalColumns(t, d.Q)

			if !d.R.IsUpperTriangular(0) {
				t.Errorf("R no es triangular superior de forma exacta:\n%v", d.R)
			}
			if d.Mode != ModeFull {
				t.Errorf("Mode = %q, se esperaba %q", d.Mode, ModeFull)
			}
			if d.Residual > tolerance {
				t.Errorf("residual = %g, supera la tolerancia %g", d.Residual, tolerance)
			}
		})
	}
}

// TestQRDoesNotMutateInput comprueba que la matriz recibida quede intacta. El
// handler HTTP la reutiliza para calcular el residual y para la respuesta, así
// que una mutación silenciosa corrompería ambos.
func TestQRDoesNotMutateInput(t *testing.T) {
	a := Matrix{{12, -51, 4}, {6, 167, -68}, {-4, 24, -41}}
	original := a.Clone()

	QR(a, ModeFull)

	for i := range a {
		for j := range a[i] {
			if a[i][j] != original[i][j] {
				t.Fatalf("la entrada fue mutada en [%d][%d]: %g → %g", i, j, original[i][j], a[i][j])
			}
		}
	}
}

// TestQRReducedDimensions verifica el recorte de la variante reducida y que
// esta siga reconstruyendo la matriz original.
func TestQRReducedDimensions(t *testing.T) {
	cases := []struct {
		name                 string
		a                    Matrix
		wantQRows, wantQCols int
		wantRRows, wantRCols int
	}{
		{
			name: "m > n recorta Q y R",
			a:    Matrix{{1, 2}, {3, 4}, {5, 6}, {7, 8}},
			// Q pasa de 4×4 a 4×2 y R de 4×2 a 2×2.
			wantQRows: 4, wantQCols: 2, wantRRows: 2, wantRCols: 2,
		},
		{
			name: "m = n coincide con la variante completa",
			a:    Matrix{{12, -51, 4}, {6, 167, -68}, {-4, 24, -41}},
			wantQRows: 3, wantQCols: 3, wantRRows: 3, wantRCols: 3,
		},
		{
			name: "m < n coincide con la variante completa",
			a:    Matrix{{1, 2, 3, 4}, {5, 6, 7, 8}},
			wantQRows: 2, wantQCols: 2, wantRRows: 2, wantRCols: 4,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := QR(tc.a, ModeReduced)

			assertDimensions(t, "Q", d.Q, tc.wantQRows, tc.wantQCols)
			assertDimensions(t, "R", d.R, tc.wantRRows, tc.wantRCols)

			assertReconstructs(t, tc.a, d.Q, d.R)
			assertOrthonormalColumns(t, d.Q)

			if d.Mode != ModeReduced {
				t.Errorf("Mode = %q, se esperaba %q", d.Mode, ModeReduced)
			}
		})
	}
}

// TestQRKnownMagnitudes contrasta contra el ejemplo canónico de la literatura.
// Los signos dependen de la convención del algoritmo, así que se comparan
// magnitudes: son invariantes entre implementaciones.
func TestQRKnownMagnitudes(t *testing.T) {
	a := Matrix{{12, -51, 4}, {6, 167, -68}, {-4, 24, -41}}
	want := []float64{14, 175, 35} // diagonal de R en valor absoluto

	d := QR(a, ModeFull)

	for i, w := range want {
		if got := math.Abs(d.R[i][i]); math.Abs(got-w) > 1e-9 {
			t.Errorf("|R[%d][%d]| = %g, se esperaba %g", i, i, got, w)
		}
	}
}

// TestQRIllConditioned usa una matriz de Hilbert, cuyo número de condición
// crece de forma explosiva con el tamaño. Es el escenario donde Gram-Schmidt
// clásico pierde ortogonalidad y donde Householder debe sostenerla.
func TestQRIllConditioned(t *testing.T) {
	const n = 8
	hilbert := New(n, n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			hilbert[i][j] = 1 / float64(i+j+1)
		}
	}

	d := QR(hilbert, ModeFull)

	assertOrthonormalColumns(t, d.Q)
	assertReconstructs(t, hilbert, d.Q, d.R)
}

func TestNorm2(t *testing.T) {
	cases := []struct {
		name  string
		input []float64
		want  float64
	}{
		{"terna pitagórica", []float64{3, 4}, 5},
		{"vector nulo", []float64{0, 0, 0}, 0},
		{"slice vacío", []float64{}, 0},
		{"valor único negativo", []float64{-7}, 7},
		{"sin desbordar", []float64{3e200, 4e200}, 5e200},
		{"sin anularse por underflow", []float64{3e-200, 4e-200}, 5e-200},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := norm2(tc.input)
			if tc.want == 0 {
				if got != 0 {
					t.Errorf("norm2 = %g, se esperaba 0", got)
				}
				return
			}
			if relErr := math.Abs(got-tc.want) / tc.want; relErr > 1e-15 {
				t.Errorf("norm2 = %g, se esperaba %g (error relativo %g)", got, tc.want, relErr)
			}
		})
	}
}

func TestResidualOnNullMatrix(t *testing.T) {
	// Con A nula el error relativo es 0/0; la función debe devolver 0 y no NaN.
	a := New(2, 2)
	d := QR(a, ModeFull)
	if d.Residual != 0 {
		t.Errorf("residual = %g, se esperaba 0 para la matriz nula", d.Residual)
	}
}

// BenchmarkQR mide el caso denso más grande que admite la API por defecto.
func BenchmarkQR(b *testing.B) {
	const size = 128
	a := New(size, size)
	seed := uint64(42)
	for i := 0; i < size; i++ {
		for j := 0; j < size; j++ {
			// Generador congruencial lineal: reproducible y sin dependencias.
			seed = seed*6364136223846793005 + 1442695040888963407
			a[i][j] = float64(int64(seed>>33)) / 1e9
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		QR(a, ModeFull)
	}
}
