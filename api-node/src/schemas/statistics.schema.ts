/**
 * Validación del cuerpo de POST /api/v1/statistics.
 *
 * Los errores de forma se detectan con Zod, pero cada regla de dominio adjunta
 * su propio código estable en `params`. Así el cliente recibe RAGGED_ROWS en
 * lugar de un mensaje genérico de validación, y —lo que importa más— recibe
 * exactamente los mismos códigos que emite la API Go ante el mismo problema.
 */

import { z } from 'zod';
import { ApiError, ErrorCode, type ErrorCodeValue } from '../errors.js';

/** Información que cada regla adjunta a su issue para poder traducirlo. */
interface IssueParams {
  errorCode: ErrorCodeValue;
  details?: Record<string, unknown>;
}

/** Límites aplicados a la entrada. */
export interface SchemaLimits {
  maxMatrixDimension: number;
  maxMatrices: number;
}

/** Cuerpo válido de la petición. */
export interface StatisticsRequest {
  matrices: Record<string, number[][]>;
}

/**
 * Construye el esquema con los límites de la configuración.
 *
 * Los límites se inyectan en vez de leerse del entorno acá para que los tests
 * puedan ejercitar los rechazos con matrices pequeñas.
 */
export function createStatisticsSchema(limits: SchemaLimits) {
  const matrixSchema = z
    .array(z.array(z.number()))
    .superRefine((rows: number[][], ctx: z.RefinementCtx) => {
      if (rows.length === 0) {
        addIssue(ctx, ErrorCode.EMPTY_MATRIX, 'la matriz debe tener al menos una fila');
        return;
      }

      const cols = rows[0].length;
      if (cols === 0) {
        addIssue(ctx, ErrorCode.EMPTY_MATRIX, 'la matriz debe tener al menos una columna');
        return;
      }

      if (rows.length > limits.maxMatrixDimension || cols > limits.maxMatrixDimension) {
        addIssue(
          ctx,
          ErrorCode.MATRIX_TOO_LARGE,
          `la matriz de ${rows.length}×${cols} supera el límite de ${limits.maxMatrixDimension} por dimensión`,
          { rows: rows.length, cols, maxDimension: limits.maxMatrixDimension },
        );
        return;
      }

      for (let i = 0; i < rows.length; i += 1) {
        if (rows[i].length !== cols) {
          addIssue(
            ctx,
            ErrorCode.RAGGED_ROWS,
            `todas las filas deben tener el mismo largo: la fila 0 tiene ${cols} columnas y la fila ${i} tiene ${rows[i].length}`,
            { expectedCols: cols, rowIndex: i, actualCols: rows[i].length },
          );
          return;
        }

        // Defensa en profundidad: JSON no tiene literales para NaN ni infinito,
        // pero el módulo puede invocarse desde otro transporte.
        for (let j = 0; j < cols; j += 1) {
          if (!Number.isFinite(rows[i][j])) {
            addIssue(
              ctx,
              ErrorCode.NON_FINITE_VALUE,
              `la posición [${i}][${j}] contiene un valor no finito (NaN o infinito)`,
              { rowIndex: i, colIndex: j },
            );
            return;
          }
        }
      }
    });

  return z.object({
    matrices: z
      .record(z.string(), matrixSchema)
      .superRefine((matrices: Record<string, number[][]>, ctx: z.RefinementCtx) => {
        const names = Object.keys(matrices);
        if (names.length === 0) {
          addIssue(ctx, ErrorCode.NO_MATRICES, "se requiere al menos una matriz en 'matrices'");
          return;
        }
        if (names.length > limits.maxMatrices) {
          addIssue(
            ctx,
            ErrorCode.TOO_MANY_MATRICES,
            `se recibieron ${names.length} matrices y el límite es ${limits.maxMatrices}`,
            { received: names.length, maxMatrices: limits.maxMatrices },
          );
        }
      }),
  });
}

/** Adjunta un issue con su código de dominio. */
function addIssue(
  ctx: z.RefinementCtx,
  errorCode: ErrorCodeValue,
  message: string,
  details?: Record<string, unknown>,
): void {
  const params: IssueParams = { errorCode, details };
  ctx.addIssue({ code: 'custom', message, params });
}

/**
 * Traduce un error de Zod al formato de error de la API.
 *
 * Se reporta solo el primer problema encontrado: enumerar todos los issues de
 * una matriz malformada produce ruido sin agregar información accionable, ya
 * que suelen ser el mismo defecto repetido en cada fila.
 */
export function zodErrorToApiError(error: z.ZodError): ApiError {
  const issue = error.issues[0];
  if (!issue) {
    return ApiError.badRequest(ErrorCode.INVALID_BODY, 'el cuerpo del request es inválido');
  }

  // Las reglas de dominio traen su propio código; el resto son fallos de forma
  // (falta `matrices`, un elemento no es número, etc.).
  const params = (issue as { params?: IssueParams }).params;
  if (params?.errorCode) {
    return ApiError.badRequest(params.errorCode, issue.message, {
      ...params.details,
      path: issue.path.join('.'),
    });
  }

  const location = issue.path.length > 0 ? ` en '${issue.path.join('.')}'` : '';
  return ApiError.badRequest(ErrorCode.INVALID_BODY, `${issue.message}${location}`, {
    path: issue.path.join('.'),
  });
}
