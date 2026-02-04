import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react-swc";

// Vite + Vitest configuration for the CDN Streaming App frontend
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      // Proxy API and HLS requests to the Go backend running on :8080
      "/api": {
        target: "https://localhost:8443",
        changeOrigin: true,
        secure: false,
      },
      "/hls": {
        target: "https://localhost:8443",
        changeOrigin: true,
        secure: false,
      },
    },
  },
  test: {
    globals: true,
    environment: "jsdom",
    setupFiles: "./src/setupTests.ts",
  },
});
