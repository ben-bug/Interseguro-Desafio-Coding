package matrix

import "testing"

func TestRotate90(t *testing.T) {
	cases := []struct {
		name  string
		input Matrix
		want  Matrix
	}{
		{
			name:  "cuadrada",
			input: Matrix{{1, 2}, {3, 4}},
			want:  Matrix{{3, 1}, {4, 2}},
		},
		{
			name:  "rectangular ancha se vuelve alta",
			input: Matrix{{1, 2, 3}, {4, 5, 6}},
			want:  Matrix{{4, 1}, {5, 2}, {6, 3}},
		},
		{
			name:  "vector fila se vuelve columna",
			input: Matrix{{1, 2, 3}},
			want:  Matrix{{1}, {2}, {3}},
		},
		{
			name:  "un solo elemento",
			input: Matrix{{7}},
			want:  Matrix{{7}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Rotate90(tc.input)

			assertDimensions(t, "rotada", got, tc.want.Rows(), tc.want.Cols())
			for i := range tc.want {
				for j := range tc.want[i] {
					if got[i][j] != tc.want[i][j] {
						t.Errorf("rotada[%d][%d] = %g, se esperaba %g", i, j, got[i][j], tc.want[i][j])
					}
				}
			}
		})
	}
}

// TestRotate90FourTimesIsIdentity comprueba la propiedad de grupo: cuatro
// rotaciones de 90° devuelven la matriz original.
func TestRotate90FourTimesIsIdentity(t *testing.T) {
	original := Matrix{{1, 2, 3}, {4, 5, 6}}

	got := Rotate90(Rotate90(Rotate90(Rotate90(original))))

	assertDimensions(t, "rotada 4 veces", got, original.Rows(), original.Cols())
	for i := range original {
		for j := range original[i] {
			if got[i][j] != original[i][j] {
				t.Errorf("[%d][%d] = %g, se esperaba %g", i, j, got[i][j], original[i][j])
			}
		}
	}
}
