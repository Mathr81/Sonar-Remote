import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";

// Built assets are embedded into the Go binary (web/dist). Served from "/".
export default defineConfig({
  plugins: [svelte()],
  build: {
    outDir: "dist",
    emptyOutDir: true,
    target: "es2020",
  },
  server: {
    // During `bun run dev`, proxy API calls to the running Go server.
    proxy: {
      "/api": "http://127.0.0.1:8080",
    },
  },
});
