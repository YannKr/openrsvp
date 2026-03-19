import { test, expect } from '@playwright/test';

test.describe('Visual regression - public pages', () => {
	test('landing page', async ({ page }) => {
		await page.goto('/');
		await page.waitForLoadState('networkidle');
		await expect(page).toHaveScreenshot('landing.png', {
			fullPage: true,
			maxDiffPixelRatio: 0.01,
		});
	});

	test('login page', async ({ page }) => {
		await page.goto('/auth/login');
		await page.waitForLoadState('networkidle');
		await expect(page).toHaveScreenshot('login.png', {
			fullPage: true,
			maxDiffPixelRatio: 0.01,
		});
	});

	test('design system gallery', async ({ page }) => {
		await page.goto('/design');
		await page.waitForLoadState('networkidle');
		await expect(page).toHaveScreenshot('design-gallery.png', {
			fullPage: true,
			maxDiffPixelRatio: 0.01,
		});
	});
});

test.describe('Visual regression - dark mode', () => {
	test.use({
		storageState: undefined,
	});

	test.beforeEach(async ({ page }) => {
		// Set dark theme in localStorage before any page scripts run.
		// The app's inline script in app.html reads this and applies
		// data-theme="dark" on the <html> element.
		await page.addInitScript(() => {
			localStorage.setItem('theme', 'dark');
		});
	});

	test('landing page - dark', async ({ page }) => {
		await page.goto('/');
		await page.waitForLoadState('networkidle');
		await expect(page).toHaveScreenshot('landing-dark.png', {
			fullPage: true,
			maxDiffPixelRatio: 0.01,
		});
	});

	test('login page - dark', async ({ page }) => {
		await page.goto('/auth/login');
		await page.waitForLoadState('networkidle');
		await expect(page).toHaveScreenshot('login-dark.png', {
			fullPage: true,
			maxDiffPixelRatio: 0.01,
		});
	});

	test('design gallery - dark', async ({ page }) => {
		await page.goto('/design');
		await page.waitForLoadState('networkidle');
		await expect(page).toHaveScreenshot('design-gallery-dark.png', {
			fullPage: true,
			maxDiffPixelRatio: 0.01,
		});
	});
});
