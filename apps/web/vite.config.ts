import { resolve } from "node:path";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

const configDir = import.meta.dirname;

export default defineConfig({
	plugins: [react()],
	server: {
		proxy: {
			"/api": "http://localhost:8080",
			"/ws": {
				target: "ws://localhost:8080",
				ws: true,
			},
		},
	},
	build: {
		rollupOptions: {
			input: {
				main: resolve(configDir, "index.html"),
				debug: resolve(configDir, "debug/index.html"),
				iolite: resolve(configDir, "iolite/index.html"),
			},
		},
	},
});
