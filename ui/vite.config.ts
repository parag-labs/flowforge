import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// The console talks to the engine; in dev we proxy /api to the local engine so the UI
// and engine can run on different ports without CORS.
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/api": {
        target: "http://localhost:8080",
        changeOrigin: true,
        rewrite: (p) => p.replace(/^\/api/, ""),
      },
    },
  },
});
