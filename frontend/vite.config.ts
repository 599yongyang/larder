import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import wails from "@wailsio/runtime/plugins/vite";
import path from "node:path";
import { mkdirSync, writeFileSync } from "node:fs";

// https://vitejs.dev/config/
export default defineConfig({
  server: {
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
  resolve: {
    alias: { "@": path.resolve(import.meta.dirname, "src") },
  },
  plugins: [
    react(),
    tailwindcss(),
    wails("./bindings"),
    {
      name: "keep-dist-marker",
      closeBundle() {
        const dir = path.resolve(import.meta.dirname, "dist");
        mkdirSync(dir, { recursive: true });
        writeFileSync(path.join(dir, ".keep"), "");
      },
    },
  ],
});
