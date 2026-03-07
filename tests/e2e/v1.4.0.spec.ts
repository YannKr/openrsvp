import { test, expect } from '@playwright/test';
import {
	createEventViaAPI,
	getOrCreateSession,
	setSessionInBrowser,
	submitRSVPViaAPI
} from './helpers';

const BASE = 'http://localhost:8091';
const RUN_ID = Date.now();
const ORGANIZER_EMAIL = `e2e-v14-organizer-${RUN_ID}@example.com`;

let sessionToken: string;
let testEventId: string;
let testShareToken: string;
let testRsvpToken: string;

test.describe.serial('v1.4.0 Features', () => {
	test.beforeAll(async () => {
		sessionToken = await getOrCreateSession(ORGANIZER_EMAIL);
		const event = await createEventViaAPI(sessionToken);
		testEventId = (event as any).id;
		testShareToken = (event as any).shareToken;

		// Submit an RSVP so we have an attendee for comment tests.
		const rsvp = await submitRSVPViaAPI(testShareToken, {
			name: 'V14 Attendee',
			email: `v14-attendee-${RUN_ID}@example.com`,
			rsvpStatus: 'attending'
		});
		testRsvpToken = (rsvp as any).rsvpToken;
	});

	// ─── Comments / Guestbook ────────────────────────────
	test.describe('Comments / Guestbook', () => {
		let commentId: string;

		test('list public comments on empty event returns empty', async () => {
			const res = await fetch(`${BASE}/api/v1/comments/public/${testShareToken}`);
			expect(res.status).toBe(200);
			const json = await res.json();
			expect(json.data.comments).toHaveLength(0);
		});

		test('post comment requires X-RSVP-Token', async () => {
			const res = await fetch(`${BASE}/api/v1/comments/public/${testShareToken}`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ body: 'Hello!' })
			});
			expect(res.status).toBe(401);
		});

		test('post comment with valid RSVP token', async () => {
			const res = await fetch(`${BASE}/api/v1/comments/public/${testShareToken}`, {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
					'X-RSVP-Token': testRsvpToken
				},
				body: JSON.stringify({ body: 'Great event, looking forward to it!' })
			});
			expect(res.status).toBe(201);
			const json = await res.json();
			expect(json.data.body).toBe('Great event, looking forward to it!');
			expect(json.data.authorName).toBe('V14 Attendee');
			commentId = json.data.id;
		});

		test('list public comments returns posted comment', async () => {
			const res = await fetch(`${BASE}/api/v1/comments/public/${testShareToken}`);
			expect(res.status).toBe(200);
			const json = await res.json();
			expect(json.data.comments.length).toBeGreaterThanOrEqual(1);
			expect(json.data.comments.find((c: any) => c.id === commentId)).toBeTruthy();
		});

		test('organizer can list all comments', async () => {
			const res = await fetch(`${BASE}/api/v1/comments/event/${testEventId}`, {
				headers: { Authorization: `Bearer ${sessionToken}` }
			});
			expect(res.status).toBe(200);
			const json = await res.json();
			expect(json.data.length).toBeGreaterThanOrEqual(1);
		});

		test('post comment with empty body is rejected', async () => {
			const res = await fetch(`${BASE}/api/v1/comments/public/${testShareToken}`, {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
					'X-RSVP-Token': testRsvpToken
				},
				body: JSON.stringify({ body: '' })
			});
			expect(res.status).toBe(400);
		});

		test('post comment with invalid share token returns 404', async () => {
			const res = await fetch(`${BASE}/api/v1/comments/public/invalidtoken`, {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
					'X-RSVP-Token': testRsvpToken
				},
				body: JSON.stringify({ body: 'Should fail' })
			});
			expect(res.status).toBe(404);
		});

		test('delete own comment', async () => {
			const res = await fetch(`${BASE}/api/v1/comments/public/${commentId}`, {
				method: 'DELETE',
				headers: { 'X-RSVP-Token': testRsvpToken }
			});
			expect(res.status).toBe(200);
		});

		test('post another comment for organizer delete test', async () => {
			const res = await fetch(`${BASE}/api/v1/comments/public/${testShareToken}`, {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
					'X-RSVP-Token': testRsvpToken
				},
				body: JSON.stringify({ body: 'Comment for organizer to delete' })
			});
			expect(res.status).toBe(201);
			commentId = (await res.json()).data.id;
		});

		test('organizer can delete any comment', async () => {
			const res = await fetch(
				`${BASE}/api/v1/comments/event/${testEventId}/${commentId}`,
				{
					method: 'DELETE',
					headers: { Authorization: `Bearer ${sessionToken}` }
				}
			);
			expect(res.status).toBe(200);
		});

		test('guestbook section visible on public invite page', async ({ page }) => {
			// Post a comment first so the section has content.
			await fetch(`${BASE}/api/v1/comments/public/${testShareToken}`, {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
					'X-RSVP-Token': testRsvpToken
				},
				body: JSON.stringify({ body: 'E2E guestbook comment' })
			});

			await page.goto(`/i/${testShareToken}`);
			await expect(page.locator('body')).toContainText(/guestbook|comment/i, {
				timeout: 10000
			});
		});
	});

	// ─── Webhooks ────────────────────────────────────────
	test.describe('Webhooks', () => {
		let webhookId: string;

		test('create webhook', async () => {
			const res = await fetch(`${BASE}/api/v1/webhooks/event/${testEventId}`, {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
					Authorization: `Bearer ${sessionToken}`
				},
				body: JSON.stringify({
					url: 'https://httpbin.org/post',
					eventTypes: ['rsvp.created'],
					description: 'E2E test webhook'
				})
			});
			expect(res.status).toBe(201);
			const json = await res.json();
			expect(json.data.url).toBe('https://httpbin.org/post');
			expect(json.data.secret).toContain('whsec_');
			webhookId = json.data.id;
		});

		test('list webhooks returns created webhook', async () => {
			const res = await fetch(`${BASE}/api/v1/webhooks/event/${testEventId}`, {
				headers: { Authorization: `Bearer ${sessionToken}` }
			});
			expect(res.status).toBe(200);
			const json = await res.json();
			expect(json.data.length).toBeGreaterThanOrEqual(1);
			expect(json.data.find((w: any) => w.id === webhookId)).toBeTruthy();
		});

		test('get webhook by ID', async () => {
			const res = await fetch(`${BASE}/api/v1/webhooks/${webhookId}`, {
				headers: { Authorization: `Bearer ${sessionToken}` }
			});
			expect(res.status).toBe(200);
			const json = await res.json();
			expect(json.data.id).toBe(webhookId);
		});

		test('update webhook', async () => {
			const res = await fetch(`${BASE}/api/v1/webhooks/${webhookId}`, {
				method: 'PUT',
				headers: {
					'Content-Type': 'application/json',
					Authorization: `Bearer ${sessionToken}`
				},
				body: JSON.stringify({
					description: 'Updated E2E webhook',
					eventTypes: ['rsvp.created', 'event.published']
				})
			});
			expect(res.status).toBe(200);
			const json = await res.json();
			expect(json.data.description).toBe('Updated E2E webhook');
		});

		test('rotate webhook secret', async () => {
			// Get current secret.
			const before = await fetch(`${BASE}/api/v1/webhooks/${webhookId}`, {
				headers: { Authorization: `Bearer ${sessionToken}` }
			});
			const oldSecret = (await before.json()).data.secret;

			const res = await fetch(`${BASE}/api/v1/webhooks/${webhookId}/rotate-secret`, {
				method: 'POST',
				headers: { Authorization: `Bearer ${sessionToken}` }
			});
			expect(res.status).toBe(200);
			const json = await res.json();
			expect(json.data.secret).not.toBe(oldSecret);
			expect(json.data.secret).toContain('whsec_');
		});

		test('get deliveries (initially empty)', async () => {
			const res = await fetch(`${BASE}/api/v1/webhooks/${webhookId}/deliveries`, {
				headers: { Authorization: `Bearer ${sessionToken}` }
			});
			expect(res.status).toBe(200);
		});

		test('send test webhook', async () => {
			const res = await fetch(`${BASE}/api/v1/webhooks/${webhookId}/test`, {
				method: 'POST',
				headers: { Authorization: `Bearer ${sessionToken}` }
			});
			expect(res.status).toBe(200);
			const json = await res.json();
			expect(json.data.eventType).toBe('test');
		});

		test('webhook requires auth', async () => {
			const res = await fetch(`${BASE}/api/v1/webhooks/event/${testEventId}`);
			expect(res.status).toBe(401);
		});

		test('create webhook with invalid URL is rejected', async () => {
			const res = await fetch(`${BASE}/api/v1/webhooks/event/${testEventId}`, {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
					Authorization: `Bearer ${sessionToken}`
				},
				body: JSON.stringify({
					url: 'not-a-url',
					eventTypes: ['rsvp.created']
				})
			});
			expect(res.status).toBe(400);
		});

		test('create webhook with invalid event type is rejected', async () => {
			const res = await fetch(`${BASE}/api/v1/webhooks/event/${testEventId}`, {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
					Authorization: `Bearer ${sessionToken}`
				},
				body: JSON.stringify({
					url: 'https://httpbin.org/post',
					eventTypes: ['invalid.type']
				})
			});
			expect(res.status).toBe(400);
		});

		test('delete webhook', async () => {
			const res = await fetch(`${BASE}/api/v1/webhooks/${webhookId}`, {
				method: 'DELETE',
				headers: { Authorization: `Bearer ${sessionToken}` }
			});
			expect(res.status).toBe(200);
		});

		test('webhook management page loads', async ({ page }) => {
			await setSessionInBrowser(page, sessionToken);
			await page.goto(`/events/${testEventId}/webhooks`);
			await expect(page.locator('body')).toContainText(/webhook/i, { timeout: 10000 });
		});
	});

	// ─── CSV Import ──────────────────────────────────────
	test.describe('CSV Import', () => {
		test('download CSV template', async () => {
			const res = await fetch(`${BASE}/api/v1/rsvp/import/template`, {
				headers: { Authorization: `Bearer ${sessionToken}` }
			});
			expect(res.status).toBe(200);
			expect(res.headers.get('content-type')).toContain('text/csv');
			const text = await res.text();
			expect(text).toContain('Name');
			expect(text).toContain('Email');
		});

		test('preview CSV import', async () => {
			const csvContent = 'Name,Email\nAlice Smith,alice@example.com\nBob Jones,bob@example.com\n';
			const formData = new FormData();
			formData.append('file', new Blob([csvContent], { type: 'text/csv' }), 'guests.csv');

			const res = await fetch(
				`${BASE}/api/v1/rsvp/event/${testEventId}/import/preview`,
				{
					method: 'POST',
					headers: { Authorization: `Bearer ${sessionToken}` },
					body: formData
				}
			);
			expect(res.status).toBe(200);
			const json = await res.json();
			expect(json.data.totalRows).toBe(2);
			expect(json.data.validRows).toBe(2);
			expect(json.data.errorRows).toBe(0);
		});

		test('execute CSV import', async () => {
			const res = await fetch(`${BASE}/api/v1/rsvp/event/${testEventId}/import`, {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
					Authorization: `Bearer ${sessionToken}`
				},
				body: JSON.stringify({
					rows: [
						{ name: 'Import Alice', email: `import-alice-${RUN_ID}@example.com`, phone: '', dietaryNotes: 'Vegan', plusOnes: 0 },
						{ name: 'Import Bob', email: `import-bob-${RUN_ID}@example.com`, phone: '', dietaryNotes: '', plusOnes: 1 }
					],
					sendInvitations: false
				})
			});
			expect(res.status).toBe(200);
			const json = await res.json();
			expect(json.data.imported).toBe(2);
			expect(json.data.failed).toBe(0);
		});

		test('duplicate detection on re-import', async () => {
			const csvContent = `Name,Email\nImport Alice,import-alice-${RUN_ID}@example.com\nNew Guest,new-${RUN_ID}@example.com\n`;
			const formData = new FormData();
			formData.append('file', new Blob([csvContent], { type: 'text/csv' }), 'guests.csv');

			const res = await fetch(
				`${BASE}/api/v1/rsvp/event/${testEventId}/import/preview`,
				{
					method: 'POST',
					headers: { Authorization: `Bearer ${sessionToken}` },
					body: formData
				}
			);
			expect(res.status).toBe(200);
			const json = await res.json();
			expect(json.data.duplicates).toBe(1);
			expect(json.data.validRows).toBe(1);
		});

		test('import with no Name column is rejected', async () => {
			const csvContent = 'Email\nalice@example.com\n';
			const formData = new FormData();
			formData.append('file', new Blob([csvContent], { type: 'text/csv' }), 'guests.csv');

			const res = await fetch(
				`${BASE}/api/v1/rsvp/event/${testEventId}/import/preview`,
				{
					method: 'POST',
					headers: { Authorization: `Bearer ${sessionToken}` },
					body: formData
				}
			);
			expect(res.status).toBe(400);
		});

		test('import requires authentication', async () => {
			const res = await fetch(`${BASE}/api/v1/rsvp/import/template`);
			expect(res.status).toBe(401);
		});

		test('import page loads in browser', async ({ page }) => {
			await setSessionInBrowser(page, sessionToken);
			await page.goto(`/events/${testEventId}/import`);
			await expect(page.locator('body')).toContainText(/import|csv|guest/i, {
				timeout: 10000
			});
		});
	});

	// ─── Email Tracking / Notifications ──────────────────
	test.describe('Email Tracking', () => {
		test('tracking pixel returns GIF', async () => {
			// Use a non-existent log ID — should still return the pixel.
			const res = await fetch(`${BASE}/api/v1/notifications/track/open/nonexistent`);
			expect(res.status).toBe(200);
			expect(res.headers.get('content-type')).toBe('image/gif');
			expect(res.headers.get('cache-control')).toContain('no-store');
		});

		test('email stats endpoint requires auth', async () => {
			const res = await fetch(
				`${BASE}/api/v1/notifications/event/${testEventId}/stats`
			);
			expect(res.status).toBe(401);
		});

		test('email stats returns data for event', async () => {
			const res = await fetch(
				`${BASE}/api/v1/notifications/event/${testEventId}/stats`,
				{ headers: { Authorization: `Bearer ${sessionToken}` } }
			);
			expect(res.status).toBe(200);
			const json = await res.json();
			expect(json.data).toBeDefined();
		});

		test('notification log returns entries', async () => {
			const res = await fetch(
				`${BASE}/api/v1/notifications/event/${testEventId}`,
				{ headers: { Authorization: `Bearer ${sessionToken}` } }
			);
			expect(res.status).toBe(200);
			const json = await res.json();
			expect(Array.isArray(json.data)).toBe(true);
		});

		test('email stats for wrong event returns 404', async () => {
			const res = await fetch(
				`${BASE}/api/v1/notifications/event/nonexistent-event-id/stats`,
				{ headers: { Authorization: `Bearer ${sessionToken}` } }
			);
			expect(res.status).toBe(404);
		});
	});

	// ─── Cross-Feature Integration ───────────────────────
	test.describe('Integration', () => {
		test('imported attendees appear in RSVP list', async () => {
			const res = await fetch(`${BASE}/api/v1/rsvp/event/${testEventId}`, {
				headers: { Authorization: `Bearer ${sessionToken}` }
			});
			expect(res.status).toBe(200);
			const json = await res.json();
			const imported = json.data.filter(
				(a: any) => a.importSource === 'csv'
			);
			expect(imported.length).toBeGreaterThanOrEqual(2);
		});

		test('webhook fires on new RSVP', async () => {
			// Create a webhook first.
			const whRes = await fetch(`${BASE}/api/v1/webhooks/event/${testEventId}`, {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
					Authorization: `Bearer ${sessionToken}`
				},
				body: JSON.stringify({
					url: 'https://httpbin.org/post',
					eventTypes: ['rsvp.created']
				})
			});
			expect(whRes.status).toBe(201);
			const wh = (await whRes.json()).data;

			// Submit RSVP to trigger webhook.
			await submitRSVPViaAPI(testShareToken, {
				name: 'Webhook Trigger Guest',
				email: `webhook-trigger-${RUN_ID}@example.com`,
				rsvpStatus: 'attending'
			});

			// Wait briefly for async dispatch.
			await new Promise((r) => setTimeout(r, 3000));

			// Check deliveries.
			const delRes = await fetch(`${BASE}/api/v1/webhooks/${wh.id}/deliveries`, {
				headers: { Authorization: `Bearer ${sessionToken}` }
			});
			expect(delRes.status).toBe(200);
			const deliveries = (await delRes.json()).data;
			expect(deliveries.length).toBeGreaterThanOrEqual(1);
			expect(deliveries[0].eventType).toBe('rsvp.created');

			// Cleanup.
			await fetch(`${BASE}/api/v1/webhooks/${wh.id}`, {
				method: 'DELETE',
				headers: { Authorization: `Bearer ${sessionToken}` }
			});
		});

		test('commentsEnabled toggle is respected', async () => {
			// Disable comments via event update.
			await fetch(`${BASE}/api/v1/events/${testEventId}`, {
				method: 'PUT',
				headers: {
					'Content-Type': 'application/json',
					Authorization: `Bearer ${sessionToken}`
				},
				body: JSON.stringify({ commentsEnabled: false })
			});

			// Try posting a comment — should be rejected.
			const res = await fetch(`${BASE}/api/v1/comments/public/${testShareToken}`, {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
					'X-RSVP-Token': testRsvpToken
				},
				body: JSON.stringify({ body: 'Should be rejected' })
			});
			expect(res.status).toBe(400);
			const json = await res.json();
			expect(json.message).toContain('disabled');

			// Re-enable comments.
			await fetch(`${BASE}/api/v1/events/${testEventId}`, {
				method: 'PUT',
				headers: {
					'Content-Type': 'application/json',
					Authorization: `Bearer ${sessionToken}`
				},
				body: JSON.stringify({ commentsEnabled: true })
			});
		});
	});
});
