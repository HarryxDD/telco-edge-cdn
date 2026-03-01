import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react-swc";

// Vite + Vitest configuration for the CDN Streaming App frontend
export default defineConfig({
    plugins: [react()],
    server: {
        proxy: {
            "/api": {
                target: "http://localhost:8080", // containerlab LB port
                changeOrigin: true,
                secure: false,
            },
            "/hls": {
                target: "http://localhost:8080", // containerlab LB port
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
