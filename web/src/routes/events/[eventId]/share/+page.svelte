<script lang="ts">
	import { page } from '$app/stores';
	import { api } from '$lib/api/client';
	import { toast } from '$lib/stores/toast';
	import type { Event, RSVPStats } from '$lib/types';
	import AppShell from '$lib/components/layout/AppShell.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import Spinner from '$lib/components/ui/Spinner.svelte';
	import { onMount } from 'svelte';
	import QRCode from 'qrcode';

	const eventId = $derived($page.params.eventId);

	let loading = $state(true);
	let event: Event | null = $state(null);
	let stats: RSVPStats = $state({ attending: 0, attendingHeadcount: 0, maybe: 0, maybeHeadcount: 0, declined: 0, pending: 0, waitlisted: 0, total: 0, totalHeadcount: 0 });
	let copied = $state(false);
	let qrDataUrl = $state('');
	let shareUrl = $state('');

	onMount(async () => {
		try {
			const [eventResult, statsResult] = await Promise.all([
				api.get<{ data: Event }>(`/events/${eventId}`),
				api.get<{ data: RSVPStats }>(`/rsvp/event/${eventId}/stats`).catch(() => ({
					data: { attending: 0, attendingHeadcount: 0, maybe: 0, maybeHeadcount: 0, declined: 0, pending: 0, waitlisted: 0, total: 0, totalHeadcount: 0 }
				}))
			]);
			event = eventResult.data;
			stats = statsResult.data;
			shareUrl = event ? `${window.location.origin}/i/${event.shareToken}` : '';
		if (eventResult.data) {
				const url = `${window.location.origin}/i/${eventResult.data.shareToken}`;
				try {
					qrDataUrl = await QRCode.toDataURL(url, {
						width: 256,
						margin: 2,
						color: { dark: '#1e293b', light: '#ffffff' }
					});
				} catch {
					// QR generation failed silently
				}
			}
		} catch (err: unknown) {
			const apiErr = err as { message?: string };
			toast.error(apiErr.message || 'Failed to load event');
		} finally {
			loading = false;
		}
	});

	async function copyLink() {
		try {
			await navigator.clipboard.writeText(shareUrl);
			copied = true;
			toast.success('Link copied to clipboard!');
			setTimeout(() => (copied = false), 2000);
		} catch {
			toast.error('Failed to copy link');
		}
	}
</script>

<svelte:head>
	<title>Share Event -- OpenRSVP</title>
</svelte:head>

<AppShell>
	<div class="max-w-3xl mx-auto">
		<div class="mb-6">
			<a href="/events/{eventId}" class="text-sm text-indigo-600 hover:text-indigo-500">&larr; Back to event</a>
			<h1 class="mt-2 text-2xl font-bold text-slate-900">Share Event</h1>
			{#if event}
				<p class="text-sm text-slate-500">{event.title}</p>
			{/if}
		</div>

		{#if loading}
			<div class="flex items-center justify-center py-16">
				<Spinner size="lg" class="text-indigo-500" />
			</div>
		{:else if event}
			<!-- Share link -->
			<Card class="mb-6">
				{#snippet header()}
					<h2 class="text-lg font-semibold text-slate-900">Share Link</h2>
				{/snippet}

				<p class="text-sm text-slate-600 mb-4">
					Share this link with your guests so they can view the invitation and RSVP.
				</p>

				<div class="flex items-center gap-2">
					<input
						type="text"
						readonly
						value={shareUrl}
						class="flex-1 block rounded-lg border border-slate-300 bg-slate-50 px-3 py-2 text-sm text-slate-700 font-mono"
					/>
					<Button onclick={copyLink} variant={copied ? 'secondary' : 'primary'} size="md">
						{copied ? 'Copied!' : 'Copy Link'}
					</Button>
				</div>
			</Card>

			<!-- QR Code -->
			<Card class="mb-6">
				{#snippet header()}
					<h2 class="text-lg font-semibold text-slate-900">QR Code</h2>
				{/snippet}

				<div class="flex flex-col items-center py-6">
					{#if qrDataUrl}
						<img src={qrDataUrl} alt="QR Code for event invitation" class="w-48 h-48 rounded-xl" />
					{:else}
						<div class="w-48 h-48 bg-slate-100 border-2 border-dashed border-slate-300 rounded-xl flex items-center justify-center">
							<div class="animate-spin rounded-full h-6 w-6 border-b-2 border-slate-400"></div>
						</div>
					{/if}
					<p class="mt-4 text-sm text-slate-500 text-center max-w-sm">
						Point a phone camera at the QR code to open the invitation link directly.
					</p>
					<p class="mt-1 text-xs text-slate-400 font-mono break-all text-center">{shareUrl}</p>
				</div>
			</Card>

			<!-- Attendee summary -->
			<Card>
				{#snippet header()}
					<h2 class="text-lg font-semibold text-slate-900">Response Summary</h2>
				{/snippet}

				<div class="grid grid-cols-2 sm:grid-cols-4 gap-4">
					<div class="text-center">
						<p class="text-2xl font-bold text-green-600">{stats.attending}</p>
						<p class="text-xs text-slate-500">Attending</p>
					</div>
					<div class="text-center">
						<p class="text-2xl font-bold text-yellow-600">{stats.maybe}</p>
						<p class="text-xs text-slate-500">Maybe</p>
					</div>
					<div class="text-center">
						<p class="text-2xl font-bold text-red-600">{stats.declined}</p>
						<p class="text-xs text-slate-500">Declined</p>
					</div>
					<div class="text-center">
						<p class="text-2xl font-bold text-blue-600">{stats.pending}</p>
						<p class="text-xs text-slate-500">Pending</p>
					</div>
				</div>
				<div class="mt-4 pt-4 border-t border-slate-200 text-center">
					<p class="text-sm text-slate-600">
						<span class="font-semibold">{stats.total}</span> total invitees
					</p>
				</div>
			</Card>
		{/if}
	</div>
</AppShell>
