package matrix

// Rotate90 devuelve la matriz rotada 90° en sentido horario: una matriz de m×n
// produce una de n×m.
//
// Este endpoint existe por una razón de interpretación del enunciado, no
// matemática. El PDF del desafío se contradice: la sección "Arquitectura"
// habla de "realizar la rotación de la matriz", mientras que "Funcionalidad
// requerida" pide explícitamente "la factorización QR de dicha matriz". Se
// implementó QR por ser el requisito funcional explícito, y esta rotación
// clásica queda disponible para cubrir la lectura alternativa sin ambigüedad.
// Ver docs/DECISIONS.md.
func Rotate90(m Matrix) Matrix {
	rows, cols := m.Rows(), m.Cols()
	out := New(cols, rows)
	for i := 0; i < rows; i++ {
		for j := 0; j < cols; j++ {
			// El elemento (i, j) pasa a la fila j y a la columna espejada
			// rows−1−i: la primera fila del original se convierte en la última
			// columna del resultado.
			out[j][rows-1-i] = m[i][j]
		}
	}
	return out
}
