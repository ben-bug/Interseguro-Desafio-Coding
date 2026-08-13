import { Router, type Request, type Response, type NextFunction } from 'express';
import type { Config } from '../config.js';
import { requireJwt } from '../middleware/auth.js';
import { createStatisticsSchema, zodErrorToApiError } from '../schemas/statistics.schema.js';
import { computeStatistics, type StatisticsResult } from '../services/statistics.service.js';

/**
 * Monta POST /api/v1/statistics.
 *
 * Es el endpoint que consume la API Go tras factorizar: recibe las matrices Q y
 * R y devuelve las estadísticas pedidas por el enunciado.
 */
export function statisticsRouter(config: Config): Router {
  const router = Router();
  const schema = createStatisticsSchema({
    maxMatrixDimension: config.maxMatrixDimension,
    maxMatrices: config.maxMatrices,
  });

  router.post(
    '/statistics',
    requireJwt(config),
    (req: Request, res: Response<StatisticsResult>, next: NextFunction) => {
      const parsed = schema.safeParse(req.body);
      if (!parsed.success) {
        next(zodErrorToApiError(parsed.error));
        return;
      }

      res.json(computeStatistics(parsed.data.matrices));
    },
  );

  return router;
}
