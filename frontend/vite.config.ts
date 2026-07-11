import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// During local development we proxy API calls to the Go backend so the browser
// talks to a single origin (http://localhost:5173) and avoids CORS entirely.
// In production the app is served as static files and calls the API via the
// Ingress on the same origin, so no proxy is needed.
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      "/health": "http://localhost:8080",
      "/stats": "http://localhost:8080",
      "/upload": "http://localhost:8080",
      "/files": "http://localhost:8080",
      "/simulate": "http://localhost:8080",
    },
  },
});
