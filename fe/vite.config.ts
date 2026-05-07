import { defineConfig, loadEnv } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig(({ mode }) => {
  const rootEnv = loadEnv(mode, '..', '');
  const configuredProxyTarget = process.env.VITE_API_PROXY_TARGET || rootEnv.VITE_API_PROXY_TARGET;
  const apiHostPort = process.env.API_HOST_PORT || rootEnv.API_HOST_PORT;
  if (!configuredProxyTarget && !apiHostPort) {
    throw new Error('API_HOST_PORT must be set in the root .env file.');
  }
  const apiProxyTarget = configuredProxyTarget || `http://localhost:${apiHostPort}`;
  const frontendContainerPort = Number(process.env.FRONTEND_CONTAINER_PORT || rootEnv.FRONTEND_CONTAINER_PORT || 3000);

  return {
    plugins: [react()],
    server: {
      port: frontendContainerPort,
      proxy: {
        '/api': {
          target: apiProxyTarget,
          changeOrigin: true
        },
        '/mcp': {
          target: apiProxyTarget,
          changeOrigin: true
        }
      }
    },
    test: {
      environment: 'jsdom',
      setupFiles: './src/testSetup.ts'
    }
  };
});
