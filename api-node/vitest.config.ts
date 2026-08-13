import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    environment: 'node',
    include: ['tests/**/*.test.ts'],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'html'],
      include: ['src/**/*.ts'],
      // server.ts solo abre el puerto y registra los manejadores de señales:
      // se ejercita levantando el servicio, no con pruebas unitarias.
      exclude: ['src/server.ts'],
    },
  },
});
