// Package matrix implementa las operaciones de álgebra lineal del desafío:
// validación de matrices rectangulares y factorización QR mediante reflexiones
// de Householder.
//
// El paquete es deliberadamente independiente del framework HTTP y no usa
// librerías externas de álgebra lineal: así la lógica numérica puede probarse
// de forma aislada y el algoritmo queda a la vista, que es lo que evalúa el
// desafío.
package matrix

import "math"

// Matrix representa una matriz densa de m filas por n columnas, almacenada como
// un slice de filas. Es exactamente la forma en que viaja por JSON
// (`[[1,2],[3,4]]`), lo que evita conversiones en la capa de transporte.
type Matrix [][]float64

// New construye una matriz de rows×cols inicializada en ceros.
func New(rows, cols int) Matrix {
	m := make(Matrix, rows)
	// Una única reserva contigua para todos los elementos: mejora la localidad
	// de caché frente a asignar cada fila por separado.
	backing := make([]float64, rows*cols)
	for i := range m {
		m[i] = backing[i*cols : (i+1)*cols : (i+1)*cols]
	}
	return m
}

// Identity construye la matriz identidad de tamaño n×n.
func Identity(n int) Matrix {
	m := New(n, n)
	for i := 0; i < n; i++ {
		m[i][i] = 1
	}
	return m
}

// Rows devuelve la cantidad de filas (m).
func (m Matrix) Rows() int { return len(m) }

// Cols devuelve la cantidad de columnas (n). Asume que la matriz ya pasó por
// Validate, es decir, que todas las filas tienen el mismo largo.
func (m Matrix) Cols() int {
	if len(m) == 0 {
		return 0
	}
	return len(m[0])
}

// Clone devuelve una copia profunda, para no mutar la matriz de entrada.
func (m Matrix) Clone() Matrix {
	out := New(m.Rows(), m.Cols())
	for i := range m {
		copy(out[i], m[i])
	}
	return out
}

// Transpose devuelve la transpuesta de la matriz.
func (m Matrix) Transpose() Matrix {
	out := New(m.Cols(), m.Rows())
	for i := range m {
		for j := range m[i] {
			out[j][i] = m[i][j]
		}
	}
	return out
}

// Mul devuelve el producto matricial m·other, o nil si las dimensiones no son
// compatibles. Se usa para verificar la factorización (Q·R debe reconstruir A)
// y en el endpoint de diagnóstico.
func (m Matrix) Mul(other Matrix) Matrix {
	if m.Cols() != other.Rows() {
		return nil
	}
	out := New(m.Rows(), other.Cols())
	for i := 0; i < m.Rows(); i++ {
		for k := 0; k < m.Cols(); k++ {
			// Recorrido i-k-j: mantiene fija la fila de `other` en el bucle
			// interno y recorre memoria contigua en ambas matrices.
			a := m[i][k]
			if a == 0 {
				continue
			}
			for j := 0; j < other.Cols(); j++ {
				out[i][j] += a * other[k][j]
			}
		}
	}
	return out
}

// MaxAbs devuelve el mayor valor absoluto presente en la matriz. Sirve como
// escala de referencia para construir tolerancias relativas.
func (m Matrix) MaxAbs() float64 {
	max := 0.0
	for i := range m {
		for _, v := range m[i] {
			if a := math.Abs(v); a > max {
				max = a
			}
		}
	}
	return max
}

// IsUpperTriangular indica si todos los elementos bajo la diagonal principal
// son cero dentro de la tolerancia dada.
func (m Matrix) IsUpperTriangular(tol float64) bool {
	for i := range m {
		for j := 0; j < i && j < len(m[i]); j++ {
			if math.Abs(m[i][j]) > tol {
				return false
			}
		}
	}
	return true
}

// scrubNegativeZero reemplaza -0.0 por 0.0 en toda la matriz.
//
// El cero negativo es un valor válido de IEEE-754 y aparece de forma natural al
// multiplicar por cero, pero se serializa como `-0` en JSON, lo que resulta
// confuso para quien consume la API. Como -0.0 == 0.0 en toda comparación
// aritmética, normalizarlo no altera ningún resultado.
func (m Matrix) scrubNegativeZero() {
	for i := range m {
		for j := range m[i] {
			if m[i][j] == 0 {
				m[i][j] = 0
			}
		}
	}
}
