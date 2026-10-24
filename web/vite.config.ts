import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';
import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [
		tailwindcss(),
		sveltekit({
			preprocess: vitePreprocess(),
			adapter: adapter({
				pages: 'build',
				assets: 'build',
				fallback: '200.html',
				precompress: false,
				strict: false
			})
		})
	],
	server: {
		proxy: {
			'/api': {
				target: 'http://localhost:8090',
				changeOrigin: true
			}
		}
	}
});
