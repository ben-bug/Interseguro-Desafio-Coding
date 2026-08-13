package matrix

import "math"

// Mode selecciona la variante de factorización QR a calcular.
type Mode string

const (
	// ModeFull produce la factorización completa: Q de m×m (ortogonal) y R de
	// m×n (triangular superior). Es la definición canónica y la que se usa por
	// defecto, porque está definida para cualquier matriz rectangular.
	ModeFull Mode = "full"

	// ModeReduced produce la factorización reducida ("thin"): cuando m > n,
	// Q queda de m×n y R de n×n, descartando las columnas de Q que no aportan
	// al producto. Cuando m ≤ n ambas variantes coinciden.
	ModeReduced Mode = "reduced"
)

// Decomposition es el resultado de factorizar A = Q·R.
type Decomposition struct {
	Q Matrix
	R Matrix
	// Mode es la variante efectivamente aplicada.
	Mode Mode
	// Residual es el error relativo de reconstrucción ‖Q·R − A‖_F / ‖A‖_F.
	// Se expone en la respuesta como evidencia verificable de que el resultado
	// es correcto: en aritmética de doble precisión debe rondar 1e-16.
	Residual float64
}

// QR factoriza A = Q·R usando reflexiones de Householder.
//
// Cada paso k construye una matriz de reflexión H_k = I − 2vvᵀ que anula todos
// los elementos bajo la diagonal de la columna k. Tras p = mín(m−1, n) pasos,
// R queda triangular superior y Q = H_1·H_2·…·H_p resulta ortogonal.
//
// Se eligió Householder sobre Gram-Schmidt porque este último pierde
// ortogonalidad en Q cuando las columnas de A están casi alineadas (matrices
// mal condicionadas), mientras que Householder es incondicionalmente estable:
// las reflexiones son transformaciones ortogonales y no amplifican el error.
//
// La matriz de entrada no se modifica. Complejidad: O(m·n²) en tiempo para R
// y O(m²·n) para acumular Q explícitamente.
func QR(a Matrix, mode Mode) Decomposition {
	m, n := a.Rows(), a.Cols()

	r := a.Clone()
	q := Identity(m)

	// v es el vector de Householder del paso actual. Se reserva una sola vez y
	// se reutiliza en cada iteración: solo el tramo v[k:m] es significativo.
	v := make([]float64, m)

	// Con m ≤ n el último paso útil es m−1: la fila final ya no tiene elementos
	// bajo la diagonal que anular.
	steps := min(m-1, n)

	for k := 0; k < steps; k++ {
		// x = R[k:m][k], la porción de la columna k que queda por anular.
		for i := k; i < m; i++ {
			v[i] = r[i][k]
		}

		normX := norm2(v[k:m])
		if normX == 0 {
			// La columna ya es nula bajo la diagonal: no hay nada que reflejar.
			continue
		}

		// alpha = −signo(x₀)·‖x‖. Elegir el signo opuesto a x₀ maximiza |v₀| y
		// evita la cancelación catastrófica que ocurriría si x₀ ya estuviera
		// cerca de ‖x‖ y los restáramos.
		alpha := -math.Copysign(normX, v[k])

		// v = x − alpha·e₁, luego normalizado a ‖v‖ = 1.
		v[k] -= alpha
		normV := norm2(v[k:m])
		if normV == 0 {
			continue
		}
		for i := k; i < m; i++ {
			v[i] /= normV
		}

		// R ← H_k·R, aplicado solo al bloque activo [k:m]×[k:n].
		// H_k·R = R − 2v(vᵀR): nunca se materializa H_k, lo que baja el coste
		// del paso de O(m²·n) a O((m−k)·(n−k)).
		for j := k; j < n; j++ {
			dot := 0.0
			for i := k; i < m; i++ {
				dot += v[i] * r[i][j]
			}
			dot *= 2
			for i := k; i < m; i++ {
				r[i][j] -= dot * v[i]
			}
		}

		// Q ← Q·H_k. Se acumula por la derecha porque Q = H_1·H_2·…·H_p, y
		// cada H_k es simétrica, de modo que basta con el mismo producto.
		for i := 0; i < m; i++ {
			dot := 0.0
			for j := k; j < m; j++ {
				dot += q[i][j] * v[j]
			}
			dot *= 2
			for j := k; j < m; j++ {
				q[i][j] -= dot * v[j]
			}
		}
	}

	// Los elementos bajo la diagonal de R son cero por construcción: lo que
	// queda tras el bucle es únicamente ruido de redondeo del orden de ε·‖A‖.
	// Se anulan explícitamente para que R cumpla su definición de forma exacta
	// y el consumidor no reciba valores como 3.5e-17 donde espera un cero.
	for i := 1; i < m; i++ {
		for j := 0; j < i && j < n; j++ {
			r[i][j] = 0
		}
	}

	effectiveMode := ModeFull
	if mode == ModeReduced {
		effectiveMode = ModeReduced
		if m > n {
			// Se conservan las primeras n columnas de Q y las primeras n filas
			// de R; el resto solo multiplica el bloque nulo inferior de R.
			thinQ := New(m, n)
			for i := 0; i < m; i++ {
				copy(thinQ[i], q[i][:n])
			}
			thinR := New(n, n)
			for i := 0; i < n; i++ {
				copy(thinR[i], r[i])
			}
			q, r = thinQ, thinR
		}
	}

	q.scrubNegativeZero()
	r.scrubNegativeZero()

	return Decomposition{
		Q:        q,
		R:        r,
		Mode:     effectiveMode,
		Residual: Residual(a, q, r),
	}
}

// Residual devuelve el error relativo de reconstrucción ‖Q·R − A‖_F / ‖A‖_F.
// Devuelve 0 cuando A es la matriz nula, caso en que el error relativo no está
// definido y el absoluto es necesariamente cero.
func Residual(a, q, r Matrix) float64 {
	product := q.Mul(r)
	if product == nil {
		return math.NaN()
	}

	var diffSq, refSq float64
	for i := range a {
		for j := range a[i] {
			d := product[i][j] - a[i][j]
			diffSq += d * d
			refSq += a[i][j] * a[i][j]
		}
	}
	if refSq == 0 {
		return 0
	}
	return math.Sqrt(diffSq / refSq)
}

// norm2 calcula la norma euclidiana ‖x‖₂ con escalado previo.
//
// La suma directa de cuadrados desbordaría con valores del orden de 1e200 y se
// anularía por underflow con valores del orden de 1e-200, aun cuando la norma
// resultante fuese perfectamente representable. Dividir por el mayor valor
// absoluto antes de elevar al cuadrado evita ambos extremos; es la misma
// estrategia que usa la rutina DNRM2 de LAPACK.
func norm2(x []float64) float64 {
	scale := 0.0
	for _, v := range x {
		if a := math.Abs(v); a > scale {
			scale = a
		}
	}
	if scale == 0 {
		return 0
	}

	sum := 0.0
	for _, v := range x {
		t := v / scale
		sum += t * t
	}
	return scale * math.Sqrt(sum)
}
