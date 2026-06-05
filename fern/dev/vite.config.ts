import path from "node:path";

import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: [
      { find: "react/jsx-runtime", replacement: path.resolve(__dirname, "node_modules/react/jsx-runtime.js") },
      { find: "react", replacement: path.resolve(__dirname, "node_modules/react/index.js") },
      { find: "react-dom/client", replacement: path.resolve(__dirname, "node_modules/react-dom/client.js") },
    ],
    dedupe: ["react", "react-dom"],
  },
  server: {
    fs: {
      allow: [path.resolve(__dirname, "..")],
    },
  },
});
