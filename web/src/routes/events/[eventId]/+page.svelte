<script lang="ts">
	import { page } from '$app/stores';
	import { api } from '$lib/api/client';
	import { toast } from '$lib/stores/toast';
	import { currentEvent } from '$lib/stores/events';
	import { formatDateTime } from '$lib/utils/dates';
	import type { Event, Attendee, RSVPStats } from '$lib/types';
	import AppShell from '$lib/components/layout/AppShell.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import Spinner from '$lib/components/ui/Spinner.svelte';
	import { onMount } from 'svelte';

	let copied = $state(false);
	let loading = $state(true);
	let event: Event | null = $state(null);
	let attendees: Attendee[] = $state([]);
	let stats: RSVPStats = $state({ attending: 0, maybe: 0, declined: 0, pending: 0, total: 0 });
	let activeFilter: string = $state('all');

	const eventId = $derived($page.params.eventId);

	let filteredAttendees = $derived.by(() => {
		if (activeFilter === 'all') return attendees;
		return attendees.filter((a) => a.rsvpStatus === activeFilter);
	});

	onMount(async () => {
		try {
			const [eventResult, attendeeResult, statsResult] = await Promise.all([
				api.get<{ data: Event }>(`/events/${eventId}`),
				api.get<{ data: Attendee[] }>(`/rsvp/event/${eventId}`).catch(() => ({ data: [] })),
				api.get<{ data: RSVPStats }>(`/rsvp/event/${eventId}/stats`).catch(() => ({
					data: { attending: 0, maybe: 0, declined: 0, pending: 0, total: 0 }
				}))
			]);
			event = eventResult.data;
			$currentEvent = event;
			attendees = attendeeResult.data;
			stats = statsResult.data;
		} catch (err: unknown) {
			const apiErr = err as { message?: string };
			toast.error(apiErr.message || 'Failed to load event');
		} finally {
			loading = false;
		}
	});

	function statusVariant(status: string): 'success' | 'warning' | 'error' | 'info' | 'neutral' {
		const map: Record<string, 'success' | 'warning' | 'error' | 'info' | 'neutral'> = {
			draft: 'neutral',
			published: 'success',
			cancelled: 'error',
			archived: 'warning',
			attending: 'success',
			maybe: 'warning',
			declined: 'error',
			pending: 'info'
		};
		return map[status] || 'neutral';
	}

	async function publishEvent() {
		if (!event) return;
		try {
			const result = await api.post<{ data: Event }>(`/events/${eventId}/publish`);
			event = result.data;
			$currentEvent = event;
			toast.success('Event published!');
		} catch (err: unknown) {
			const apiErr = err as { message?: string };
			toast.error(apiErr.message || 'Failed to publish event');
		}
	}

	async function copyShareLink() {
		if (!event) return;
		try {
			await navigator.clipboard.writeText(`${$page.url.origin}/i/${event.shareToken}`);
			copied = true;
			toast.success('Link copied!');
			setTimeout(() => (copied = false), 2000);
		} catch {
			toast.error('Failed to copy link');
		}
	}

	async function cancelEvent() {
		if (!event) return;
		try {
			const result = await api.post<{ data: Event }>(`/events/${eventId}/cancel`);
			event = result.data;
			$currentEvent = event;
			toast.success('Event cancelled');
		} catch (err: unknown) {
			const apiErr = err as { message?: string };
			toast.error(apiErr.message || 'Failed to cancel event');
		}
	}
</script>

<svelte:head>
	<title>{event?.title || 'Event Details'} -- OpenRSVP</title>
</svelte:head>

<AppShell>
	{#if loading}
		<div class="flex items-center justify-center py-16">
			<Spinner size="lg" class="text-indigo-500" />
		</div>
	{:else if event}
		<!-- Back link + actions -->
		<div class="mb-6 flex items-center justify-between">
			<a href="/events" class="text-sm text-indigo-600 hover:text-indigo-500">&larr; Back to events</a>
			<div class="flex items-center gap-2">
				<Button variant="outline" size="sm" href="/events/{eventId}/edit">Edit</Button>
				<Button variant="outline" size="sm" href="/events/{eventId}/invite">Design Invite</Button>
				<Button variant="outline" size="sm" href="/events/{eventId}/share">Share</Button>
				<Button variant="outline" size="sm" href="/events/{eventId}/messages">Send Message</Button>
			</div>
		</div>

		<!-- Event info card -->
		<Card class="mb-6">
			<div class="flex items-start justify-between">
				<div>
					<h1 class="text-2xl font-bold text-slate-900">{event.title}</h1>
					<p class="mt-2 text-sm text-slate-600">{formatDateTime(event.eventDate)}</p>
					{#if event.endDate}
						<p class="text-sm text-slate-500">Ends: {formatDateTime(event.endDate)}</p>
					{/if}
					{#if event.location}
						<p class="mt-1 text-sm text-slate-500 flex items-center gap-1">
							<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17.657 16.657L13.414 20.9a1.998 1.998 0 01-2.827 0l-4.244-4.243a8 8 0 1111.314 0z" />
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 11a3 3 0 11-6 0 3 3 0 016 0z" />
							</svg>
							{event.location}
						</p>
					{/if}
					{#if event.description}
						<p class="mt-3 text-sm text-slate-700 whitespace-pre-wrap">{event.description}</p>
					{/if}
				</div>
				<div class="flex flex-col items-end gap-2">
					<Badge variant={statusVariant(event.status)}>{event.status}</Badge>
					{#if event.status === 'draft'}
						<Button size="sm" onclick={publishEvent}>Publish</Button>
					{:else if event.status === 'published'}
						<Button variant="danger" size="sm" onclick={cancelEvent}>Cancel Event</Button>
					{/if}
				</div>
			</div>
			{#if event.shareToken}
				<div class="mt-4 pt-4 border-t border-slate-200 flex items-center gap-2">
					<p class="text-xs text-slate-500">
						Share link: <code class="bg-slate-100 px-1.5 py-0.5 rounded text-indigo-600">{$page.url.origin}/i/{event.shareToken}</code>
					</p>
					<button
						type="button"
						onclick={copyShareLink}
						class="inline-flex items-center justify-center rounded p-1 text-slate-400 hover:text-indigo-600 hover:bg-slate-100 transition-colors"
						title="Copy share link"
					>
						{#if copied}
							<svg class="h-4 w-4 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
								<path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
							</svg>
						{:else}
							<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
								<path stroke-linecap="round" stroke-linejoin="round" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
							</svg>
						{/if}
					</button>
				</div>
			{/if}
		</Card>

		<!-- RSVP Stats -->
		<div class="grid grid-cols-2 sm:grid-cols-5 gap-4 mb-6">
			{#each [
				{ label: 'Attending', value: stats.attending, color: 'text-green-600 bg-green-50' },
				{ label: 'Maybe', value: stats.maybe, color: 'text-yellow-600 bg-yellow-50' },
				{ label: 'Declined', value: stats.declined, color: 'text-red-600 bg-red-50' },
				{ label: 'Pending', value: stats.pending, color: 'text-blue-600 bg-blue-50' },
				{ label: 'Total', value: stats.total, color: 'text-slate-700 bg-slate-50' }
			] as stat}
				<div class="rounded-xl border border-slate-200 p-4 {stat.color}">
					<p class="text-2xl font-bold">{stat.value}</p>
					<p class="text-xs font-medium mt-1">{stat.label}</p>
				</div>
			{/each}
		</div>

		<!-- Attendee list -->
		<Card>
			{#snippet header()}
				<div class="flex items-center justify-between">
					<h2 class="text-lg font-semibold text-slate-900">Attendees</h2>
					<div class="flex gap-1">
						{#each ['all', 'attending', 'maybe', 'declined'] as filter}
							<button
								type="button"
								class="px-3 py-1 rounded-full text-xs font-medium transition-colors {activeFilter === filter
									? 'bg-indigo-600 text-white'
									: 'bg-slate-100 text-slate-600 hover:bg-slate-200'}"
								onclick={() => (activeFilter = filter)}
							>
								{filter.charAt(0).toUpperCase() + filter.slice(1)}
							</button>
						{/each}
					</div>
				</div>
			{/snippet}

			{#if filteredAttendees.length === 0}
				<p class="text-sm text-slate-500 text-center py-8">
					{attendees.length === 0 ? 'No attendees yet. Share your event to start receiving RSVPs.' : 'No attendees match this filter.'}
				</p>
			{:else}
				<div class="divide-y divide-slate-200 -mx-6 -mb-4">
					{#each filteredAttendees as attendee (attendee.id)}
						<div class="px-6 py-3 flex items-center justify-between">
							<div class="flex-1 min-w-0">
								<p class="text-sm font-medium text-slate-900">{attendee.name}</p>
								<p class="text-xs text-slate-500">
									{attendee.email || attendee.phone || 'No contact info'}
								</p>
							</div>
							<div class="flex items-center gap-3 ml-4">
								{#if attendee.dietaryNotes}
									<span class="text-xs text-slate-500" title="Dietary notes">{attendee.dietaryNotes}</span>
								{/if}
								{#if attendee.plusOnes > 0}
									<span class="text-xs text-slate-500">+{attendee.plusOnes}</span>
								{/if}
								<Badge variant={statusVariant(attendee.rsvpStatus)}>
									{attendee.rsvpStatus}
								</Badge>
							</div>
						</div>
					{/each}
				</div>
			{/if}
		</Card>
	{:else}
		<Card>
			<p class="text-center text-slate-500 py-8">Event not found.</p>
		</Card>
	{/if}
</AppShell>
