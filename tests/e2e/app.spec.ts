import { test, expect } from '@playwright/test';
import {
	clearSession,
	createEventViaAPI,
	getOrCreateSession,
	setSessionInBrowser,
	submitRSVPViaAPI
} from './helpers';

const RUN_ID = Date.now();
const ORGANIZER_EMAIL = `e2e-organizer-${RUN_ID}@example.com`;

let sessionToken: string;
let testEventId: string;
let testShareToken: string;
let testRsvpToken: string;

test.describe.serial('OpenRSVP E2E', () => {
	// ─── Landing & Health ────────────────────────────────
	test('landing page loads with app branding', async ({ page }) => {
		await page.goto('/');
		await expect(page).toHaveTitle(/OpenRSVP/i);
	});

	test('health endpoint returns ok', async ({ request }) => {
		const res = await request.get('/health');
		expect(res.status()).toBe(200);
		expect((await res.json()).status).toBe('ok');
	});

	test('health/ready returns database connected', async ({ request }) => {
		const res = await request.get('/health/ready');
		expect(res.status()).toBe(200);
		const json = await res.json();
		expect(json.status).toBe('ok');
		expect(json.database).toBe('connected');
	});

	test('API config returns feature flags', async ({ request }) => {
		const res = await request.get('/api/v1/config');
		expect(res.status()).toBe(200);
		expect(typeof (await res.json()).data.smsEnabled).toBe('boolean');
	});

	// ─── Authentication ──────────────────────────────────
	test('login page renders with email input', async ({ page }) => {
		await page.goto('/auth/login');
		await expect(page.locator('input[type="email"]')).toBeVisible();
	});

	test('magic link request shows success message', async ({ page }) => {
		await page.goto('/auth/login');
		await page.fill('input[type="email"]', `e2e-login-ui-${RUN_ID}@example.com`);
		await page.click('button[type="submit"]');
		await expect(
			page.getByRole('heading', { name: /check your email/i })
		).toBeVisible({ timeout: 10000 });
	});

	test('authenticate via magic link', async () => {
		// Uses 2 auth requests: POST /magic-link + POST /verify.
		sessionToken = await getOrCreateSession(ORGANIZER_EMAIL);
		expect(sessionToken).toBeTruthy();
	});

	test('session grants access to protected API', async () => {
		// Verify via events endpoint (general rate limiter, not auth).
		const res = await fetch('http://localhost:8091/api/v1/events', {
			headers: { Authorization: `Bearer ${sessionToken}` }
		});
		expect(res.status).toBe(200);
	});

	test('protected API returns 401 without auth', async () => {
		const res = await fetch('http://localhost:8091/api/v1/events');
		expect(res.status).toBe(401);
	});

	test('unauthenticated user redirected from /events', async ({ page }) => {
		await page.goto('/');
		await clearSession(page);
		await page.goto('/events');
		await page.waitForURL('**/auth/login', { timeout: 10000 });
	});

	// ─── Event Management (authenticated) ────────────────
	test('events dashboard loads when authenticated', async ({ page }) => {
		await setSessionInBrowser(page, sessionToken);
		await page.goto('/events');
		await expect(page.locator('body')).not.toContainText(/Sign in to your account/, { timeout: 10000 });
	});

	test('create event page loads', async ({ page }) => {
		await setSessionInBrowser(page, sessionToken);
		await page.goto('/events/new');
		await expect(page.locator('input').first()).toBeVisible({ timeout: 10000 });
	});

	test('create event via API', async () => {
		const event = await createEventViaAPI(sessionToken);
		testEventId = (event as any).id;
		testShareToken = (event as any).shareToken;
		expect(testEventId).toBeTruthy();
		expect(testShareToken).toBeTruthy();
	});

	test('view event in browser', async ({ page }) => {
		await setSessionInBrowser(page, sessionToken);
		await page.goto(`/events/${testEventId}`);
		await expect(page.locator('body')).toContainText(/E2E Test Event/, { timeout: 10000 });
	});

	test('edit event page loads', async ({ page }) => {
		await setSessionInBrowser(page, sessionToken);
		await page.goto(`/events/${testEventId}/edit`);
		await expect(page.locator('input').first()).toBeVisible({ timeout: 10000 });
	});

	test('share page loads', async ({ page }) => {
		await setSessionInBrowser(page, sessionToken);
		await page.goto(`/events/${testEventId}/share`);
		await expect(page.locator('body')).toBeVisible();
	});

	test('event API returns correct data', async () => {
		const res = await fetch(`http://localhost:8091/api/v1/events/${testEventId}`, {
			headers: { Authorization: `Bearer ${sessionToken}` }
		});
		expect(res.status).toBe(200);
		const json = await res.json();
		expect(json.data.id).toBe(testEventId);
		expect(json.data.title).toContain('E2E Test Event');
	});

	test('events list API returns events', async () => {
		const res = await fetch('http://localhost:8091/api/v1/events', {
			headers: { Authorization: `Bearer ${sessionToken}` }
		});
		expect(res.status).toBe(200);
		expect((await res.json()).data.length).toBeGreaterThan(0);
	});

	// ─── Public RSVP Flow ────────────────────────────────
	test('public invite page loads with event details', async ({ page }) => {
		await page.goto(`/i/${testShareToken}`);
		await expect(page.locator('body')).toContainText(/E2E Test Event/, { timeout: 10000 });
	});

	test('invite page has RSVP form inputs', async ({ page }) => {
		await page.goto(`/i/${testShareToken}`);
		await expect(page.locator('body')).toContainText(/E2E Test Event/, { timeout: 10000 });
		expect(await page.locator('input').count()).toBeGreaterThanOrEqual(2);
	});

	test('RSVP submission via API', async () => {
		const data = await submitRSVPViaAPI(testShareToken, {
			name: 'E2E Test Attendee',
			email: `attendee-${RUN_ID}@example.com`,
			rsvpStatus: 'attending'
		});
		testRsvpToken = (data as any).rsvpToken;
		expect(testRsvpToken).toBeTruthy();
	});

	test('RSVP submission via browser', async ({ page }) => {
		await page.goto(`/i/${testShareToken}`);
		await expect(page.locator('body')).toContainText(/E2E Test Event/, { timeout: 10000 });

		const nameInput = page.locator(
			'input[name="name"], input[id="name"], input[placeholder*="name" i]'
		).first();
		await nameInput.waitFor({ state: 'visible', timeout: 5000 });
		await nameInput.fill('Browser Test Attendee');

		const emailInput = page.locator(
			'input[name="email"], input[id="email"], input[type="email"]'
		).first();
		await emailInput.waitFor({ state: 'visible', timeout: 5000 });
		await emailInput.fill(`browser-attendee-${RUN_ID}@example.com`);

		const attendingBtn = page.locator(
			'button:has-text("Attending"), button:has-text("Yes"), button:has-text("Accept")'
		).first();
		if (await attendingBtn.isVisible().catch(() => false)) {
			await attendingBtn.click();
		}

		const submitBtn = page.locator('button[type="submit"]').first();
		await submitBtn.click();

		await expect(
			page.getByText(/thank|confirmed|submitted|success/i).first()
		).toBeVisible({ timeout: 10000 });
	});

	test('invalid share token shows error', async ({ page }) => {
		await page.goto('/i/invalidtoken123');
		await expect(
			page.getByText(/not found|invalid|error|expired/i).first()
		).toBeVisible({ timeout: 10000 });
	});

	// ─── RSVP Management ─────────────────────────────────
	test('RSVP management page loads', async ({ page }) => {
		await page.goto(`/r/${testRsvpToken}`);
		await expect(
			page.locator('body')
		).toContainText(/E2E Test Attendee|attending/i, { timeout: 10000 });
	});

	test('invalid RSVP token shows error', async ({ page }) => {
		await page.goto('/r/invalidtoken123');
		await expect(
			page.getByText(/not found|invalid|error/i).first()
		).toBeVisible({ timeout: 10000 });
	});

	// ─── API Security ────────────────────────────────────
	test('Bearer token bypasses CSRF (by design)', async () => {
		const event = await createEventViaAPI(sessionToken);
		expect((event as any).id).toBeTruthy();
	});

	test('cookie auth without CSRF is rejected (403)', async ({ request }) => {
		const res = await request.post('/api/v1/events', {
			headers: {
				'Content-Type': 'application/json',
				Cookie: `session=${sessionToken}`
			},
			data: {
				title: 'CSRF Test',
				eventDate: '2027-01-01T12:00:00',
				timezone: 'UTC'
			}
		});
		expect(res.status()).toBe(403);
	});

	test('honeypot catches bot submissions', async ({ request }) => {
		const res = await request.post(`/api/v1/rsvp/public/${testShareToken}`, {
			headers: { 'Content-Type': 'application/json' },
			data: {
				name: 'Bot User',
				email: 'bot@example.com',
				rsvpStatus: 'attending',
				website: 'http://spam.com'
			}
		});
		expect(res.status()).toBe(200);

		const listRes = await fetch(`http://localhost:8091/api/v1/rsvp/event/${testEventId}`, {
			headers: { Authorization: `Bearer ${sessionToken}` }
		});
		const attendees = (await listRes.json()).data || [];
		expect(attendees.find((a: any) => a.email === 'bot@example.com')).toBeUndefined();
	});

	// Logout must run before rate limit test (both hit auth rate limiter).
	test('logout invalidates session', async () => {
		const logoutRes = await fetch('http://localhost:8091/api/v1/auth/logout', {
			method: 'POST',
			headers: { Authorization: `Bearer ${sessionToken}` }
		});
		expect(logoutRes.status).toBe(200);

		const evRes = await fetch('http://localhost:8091/api/v1/events', {
			headers: { Authorization: `Bearer ${sessionToken}` }
		});
		expect(evRes.status).toBe(401);
	});

	// Rate limit test runs LAST (exhausts auth rate limit).
	test('rate limiting on auth endpoints', async ({ request }) => {
		const results: number[] = [];
		for (let i = 0; i < 15; i++) {
			const res = await request.post('/api/v1/auth/magic-link', {
				headers: { 'Content-Type': 'application/json' },
				data: { email: `ratelimit-${RUN_ID}-${i}@example.com` }
			});
			results.push(res.status());
		}
		expect(results).toContain(429);
	});
});
