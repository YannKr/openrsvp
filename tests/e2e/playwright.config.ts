import { defineConfig } from '@playwright/test';

export default defineConfig({
	testDir: '.',
	timeout: 30_000,
	retries: 0,
	use: {
		baseURL: 'http://localhost:8091',
		headless: true,
		screenshot: 'only-on-failure',
		trace: 'retain-on-failure'
	},
	reporter: [['list']],
	projects: [
		{
			name: 'chromium',
			use: { browserName: 'chromium' }
		}
	]
});
