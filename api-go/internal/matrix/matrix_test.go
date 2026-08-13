package matrix

import (
	"math"
	"testing"
)

func TestIdentity(t *testing.T) {
	id := Identity(3)
	assertDimensions(t, "Identity(3)", id, 3, 3)

	for i := range id {
		for j := range id[i] {
			want := 0.0
			if i == j {
				want = 1.0
			}
			if id[i][j] != want {
				t.Errorf("Identity[%d][%d] = %g, se esperaba %g", i, j, id[i][j], want)
			}
		}
	}
}

func TestTranspose(t *testing.T) {
	a := Matrix{{1, 2, 3}, {4, 5, 6}}
	got := a.Transpose()

	assertDimensions(t, "Aᵀ", got, 3, 2)

	want := Matrix{{1, 4}, {2, 5}, {3, 6}}
	for i := range want {
		for j := range want[i] {
			if got[i][j] != want[i][j] {
				t.Errorf("Aᵀ[%d][%d] = %g, se esperaba %g", i, j, got[i][j], want[i][j])
			}
		}
	}
}

func TestMul(t *testing.T) {
	cases := []struct {
		name string
		a, b Matrix
		want Matrix // nil indica que se esperan dimensiones incompatibles
	}{
		{
			name: "2×3 por 3×2",
			a:    Matrix{{1, 2, 3}, {4, 5, 6}},
			b:    Matrix{{7, 8}, {9, 10}, {11, 12}},
			want: Matrix{{58, 64}, {139, 154}},
		},
		{
			name: "por la identidad",
			a:    Matrix{{1, 2}, {3, 4}},
			b:    Identity(2),
			want: Matrix{{1, 2}, {3, 4}},
		},
		{
			name: "dimensiones incompatibles",
			a:    Matrix{{1, 2}},
			b:    Matrix{{1, 2}},
			want: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.a.Mul(tc.b)

			if tc.want == nil {
				if got != nil {
					t.Fatalf("se esperaba nil por dimensiones incompatibles, se obtuvo %v", got)
				}
				return
			}

			assertDimensions(t, "A·B", got, tc.want.Rows(), tc.want.Cols())
			for i := range tc.want {
				for j := range tc.want[i] {
					if got[i][j] != tc.want[i][j] {
						t.Errorf("A·B[%d][%d] = %g, se esperaba %g", i, j, got[i][j], tc.want[i][j])
					}
				}
			}
		})
	}
}

func TestCloneIsDeep(t *testing.T) {
	a := Matrix{{1, 2}, {3, 4}}
	c := a.Clone()
	c[0][0] = 99

	if a[0][0] != 1 {
		t.Errorf("la copia comparte memoria con el original: A[0][0] = %g", a[0][0])
	}
}

func TestMaxAbs(t *testing.T) {
	cases := []struct {
		name  string
		input Matrix
		want  float64
	}{
		{"valor negativo dominante", Matrix{{1, -9}, {3, 4}}, 9},
		{"matriz nula", New(2, 2), 0},
		{"valor positivo dominante", Matrix{{-1, 2}, {8, -4}}, 8},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.input.MaxAbs(); got != tc.want {
				t.Errorf("MaxAbs = %g, se esperaba %g", got, tc.want)
			}
		})
	}
}

func TestIsUpperTriangular(t *testing.T) {
	cases := []struct {
		name  string
		input Matrix
		tol   float64
		want  bool
	}{
		{"triangular exacta", Matrix{{1, 2}, {0, 3}}, 0, true},
		{"no triangular", Matrix{{1, 2}, {5, 3}}, 0, false},
		{"residuo dentro de la tolerancia", Matrix{{1, 2}, {1e-18, 3}}, 1e-12, true},
		{"residuo fuera de la tolerancia", Matrix{{1, 2}, {1e-18, 3}}, 0, false},
		{"rectangular ancha", Matrix{{1, 2, 3}, {0, 4, 5}}, 0, true},
		{"rectangular alta", Matrix{{1}, {0}, {0}}, 0, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.input.IsUpperTriangular(tc.tol); got != tc.want {
				t.Errorf("IsUpperTriangular = %v, se esperaba %v", got, tc.want)
			}
		})
	}
}

func TestScrubNegativeZero(t *testing.T) {
	m := Matrix{{math.Copysign(0, -1), 1}}
	if !math.Signbit(m[0][0]) {
		t.Fatal("el caso de prueba no contiene un cero negativo")
	}

	m.scrubNegativeZero()

	if math.Signbit(m[0][0]) {
		t.Error("el cero negativo no fue normalizado")
	}
	if m[0][1] != 1 {
		t.Error("se alteró un valor distinto de cero")
	}
}
