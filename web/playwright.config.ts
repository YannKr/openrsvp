import { defineConfig } from '@playwright/test';

export default defineConfig({
	testDir: './tests/visual',
	outputDir: './test-results',
	snapshotDir: './tests/visual/snapshots',
	snapshotPathTemplate: '{snapshotDir}/{testFilePath}/{arg}{ext}',
	fullyParallel: true,
	forbidOnly: !!process.env.CI,
	retries: process.env.CI ? 2 : 0,
	workers: process.env.CI ? 1 : undefined,
	reporter: 'html',
	use: {
		baseURL: process.env.BASE_URL || 'http://localhost:8080',
		screenshot: 'only-on-failure',
	},
	projects: [
		{
			name: 'desktop-light',
			use: {
				viewport: { width: 1280, height: 720 },
				colorScheme: 'light',
			},
		},
		{
			name: 'desktop-dark',
			use: {
				viewport: { width: 1280, height: 720 },
				// Dark mode is set via localStorage in tests
			},
		},
		{
			name: 'mobile-light',
			use: {
				viewport: { width: 375, height: 812 },
				colorScheme: 'light',
			},
		},
	],
});
