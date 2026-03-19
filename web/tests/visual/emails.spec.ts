import { test, expect, type Page } from '@playwright/test';

/**
 * Email template visual regression tests via Mailpit.
 *
 * These tests navigate to the Mailpit web UI, enumerate captured emails,
 * and screenshot each one for visual regression tracking.  They are
 * designed to skip gracefully when Mailpit is unavailable (e.g. CI, or
 * when Docker is not running).
 *
 * To populate Mailpit with sample emails, run the helper script first:
 *   ./tests/visual/generate-test-emails.sh
 */

const MAILPIT_URL = process.env.MAILPIT_URL || 'http://localhost:8025';
const MAILPIT_API = `${MAILPIT_URL}/api/v1`;

interface MailpitMessage {
	ID: string;
	Subject: string;
}

interface MailpitListResponse {
	total: number;
	messages: MailpitMessage[];
}

/**
 * Fetch the list of messages from the Mailpit API.
 * Returns null if the API is unreachable.
 */
async function fetchMailpitMessages(): Promise<MailpitListResponse | null> {
	try {
		const res = await fetch(`${MAILPIT_API}/messages?limit=50`);
		if (!res.ok) return null;
		return (await res.json()) as MailpitListResponse;
	} catch {
		return null;
	}
}

/**
 * Sanitize a subject line into a safe filename fragment.
 */
function slugify(subject: string): string {
	return subject
		.toLowerCase()
		.replace(/[^a-z0-9]+/g, '-')
		.replace(/^-|-$/g, '')
		.slice(0, 60);
}

/**
 * Navigate to a single email in Mailpit and screenshot the HTML preview.
 */
async function screenshotEmail(page: Page, messageId: string, name: string) {
	// Mailpit renders individual messages at /message/<id>
	await page.goto(`${MAILPIT_URL}/message/${messageId}`, { timeout: 10000 });
	await page.waitForLoadState('networkidle');

	// Mailpit renders the email body inside an iframe with id "preview-html"
	const iframe = page.frameLocator('#preview-html');
	const body = iframe.locator('body');

	// Wait for the iframe content to load
	await body.waitFor({ state: 'visible', timeout: 10000 }).catch(() => {
		// Fall back to screenshotting the full page if iframe is missing
	});

	await expect(page).toHaveScreenshot(name, {
		fullPage: true,
		maxDiffPixelRatio: 0.05,
	});
}

// Only run on the desktop-light project — email templates don't vary by dark mode or viewport
test.describe('Email template visual regression', () => {
	test.beforeEach(async ({}, testInfo) => {
		if (testInfo.project.name !== 'desktop-light') {
			test.skip(true, 'Email tests only run in desktop-light project');
		}
	});

	test('mailpit inbox overview', async ({ page }) => {
		const data = await fetchMailpitMessages();

		if (!data) {
			test.skip(true, 'Mailpit is not running — skipping email tests');
			return;
		}

		if (data.total === 0) {
			test.skip(true, 'No emails in Mailpit — run generate-test-emails.sh first');
			return;
		}

		// Screenshot the inbox
		await page.goto(MAILPIT_URL, { timeout: 10000 });
		await page.waitForLoadState('networkidle');
		await expect(page).toHaveScreenshot('mailpit-inbox.png', {
			fullPage: true,
			maxDiffPixelRatio: 0.05,
		});
	});

	test('individual email templates', async ({ page }) => {
		const data = await fetchMailpitMessages();

		if (!data) {
			test.skip(true, 'Mailpit is not running — skipping email tests');
			return;
		}

		if (data.total === 0) {
			test.skip(true, 'No emails in Mailpit — run generate-test-emails.sh first');
			return;
		}

		// Screenshot each email
		for (const msg of data.messages) {
			const slug = slugify(msg.Subject) || msg.ID.slice(0, 12);
			await screenshotEmail(page, msg.ID, `email-${slug}.png`);
		}
	});
});
