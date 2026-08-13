import { describe, expect, it } from 'vitest';
import {
  computeMatrixStatistics,
  computeStatistics,
  isDiagonal,
  MIN_TOLERANCE,
  TOLERANCE_FACTOR,
} from '../../src/services/statistics.service.js';

describe('computeMatrixStatistics', () => {
  it('calcula los cuatro agregados que pide el enunciado', () => {
    const stats = computeMatrixStatistics([
      [1, 2, 3],
      [4, 5, 6],
    ]);

    expect(stats.max).toBe(6);
    expect(stats.min).toBe(1);
    expect(stats.sum).toBe(21);
    expect(stats.average).toBe(3.5);
    expect(stats.count).toBe(6);
    expect(stats.rows).toBe(2);
    expect(stats.cols).toBe(3);
    expect(stats.isSquare).toBe(false);
  });

  it('maneja valores negativos en los extremos', () => {
    const stats = computeMatrixStatistics([
      [-10, -2],
      [-7, -1],
    ]);

    expect(stats.max).toBe(-1);
    expect(stats.min).toBe(-10);
    expect(stats.sum).toBe(-20);
    expect(stats.average).toBe(-5);
  });

  it('trata una matriz de un solo elemento', () => {
    const stats = computeMatrixStatistics([[42]]);

    expect(stats).toMatchObject({
      max: 42,
      min: 42,
      sum: 42,
      average: 42,
      count: 1,
      isSquare: true,
      isDiagonal: true,
    });
  });

  /**
   * Sin compensación, sumar 1e16 + 1 pierde el 1 por completo (el resultado no
   * es representable) y el total daría 0 en lugar de 1.
   */
  it('conserva la precisión al sumar magnitudes muy distintas', () => {
    const stats = computeMatrixStatistics([[1e16, 1, -1e16]]);

    expect(stats.sum).toBe(1);

    // Comprobación explícita de que la suma ingenua sí falla, para que el test
    // documente por qué existe el acumulador compensado.
    const naive = [1e16, 1, -1e16].reduce((acc, value) => acc + value, 0);
    expect(naive).toBe(0);
  });
});

describe('isDiagonal', () => {
  it('reconoce una matriz diagonal exacta', () => {
    expect(
      isDiagonal(
        [
          [5, 0, 0],
          [0, 3, 0],
          [0, 0, 1],
        ],
        MIN_TOLERANCE,
      ),
    ).toBe(true);
  });

  it('rechaza una matriz con un elemento fuera de la diagonal', () => {
    expect(
      isDiagonal(
        [
          [5, 0],
          [7, 3],
        ],
        MIN_TOLERANCE,
      ),
    ).toBe(false);
  });

  it('acepta residuos de redondeo dentro de la tolerancia', () => {
    expect(
      isDiagonal(
        [
          [5, 1e-15],
          [3e-16, 3],
        ],
        1e-12,
      ),
    ).toBe(true);
  });

  it('rechaza valores fuera de la diagonal que superan la tolerancia', () => {
    expect(
      isDiagonal(
        [
          [5, 1e-6],
          [0, 3],
        ],
        1e-12,
      ),
    ).toBe(false);
  });

  it('considera diagonal la matriz nula', () => {
    // No tiene ningún elemento no nulo fuera de la diagonal, así que cumple la
    // definición.
    expect(
      isDiagonal(
        [
          [0, 0],
          [0, 0],
        ],
        MIN_TOLERANCE,
      ),
    ).toBe(true);
  });

  it('aplica la definición generalizada a matrices rectangulares', () => {
    // Una matriz 3×2 sin elementos no nulos fuera de la diagonal principal.
    expect(
      isDiagonal(
        [
          [5, 0],
          [0, 3],
          [0, 0],
        ],
        MIN_TOLERANCE,
      ),
    ).toBe(true);
  });
});

describe('tolerancia relativa', () => {
  it('escala la tolerancia con la magnitud de cada matriz', () => {
    const stats = computeMatrixStatistics([
      [1e9, 0],
      [0, 2e9],
    ]);

    expect(stats.tolerance).toBeCloseTo(TOLERANCE_FACTOR * 2e9, 10);
    expect(stats.isDiagonal).toBe(true);
  });

  it('aplica el piso absoluto cuando la matriz es de magnitud despreciable', () => {
    const stats = computeMatrixStatistics([
      [0, 0],
      [0, 0],
    ]);

    expect(stats.tolerance).toBe(MIN_TOLERANCE);
  });

  /**
   * Este es el motivo de derivar la tolerancia por matriz y no del conjunto:
   * con una tolerancia global tomada de la matriz de mayor magnitud (1e9 →
   * tolerancia 1), el 1e-3 fuera de la diagonal de la matriz pequeña quedaría
   * enmascarado y esa matriz se reportaría como diagonal sin serlo.
   */
  it('no enmascara valores significativos de una matriz de magnitud pequeña', () => {
    const result = computeStatistics({
      pequena: [
        [1, 1e-3],
        [0, 1],
      ],
      grande: [
        [1e9, 0],
        [0, 1e9],
      ],
    });

    expect(result.perMatrix.pequena.isDiagonal).toBe(false);
    expect(result.perMatrix.grande.isDiagonal).toBe(true);
  });
});

describe('computeStatistics', () => {
  const q = [
    [1, 0],
    [0, 1],
  ];
  const r = [
    [10, 20],
    [0, 40],
  ];

  it('agrega los valores de todas las matrices', () => {
    const result = computeStatistics({ q, r });

    expect(result.overall.max).toBe(40);
    expect(result.overall.min).toBe(0);
    expect(result.overall.sum).toBe(72); // (1+0+0+1) + (10+20+0+40)
    expect(result.overall.count).toBe(8);
    expect(result.overall.average).toBe(9);
  });

  it('entrega el desglose por matriz', () => {
    const result = computeStatistics({ q, r });

    expect(Object.keys(result.perMatrix)).toEqual(['q', 'r']);
    expect(result.perMatrix.q.sum).toBe(2);
    expect(result.perMatrix.r.sum).toBe(70);
  });

  it('marca anyDiagonal cuando al menos una matriz lo es', () => {
    const result = computeStatistics({ q, r });

    expect(result.perMatrix.q.isDiagonal).toBe(true);
    expect(result.perMatrix.r.isDiagonal).toBe(false);
    expect(result.anyDiagonal).toBe(true);
  });

  it('deja anyDiagonal en false cuando ninguna lo es', () => {
    const result = computeStatistics({
      a: [
        [1, 2],
        [3, 4],
      ],
      b: [
        [5, 6],
        [7, 8],
      ],
    });

    expect(result.anyDiagonal).toBe(false);
  });

  it('funciona con una sola matriz', () => {
    const result = computeStatistics({ unica: r });

    expect(result.overall.sum).toBe(70);
    expect(result.overall.count).toBe(4);
  });

  it('acepta matrices de dimensiones distintas entre sí', () => {
    const result = computeStatistics({
      alta: [[1], [2], [3]],
      ancha: [[4, 5, 6]],
    });

    expect(result.overall.count).toBe(6);
    expect(result.overall.sum).toBe(21);
    expect(result.overall.max).toBe(6);
    expect(result.overall.min).toBe(1);
  });

  it('informa el factor de tolerancia usado', () => {
    expect(computeStatistics({ q }).toleranceFactor).toBe(TOLERANCE_FACTOR);
  });
});
