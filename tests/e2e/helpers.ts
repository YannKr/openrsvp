import { execSync } from 'child_process';
import type { Page } from '@playwright/test';

const BASE = 'http://localhost:8091';

// Cache sessions to avoid hitting rate limits.
const sessionCache = new Map<string, string>();

/** Request a magic link and extract the token from Docker logs. */
export async function getAuthToken(email: string): Promise<string> {
	const res = await fetch(`${BASE}/api/v1/auth/magic-link`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ email })
	});
	if (!res.ok) throw new Error(`Magic link request failed: ${res.status}`);

	await new Promise((r) => setTimeout(r, 500));

	const logs = execSync(
		'docker compose logs --tail=50 openrsvp 2>&1',
		{ cwd: '/Users/ypk/git/openrsvp', encoding: 'utf-8' }
	);

	const lines = logs.split('\n');
	for (const line of lines.reverse()) {
		if (line.includes('magic link generated') && line.includes(email)) {
			const tokenMatch = line.match(/token=([a-f0-9]{64})/);
			if (tokenMatch) return tokenMatch[1];
		}
	}

	throw new Error(`Could not find magic link token in Docker logs for ${email}`);
}

/** Verify a magic link token and return the session token. */
export async function verifyMagicLink(token: string): Promise<string> {
	const res = await fetch(`${BASE}/api/v1/auth/verify`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ token })
	});
	if (!res.ok) throw new Error(`Verify failed: ${res.status}`);
	const data = await res.json();
	return data.token;
}

/** Get or create a cached session for the given email. */
export async function getOrCreateSession(email: string): Promise<string> {
	if (sessionCache.has(email)) return sessionCache.get(email)!;
	const magicToken = await getAuthToken(email);
	const sessionToken = await verifyMagicLink(magicToken);
	sessionCache.set(email, sessionToken);
	return sessionToken;
}

/** Set an existing session token in the browser context. */
export async function setSessionInBrowser(
	page: Page,
	sessionToken: string
): Promise<void> {
	// Set cookie so server-side auth works.
	await page.context().addCookies([
		{
			name: 'session',
			value: sessionToken,
			domain: 'localhost',
			path: '/',
			httpOnly: true,
			sameSite: 'Lax'
		}
	]);
	// Use addInitScript so localStorage is set BEFORE any page JS runs.
	// This ensures the SPA reads the token on first load.
	await page.addInitScript((token: string) => {
		localStorage.setItem('openrsvp_session', token);
	}, sessionToken);
}

/** Clear session from browser. */
export async function clearSession(page: Page): Promise<void> {
	await page.evaluate(() => localStorage.removeItem('openrsvp_session'));
	await page.context().clearCookies();
}

/** Create an event via API using Bearer token (bypasses CSRF). */
export async function createEventViaAPI(
	sessionToken: string,
	overrides: Record<string, unknown> = {}
): Promise<Record<string, unknown>> {
	const tomorrow = new Date();
	tomorrow.setDate(tomorrow.getDate() + 1);
	const dateStr = tomorrow.toISOString().split('T')[0];

	const eventData = {
		title: 'E2E Test Event ' + Date.now(),
		eventDate: `${dateStr}T18:00:00`,
		timezone: 'America/New_York',
		location: 'Test Venue, 123 Main St',
		description: 'This is an automated e2e test event.',
		maxCapacity: 50,
		rsvpDeadline: `${dateStr}T18:00:00`,
		commentsEnabled: true,
		...overrides
	};

	const res = await fetch(`${BASE}/api/v1/events`, {
		method: 'POST',
		headers: {
			'Content-Type': 'application/json',
			Authorization: `Bearer ${sessionToken}`
		},
		body: JSON.stringify(eventData)
	});
	if (!res.ok) {
		const body = await res.text();
		throw new Error(`Create event failed: ${res.status} ${body}`);
	}
	const json = await res.json();
	const event = json.data || json;

	// Publish the event so it's publicly accessible.
	const pubRes = await fetch(`${BASE}/api/v1/events/${event.id}/publish`, {
		method: 'POST',
		headers: {
			Authorization: `Bearer ${sessionToken}`
		}
	});
	if (!pubRes.ok) {
		const body = await pubRes.text();
		throw new Error(`Publish event failed: ${pubRes.status} ${body}`);
	}
	const pubJson = await pubRes.json();
	return pubJson.data || pubJson;
}

/** Submit an RSVP via API and return the response. */
export async function submitRSVPViaAPI(
	shareToken: string,
	data: { name: string; email: string; rsvpStatus: string }
): Promise<Record<string, unknown>> {
	const res = await fetch(`${BASE}/api/v1/rsvp/public/${shareToken}`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(data)
	});
	if (!res.ok) {
		const body = await res.text();
		throw new Error(`RSVP submission failed: ${res.status} ${body}`);
	}
	const json = await res.json();
	return json.data || json;
}
