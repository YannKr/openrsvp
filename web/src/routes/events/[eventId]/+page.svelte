<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { api } from '$lib/api/client';
	import { toast } from '$lib/stores/toast';
	import { currentEvent } from '$lib/stores/events';
	import { formatDateTime, toISOLocal } from '$lib/utils/dates';
	import { currentUser } from '$lib/stores/auth';
	import type { Event, Attendee, RSVPStats, Reminder, CoHost, EventComment, EmailStats } from '$lib/types';
	import AppShell from '$lib/components/layout/AppShell.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Modal from '$lib/components/ui/Modal.svelte';
	import Spinner from '$lib/components/ui/Spinner.svelte';
	import DateTimePicker from '$lib/components/ui/DateTimePicker.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import Textarea from '$lib/components/ui/Textarea.svelte';
	import { onMount } from 'svelte';

	let showCancelModal = $state(false);
	let notifyOnCancel = $state(true);
	let removeAttendeeTarget: Attendee | null = $state(null);
	let showRemoveAttendeeModal = $state(false);
	let cancelReminderTarget: Reminder | null = $state(null);
	let showCancelReminderModal = $state(false);
	let copied = $state(false);
	let loading = $state(true);
	let event: Event | null = $state(null);
	let attendees: Attendee[] = $state([]);
	let reminders: Reminder[] = $state([]);
	let stats: RSVPStats = $state({ attending: 0, attendingHeadcount: 0, maybe: 0, maybeHeadcount: 0, declined: 0, pending: 0, waitlisted: 0, total: 0, totalHeadcount: 0 });
	let activeFilter: string = $state('all');
	let creatingReminder = $state(false);
	let reminderRemindAt = $state(toISOLocal(new Date(Date.now() + 60 * 60 * 1000)));
	let reminderTargetGroup: Reminder['targetGroup'] = $state('all');
	let reminderMessage = $state('');
	let reminderErrors: Record<string, string> = $state({});
	let cohosts: CoHost[] = $state([]);
	let cohostEmail = $state('');
	let addingCohost = $state(false);
	let eventComments: EventComment[] = $state([]);
	let emailStats = $state<EmailStats | null>(null);

	const eventId = $derived($page.params.eventId);
	const currentUserId = $derived($currentUser?.id);
	const reminderMinDate = $derived(toISOLocal(new Date()));

	const reminderTargetOptions = [
		{ value: 'all', label: 'All Attendees' },
		{ value: 'attending', label: 'Attending' },
		{ value: 'maybe', label: 'Maybe' },
		{ value: 'declined', label: 'Declined' },
		{ value: 'pending', label: 'Pending RSVP' }
	];

	let filteredAttendees = $derived.by(() => {
		if (activeFilter === 'all') return attendees;
		return attendees.filter((a) => a.rsvpStatus === activeFilter);
	});

	onMount(() => {
		document.addEventListener('click', handleExportClickOutside);

		(async () => {
			try {
				const [eventResult, attendeeResult, statsResult, remindersResult, cohostsResult, commentsResult, emailStatsResult] = await Promise.all([
					api.get<{ data: Event }>(`/events/${eventId}`),
					api.get<{ data: Attendee[] }>(`/rsvp/event/${eventId}`).catch(() => ({ data: [] })),
					api.get<{ data: RSVPStats }>(`/rsvp/event/${eventId}/stats`).catch(() => ({
						data: { attending: 0, attendingHeadcount: 0, maybe: 0, maybeHeadcount: 0, declined: 0, pending: 0, waitlisted: 0, total: 0, totalHeadcount: 0 }
					})),
					api.get<{ data: Reminder[] }>(`/reminders/event/${eventId}`).catch(() => ({ data: [] })),
					api.get<{ data: CoHost[] }>(`/events/${eventId}/cohosts`).catch(() => ({ data: [] })),
					api.get<{ data: EventComment[] }>(`/comments/event/${eventId}`).catch(() => ({ data: [] })),
					api.get<{ data: EmailStats }>(`/notifications/event/${eventId}/stats`).catch(() => ({ data: null }))
				]);
				event = eventResult.data;
				$currentEvent = event;
				attendees = attendeeResult.data;
				stats = statsResult.data;
				reminders = remindersResult.data;
				cohosts = cohostsResult.data;
				eventComments = commentsResult.data;
				emailStats = emailStatsResult.data;
			} catch (err: unknown) {
				const apiErr = err as { message?: string };
				toast.error(apiErr.message || 'Failed to load event');
			} finally {
				loading = false;
			}
		})();

		return () => document.removeEventListener('click', handleExportClickOutside);
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
			pending: 'info',
			waitlisted: 'info'
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
			const result = await api.post<{ data: Event }>(`/events/${eventId}/cancel`, {
				notifyAttendees: notifyOnCancel
			});
			event = result.data;
			$currentEvent = event;
			showCancelModal = false;
			toast.success('Event cancelled');
		} catch (err: unknown) {
			const apiErr = err as { message?: string };
			toast.error(apiErr.message || 'Failed to cancel event');
		}
	}

	async function reopenEvent() {
		if (!event) return;
		try {
			const result = await api.post<{ data: Event }>(`/events/${eventId}/reopen`);
			event = result.data;
			$currentEvent = event;
			toast.success('Event re-opened as draft');
		} catch (err: unknown) {
			const apiErr = err as { message?: string };
			toast.error(apiErr.message || 'Failed to re-open event');
		}
	}

	async function duplicateEvent() {
		if (!event) return;
		try {
			const result = await api.post<{ data: Event }>(`/events/${eventId}/duplicate`);
			toast.success('Event duplicated');
			goto(`/events/${result.data.id}`);
		} catch (err: unknown) {
			const apiErr = err as { message?: string };
			toast.error(apiErr.message || 'Failed to duplicate event');
		}
	}

	async function createReminder() {
		if (!event) return;

		reminderErrors = {};
		if (!reminderRemindAt) {
			reminderErrors.remindAt = 'Reminder date is required';
		}

		if (Object.keys(reminderErrors).length > 0) {
			return;
		}

		creatingReminder = true;
		try {
			const result = await api.post<{ data: Reminder }>(`/reminders/event/${eventId}`, {
				remindAt: new Date(reminderRemindAt).toISOString(),
				targetGroup: reminderTargetGroup,
				message: reminderMessage.trim()
			});
			reminders = [...reminders, result.data].sort((a, b) => a.remindAt.localeCompare(b.remindAt));
			reminderMessage = '';
			toast.success('Reminder scheduled');
		} catch (err: unknown) {
			const apiErr = err as { message?: string };
			toast.error(apiErr.message || 'Failed to schedule reminder');
		} finally {
			creatingReminder = false;
		}
	}

	async function cancelReminder(reminderId: string) {
		try {
			await api.delete<{ data: { message: string } }>(`/reminders/${reminderId}`);
			reminders = reminders.map((r) => (r.id === reminderId ? { ...r, status: 'cancelled' } : r));
			cancelReminderTarget = null;
			showCancelReminderModal = false;
			toast.success('Reminder cancelled');
		} catch (err: unknown) {
			const apiErr = err as { message?: string };
			toast.error(apiErr.message || 'Failed to cancel reminder');
		}
	}

	// CSV Export
	let exportMenuOpen = $state(false);
	let exportDropdownRef: HTMLDivElement = $state(undefined as unknown as HTMLDivElement);

	function exportCSV(status: string) {
		const a = document.createElement('a');
		a.href = `/api/v1/rsvp/event/${eventId}/export?status=${status}`;
		a.download = '';
		a.click();
		exportMenuOpen = false;
	}

	function handleExportClickOutside(e: MouseEvent) {
		if (exportDropdownRef && !exportDropdownRef.contains(e.target as Node)) {
			exportMenuOpen = false;
		}
	}

	function handleExportKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') exportMenuOpen = false;
	}

	// Editing attendees
	let editingAttendeeId: string | null = $state(null);
	let editAttendee = $state({ name: '', email: '', phone: '', rsvpStatus: '', dietaryNotes: '', plusOnes: 0 });
	let savingAttendee = $state(false);

	function startEditAttendee(attendee: Attendee) {
		editingAttendeeId = attendee.id;
		editAttendee = {
			name: attendee.name,
			email: attendee.email || '',
			phone: attendee.phone || '',
			rsvpStatus: attendee.rsvpStatus,
			dietaryNotes: attendee.dietaryNotes,
			plusOnes: attendee.plusOnes
		};
	}

	function cancelEditAttendee() {
		editingAttendeeId = null;
	}

	async function saveAttendee() {
		if (!editingAttendeeId) return;
		savingAttendee = true;
		try {
			const result = await api.patch<{ data: Attendee }>(`/rsvp/event/${eventId}/${editingAttendeeId}`, {
				name: editAttendee.name,
				email: editAttendee.email || undefined,
				phone: editAttendee.phone || undefined,
				rsvpStatus: editAttendee.rsvpStatus,
				dietaryNotes: editAttendee.dietaryNotes,
				plusOnes: editAttendee.plusOnes
			});
			attendees = attendees.map((a) => (a.id === editingAttendeeId ? result.data : a));
			editingAttendeeId = null;
			toast.success('Attendee updated');
			// Re-fetch stats to reflect changes in status/plus-ones.
			try {
				const refreshed = await api.get<{ data: RSVPStats }>(`/rsvp/event/${eventId}/stats`);
				stats = refreshed.data;
			} catch { /* non-critical */ }
		} catch (err: unknown) {
			const apiErr = err as { message?: string };
			toast.error(apiErr.message || 'Failed to update attendee');
		} finally {
			savingAttendee = false;
		}
	}

	async function removeAttendee(attendeeId: string) {
		try {
			await api.delete<{ data: { message: string } }>(`/rsvp/event/${eventId}/${attendeeId}`);
			attendees = attendees.filter((a) => a.id !== attendeeId);
			removeAttendeeTarget = null;
			showRemoveAttendeeModal = false;
			toast.success('Attendee removed');
			// Re-fetch stats to reflect removal (status counts, headcount, plus-ones).
			try {
				const refreshed = await api.get<{ data: RSVPStats }>(`/rsvp/event/${eventId}/stats`);
				stats = refreshed.data;
			} catch { /* non-critical */ }
		} catch (err: unknown) {
			const apiErr = err as { message?: string };
			toast.error(apiErr.message || 'Failed to remove attendee');
		}
	}

	// Promote waitlisted attendee
	let promotingAttendeeId: string | null = $state(null);
	async function promoteAttendee(attendeeId: string) {
		promotingAttendeeId = attendeeId;
		try {
			const result = await api.post<{ data: Attendee }>(`/rsvp/event/${eventId}/${attendeeId}/promote`);
			attendees = attendees.map((a) => (a.id === attendeeId ? result.data : a));
			toast.success('Attendee promoted from waitlist');
			// Re-fetch stats
			try {
				const refreshed = await api.get<{ data: RSVPStats }>(`/rsvp/event/${eventId}/stats`);
				stats = refreshed.data;
			} catch { /* non-critical */ }
		} catch (err: unknown) {
			const apiErr = err as { message?: string };
			toast.error(apiErr.message || 'Failed to promote attendee');
		} finally {
			promotingAttendeeId = null;
		}
	}

	// Co-host management
	async function addCoHost() {
		if (!cohostEmail.trim()) return;
		addingCohost = true;
		try {
			const result = await api.post<{ data: CoHost }>(`/events/${eventId}/cohosts`, {
				email: cohostEmail.trim()
			});
			cohosts = [...cohosts, result.data];
			cohostEmail = '';
			toast.success('Co-host added');
		} catch (err: unknown) {
			const apiErr = err as { message?: string };
			toast.error(apiErr.message || 'Failed to add co-host');
		} finally {
			addingCohost = false;
		}
	}

	async function removeCoHost(cohostId: string) {
		try {
			await api.delete(`/events/${eventId}/cohosts/${cohostId}`);
			cohosts = cohosts.filter(c => c.id !== cohostId);
			toast.success('Co-host removed');
		} catch (err: unknown) {
			const apiErr = err as { message?: string };
			toast.error(apiErr.message || 'Failed to remove co-host');
		}
	}

	// Comment management
	async function deleteComment(commentId: string) {
		try {
			await api.delete(`/comments/event/${eventId}/${commentId}`);
			eventComments = eventComments.filter(c => c.id !== commentId);
			toast.success('Comment deleted');
		} catch (err: unknown) {
			const apiErr = err as { message?: string };
			toast.error(apiErr.message || 'Failed to delete comment');
		}
	}

	// Editing reminders
	let editingReminderId: string | null = $state(null);
	let editRemindAt = $state('');
	let editTargetGroup: Reminder['targetGroup'] = $state('all');
	let editMessage = $state('');
	let savingReminder = $state(false);

	function startEditReminder(reminder: Reminder) {
		editingReminderId = reminder.id;
		editRemindAt = toISOLocal(new Date(reminder.remindAt));
		editTargetGroup = reminder.targetGroup;
		editMessage = reminder.message;
	}

	function cancelEditReminder() {
		editingReminderId = null;
	}

	async function saveReminder() {
		if (!editingReminderId) return;
		savingReminder = true;
		try {
			const result = await api.put<{ data: Reminder }>(`/reminders/${editingReminderId}`, {
				remindAt: new Date(editRemindAt).toISOString(),
				targetGroup: editTargetGroup,
				message: editMessage.trim()
			});
			reminders = reminders
				.map((r) => (r.id === editingReminderId ? result.data : r))
				.sort((a, b) => a.remindAt.localeCompare(b.remindAt));
			editingReminderId = null;
			toast.success('Reminder updated');
		} catch (err: unknown) {
			const apiErr = err as { message?: string };
			toast.error(apiErr.message || 'Failed to update reminder');
		} finally {
			savingReminder = false;
		}
	}
</script>

<svelte:head>
	<title>{event?.title || 'Event Details'} -- OpenRSVP</title>
</svelte:head>

<AppShell>
	{#if loading}
		<div class="flex items-center justify-center py-16">
			<Spinner size="lg" class="text-primary" />
		</div>
	{:else if event}
		<!-- Back link + actions -->
		<div class="mb-6 flex items-center justify-between">
			<a href="/events" class="text-sm text-primary hover:text-primary-hover">&larr; Back to events</a>
			<div class="flex items-center gap-2">
				<Button variant="outline" size="sm" href="/events/{eventId}/edit">Edit</Button>
				<Button variant="outline" size="sm" href="/events/{eventId}/invite">Design Invite</Button>
				<Button variant="outline" size="sm" href="/events/{eventId}/share">Share</Button>
				<Button variant="outline" size="sm" href="/events/{eventId}/messages">Send Message</Button>
				<Button variant="outline" size="sm" href="/events/{eventId}/import">Import Guests</Button>
				<Button variant="outline" size="sm" href="/events/{eventId}/webhooks">Webhooks</Button>
				<Button variant="outline" size="sm" onclick={duplicateEvent}>Duplicate</Button>
			</div>
		</div>

		<!-- Series banner -->
		{#if event.seriesId}
			<div class="mb-4 flex items-center gap-2 rounded-lg border border-primary-light bg-primary-lighter px-4 py-3 text-sm text-primary">
				<svg class="h-4 w-4 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path stroke-linecap="round" stroke-linejoin="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
				</svg>
				<span>
					Part of a <a href="/events/series/{event.seriesId}" class="font-medium underline underline-offset-2 hover:text-primary-hover">recurring series</a>
					{#if event.seriesOverride}
						<span class="text-warning font-medium">(Modified)</span>
					{/if}
				</span>
			</div>
		{/if}

		<!-- Event info card -->
		<Card class="mb-6">
			<div class="flex items-start justify-between">
				<div>
					<h1 class="text-2xl font-bold font-display text-neutral-900">{event.title}</h1>
					<p class="mt-2 text-sm text-neutral-600">{formatDateTime(event.eventDate, event.timezone)}</p>
					{#if event.endDate}
						<p class="text-sm text-neutral-500">Ends: {formatDateTime(event.endDate, event.timezone)}</p>
					{/if}
					{#if event.location}
						<p class="mt-1 text-sm text-neutral-500 flex items-center gap-1">
							<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17.657 16.657L13.414 20.9a1.998 1.998 0 01-2.827 0l-4.244-4.243a8 8 0 1111.314 0z" />
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 11a3 3 0 11-6 0 3 3 0 016 0z" />
							</svg>
							{event.location}
						</p>
					{/if}
					{#if event.description}
						<p class="mt-3 text-sm text-neutral-700 whitespace-pre-wrap">{event.description}</p>
					{/if}
				</div>
				<div class="flex flex-col items-end gap-2">
					<Badge variant={statusVariant(event.status)}>{event.status}</Badge>
					{#if event.status === 'draft'}
						<Button size="sm" onclick={publishEvent}>Publish</Button>
					{:else if event.status === 'published'}
						{#if event.organizerId === currentUserId}
							<Button variant="danger" size="sm" onclick={() => showCancelModal = true}>Cancel Event</Button>
						{/if}
					{:else if event.status === 'cancelled'}
						{#if event.organizerId === currentUserId}
							<Button size="sm" onclick={reopenEvent}>Re-open as Draft</Button>
						{/if}
					{/if}
				</div>
			</div>
			{#if event.showHeadcount || event.showGuestList}
				<div class="mt-4 pt-4 border-t border-neutral-200">
					<div class="flex items-center gap-2">
						<svg class="w-4 h-4 text-primary flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
							<path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
							<path stroke-linecap="round" stroke-linejoin="round" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
						</svg>
						<p class="text-xs text-neutral-500">
							Public visibility:
							{#if event.showHeadcount && event.showGuestList}
								attendance count and guest names
							{:else if event.showHeadcount}
								attendance count
							{:else}
								guest names
							{/if}
							<a href="/events/{eventId}/edit" class="text-primary hover:text-primary underline underline-offset-2 ml-1">Edit</a>
						</p>
					</div>
				</div>
			{/if}
			{#if event.shareToken}
				<div class="mt-4 pt-4 border-t border-neutral-200 flex items-center gap-2">
					<p class="text-xs text-neutral-500">
						Share link: <code class="bg-neutral-100 px-1.5 py-0.5 rounded font-mono text-primary">{$page.url.origin}/i/{event.shareToken}</code>
					</p>
					<button
						type="button"
						onclick={copyShareLink}
						class="inline-flex items-center justify-center rounded p-1 text-neutral-400 hover:text-primary hover:bg-neutral-100 transition-colors"
						title="Copy share link"
					>
						{#if copied}
							<svg class="h-4 w-4 text-success" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
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
		<div class="grid grid-cols-2 sm:grid-cols-4 gap-4 mb-6">
			<div class="rounded-lg border border-neutral-200 p-4 bg-success-light">
				<div class="flex items-baseline gap-2">
					<p class="text-2xl font-bold font-mono text-success">{stats.attendingHeadcount}</p>
					{#if stats.attendingHeadcount !== stats.attending}
						<p class="text-xs text-success/70">({stats.attending} + {stats.attendingHeadcount - stats.attending} guests)</p>
					{/if}
				</div>
				<p class="text-xs font-medium text-success mt-1">Attending</p>
			</div>
			<div class="rounded-lg border border-neutral-200 p-4 bg-warning-light">
				<div class="flex items-baseline gap-2">
					<p class="text-2xl font-bold font-mono text-warning">{stats.maybeHeadcount}</p>
					{#if stats.maybeHeadcount !== stats.maybe}
						<p class="text-xs text-warning/70">({stats.maybe} + {stats.maybeHeadcount - stats.maybe} guests)</p>
					{/if}
				</div>
				<p class="text-xs font-medium text-warning mt-1">Maybe</p>
			</div>
			<div class="rounded-lg border border-neutral-200 p-4 bg-error-light">
				<p class="text-2xl font-bold font-mono text-error">{stats.declined}</p>
				<p class="text-xs font-medium text-error mt-1">Declined</p>
			</div>
			<div class="rounded-lg border border-neutral-200 p-4 bg-neutral-50">
				<div class="flex items-baseline gap-2">
					<p class="text-2xl font-bold font-mono text-neutral-700">{stats.totalHeadcount}</p>
					{#if stats.totalHeadcount !== stats.total}
						<p class="text-xs text-neutral-500">({stats.total} + {stats.totalHeadcount - stats.total} guests)</p>
					{/if}
				</div>
				<p class="text-xs font-medium text-neutral-600 mt-1">Total</p>
			</div>
		</div>

		{#if stats.waitlisted > 0}
			<div class="grid grid-cols-1 gap-4 mb-6">
				<div class="rounded-lg border border-info p-4 bg-info-light">
					<div class="flex items-baseline gap-2">
						<p class="text-2xl font-bold font-mono text-info">{stats.waitlisted}</p>
					</div>
					<p class="text-xs font-medium text-info mt-1">Waitlisted</p>
				</div>
			</div>
		{/if}

		<!-- Capacity Status -->
		{#if event.maxCapacity}
			<div class="mb-6 flex items-center gap-2 text-sm text-neutral-600">
				<svg class="h-4 w-4 text-neutral-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path stroke-linecap="round" stroke-linejoin="round" d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0z" />
				</svg>
				Capacity: {stats.attendingHeadcount} / {event.maxCapacity}
				{#if stats.attendingHeadcount >= event.maxCapacity}
					<Badge variant="error">Full</Badge>
				{/if}
				{#if event.waitlistEnabled}
					<Badge variant="info">Waitlist On</Badge>
				{/if}
			</div>
		{/if}

		<!-- RSVP Deadline Display -->
		{#if event.rsvpDeadline}
			<div class="mb-6 flex items-center gap-2 text-sm text-neutral-600">
				<svg class="h-4 w-4 text-neutral-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path stroke-linecap="round" stroke-linejoin="round" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
				</svg>
				RSVP Deadline: {formatDateTime(event.rsvpDeadline, event.timezone)}
				{#if new Date(event.rsvpDeadline) < new Date()}
					<Badge variant="warning">Closed</Badge>
				{/if}
			</div>
		{/if}

		<!-- Reminder management -->
		<Card class="mb-6">
			{#snippet header()}
				<h2 class="text-lg font-semibold font-display text-neutral-900">Scheduled Reminders</h2>
			{/snippet}

			<form
				onsubmit={(e) => {
					e.preventDefault();
					createReminder();
				}}
				class="space-y-4"
			>
				<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
					<DateTimePicker
						label="Remind At"
						name="remindAt"
						bind:value={reminderRemindAt}
						min={reminderMinDate}
						error={reminderErrors.remindAt || ''}
						required
					/>
					<Select
						label="Audience"
						name="targetGroup"
						bind:value={reminderTargetGroup}
						options={reminderTargetOptions}
					/>
				</div>

				<Textarea
					label="Custom Message (optional)"
					name="message"
					bind:value={reminderMessage}
					placeholder="Don't forget to RSVP before Friday!"
					rows={3}
				/>

				<div class="flex justify-end">
					<Button type="submit" loading={creatingReminder}>Schedule Reminder</Button>
				</div>
			</form>

			{#if reminders.length === 0}
				<p class="text-sm text-neutral-500 text-center py-8 border-t border-neutral-200 mt-6">
					No reminders scheduled.
				</p>
			{:else}
				<div class="divide-y divide-neutral-200 -mx-6 -mb-4 border-t border-neutral-200 mt-6">
					{#each reminders as reminder (reminder.id)}
						{#if editingReminderId === reminder.id}
							<div class="px-6 py-4 space-y-4 bg-neutral-50">
								<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
									<DateTimePicker
										label="Remind At"
										name="editRemindAt"
										bind:value={editRemindAt}
										min={reminderMinDate}
										required
									/>
									<Select
										label="Audience"
										name="editTargetGroup"
										bind:value={editTargetGroup}
										options={reminderTargetOptions}
									/>
								</div>
								<Textarea
									label="Message (optional)"
									name="editMessage"
									bind:value={editMessage}
									placeholder="Custom reminder message..."
									rows={2}
								/>
								<div class="flex items-center justify-end gap-2">
									<Button size="sm" variant="outline" onclick={cancelEditReminder}>Cancel</Button>
									<Button size="sm" onclick={saveReminder} loading={savingReminder}>Save</Button>
								</div>
							</div>
						{:else}
							<div class="px-6 py-4 flex items-center justify-between gap-4">
								<div class="min-w-0">
									<p class="text-sm font-medium text-neutral-900">
										{formatDateTime(reminder.remindAt)}
									</p>
									<p class="text-xs text-neutral-500 mt-0.5">
										Audience: {reminder.targetGroup}
									</p>
									{#if reminder.message}
										<p class="text-sm text-neutral-700 mt-2 whitespace-pre-wrap">{reminder.message}</p>
									{/if}
								</div>
								<div class="flex items-center gap-2">
									<Badge variant={statusVariant(reminder.status)}>{reminder.status}</Badge>
									{#if reminder.status === 'scheduled'}
										<Button size="sm" variant="outline" onclick={() => startEditReminder(reminder)}>Edit</Button>
										<Button size="sm" variant="outline" onclick={() => { cancelReminderTarget = reminder; showCancelReminderModal = true; }}>Cancel</Button>
									{/if}
								</div>
							</div>
						{/if}
					{/each}
				</div>
			{/if}
		</Card>

		<!-- Attendee list -->
		<Card>
			{#snippet header()}
				<div class="flex items-center justify-between">
					<div class="flex items-center gap-3">
						<h2 class="text-lg font-semibold font-display text-neutral-900">Attendees</h2>
						<!-- CSV Export split-button -->
						<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
					<div class="relative inline-flex" role="group" bind:this={exportDropdownRef} onkeydown={handleExportKeydown}>
							<Button variant="outline" size="sm" class="rounded-r-none border-r-0" onclick={() => exportCSV('all')}>
								Export CSV
							</Button>
							<button
								onclick={() => (exportMenuOpen = !exportMenuOpen)}
								aria-expanded={exportMenuOpen}
								aria-haspopup="true"
								aria-label="Export options"
								class="inline-flex items-center rounded-l-none rounded-r-lg border border-neutral-300 bg-surface px-2 py-1.5 text-neutral-500 hover:bg-neutral-50 focus:outline-none focus:ring-2 focus:ring-primary/40"
							>
								<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
									<path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
								</svg>
							</button>
							{#if exportMenuOpen}
								<div
									class="absolute right-0 z-10 mt-1 w-48 rounded-lg bg-surface shadow-lg border border-neutral-200 py-1 top-full"
									role="menu"
									aria-label="Export filter options"
								>
									<button onclick={() => exportCSV('attending')} role="menuitem" class="block w-full text-left px-4 py-2 text-sm text-neutral-700 hover:bg-neutral-50 focus:bg-neutral-50 focus:outline-none">
										Attending Only
									</button>
									<button onclick={() => exportCSV('maybe')} role="menuitem" class="block w-full text-left px-4 py-2 text-sm text-neutral-700 hover:bg-neutral-50 focus:bg-neutral-50 focus:outline-none">
										Maybe Only
									</button>
									<button onclick={() => exportCSV('declined')} role="menuitem" class="block w-full text-left px-4 py-2 text-sm text-neutral-700 hover:bg-neutral-50 focus:bg-neutral-50 focus:outline-none">
										Declined Only
									</button>
									<button onclick={() => exportCSV('pending')} role="menuitem" class="block w-full text-left px-4 py-2 text-sm text-neutral-700 hover:bg-neutral-50 focus:bg-neutral-50 focus:outline-none">
										Pending Only
									</button>
								</div>
							{/if}
						</div>
					</div>
					<div class="flex gap-1">
						{#each ['all', 'attending', 'maybe', 'declined', 'waitlisted'] as filter}
							<button
								type="button"
								class="px-3 py-1 rounded-full text-xs font-medium transition-colors {activeFilter === filter
									? 'bg-primary text-white'
									: 'bg-neutral-100 text-neutral-600 hover:bg-neutral-200'}"
								onclick={() => (activeFilter = filter)}
							>
								{filter.charAt(0).toUpperCase() + filter.slice(1)}
							</button>
						{/each}
					</div>
				</div>
			{/snippet}

			{#if filteredAttendees.length === 0}
				<p class="text-sm text-neutral-500 text-center py-8">
					{attendees.length === 0 ? 'No attendees yet. Share your event to start receiving RSVPs.' : 'No attendees match this filter.'}
				</p>
			{:else}
				<div class="divide-y divide-neutral-200 -mx-6 -mb-4">
					{#each filteredAttendees as attendee (attendee.id)}
						{#if editingAttendeeId === attendee.id}
							<div class="px-6 py-4 space-y-4 bg-neutral-50">
								<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
									<div>
										<label for="edit-name" class="block text-xs font-medium text-neutral-700 mb-1">Name</label>
										<input id="edit-name" type="text" bind:value={editAttendee.name} class="w-full rounded-lg border border-neutral-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary/40 focus:border-primary" />
									</div>
									<div>
										<label for="edit-email" class="block text-xs font-medium text-neutral-700 mb-1">Email</label>
										<input id="edit-email" type="email" bind:value={editAttendee.email} class="w-full rounded-lg border border-neutral-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary/40 focus:border-primary" />
									</div>
									<div>
										<label for="edit-phone" class="block text-xs font-medium text-neutral-700 mb-1">Phone</label>
										<input id="edit-phone" type="tel" bind:value={editAttendee.phone} class="w-full rounded-lg border border-neutral-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary/40 focus:border-primary" />
									</div>
									<div>
										<label for="edit-status" class="block text-xs font-medium text-neutral-700 mb-1">Status</label>
										<select id="edit-status" bind:value={editAttendee.rsvpStatus} class="w-full rounded-lg border border-neutral-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary/40 focus:border-primary">
											<option value="attending">Attending</option>
											<option value="maybe">Maybe</option>
											<option value="declined">Declined</option>
											<option value="pending">Pending</option>
											<option value="waitlisted">Waitlisted</option>
										</select>
									</div>
									<div>
										<label for="edit-dietary" class="block text-xs font-medium text-neutral-700 mb-1">Dietary Notes</label>
										<input id="edit-dietary" type="text" bind:value={editAttendee.dietaryNotes} class="w-full rounded-lg border border-neutral-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary/40 focus:border-primary" />
									</div>
									<div>
										<label for="edit-plusones" class="block text-xs font-medium text-neutral-700 mb-1">Plus Ones</label>
										<input id="edit-plusones" type="number" min="0" max="10" bind:value={editAttendee.plusOnes} class="w-full rounded-lg border border-neutral-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary/40 focus:border-primary" />
									</div>
								</div>
								<div class="flex items-center justify-end gap-2">
									<Button size="sm" variant="outline" onclick={cancelEditAttendee}>Cancel</Button>
									<Button size="sm" onclick={saveAttendee} loading={savingAttendee}>Save</Button>
								</div>
							</div>
						{:else}
							<div class="px-6 py-3 flex items-center justify-between">
								<div class="flex-1 min-w-0">
									<p class="text-sm font-medium text-neutral-900">{attendee.name}</p>
									<p class="text-xs text-neutral-500">
										{attendee.email || attendee.phone || 'No contact info'}
									</p>
								</div>
								<div class="flex items-center gap-3 ml-4">
									{#if attendee.dietaryNotes}
										<span class="text-xs text-neutral-500" title="Dietary notes">{attendee.dietaryNotes}</span>
									{/if}
									{#if attendee.plusOnes > 0}
										<span class="text-xs text-neutral-500">+{attendee.plusOnes}</span>
									{/if}
									<Badge variant={statusVariant(attendee.rsvpStatus)}>
										{attendee.rsvpStatus}
									</Badge>
									{#if attendee.rsvpStatus === "waitlisted"}
										<Button size="sm" loading={promotingAttendeeId === attendee.id} onclick={() => promoteAttendee(attendee.id)}>Promote</Button>
									{/if}
									<Button size="sm" variant="outline" onclick={() => startEditAttendee(attendee)}>Edit</Button>
									<Button size="sm" variant="danger" onclick={() => { removeAttendeeTarget = attendee; showRemoveAttendeeModal = true; }}>Remove</Button>
								</div>
							</div>
						{/if}
					{/each}
				</div>
			{/if}
		</Card>

		<!-- Co-host management (owner only) -->
		{#if event && event.organizerId === currentUserId}
			<Card class="mt-6">
				{#snippet header()}
					<div class="flex items-center gap-2">
						<h2 class="text-lg font-semibold font-display text-neutral-900">Co-hosts</h2>
						<span class="group relative">
							<svg class="h-4 w-4 text-neutral-400 cursor-help" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
								<circle cx="12" cy="12" r="10" /><path d="M12 16v-4m0-4h.01" />
							</svg>
							<span class="invisible group-hover:visible absolute left-6 top-0 z-10 w-64 rounded-lg bg-neutral-800 px-3 py-2 text-xs text-white shadow-lg">
								Co-hosts can edit event details, manage guests, and send messages. Only the event owner can delete the event or manage co-hosts.
							</span>
						</span>
					</div>
				{/snippet}

				{#if cohosts.length === 0}
					<p class="text-sm text-neutral-500 py-2">No co-hosts yet.</p>
				{:else}
					<div class="divide-y divide-neutral-100">
						{#each cohosts as cohost (cohost.id)}
							<div class="flex items-center justify-between py-3">
								<div>
									<p class="text-sm font-medium text-neutral-900">{cohost.organizerName || cohost.organizerEmail}</p>
									<p class="text-xs text-neutral-500">{cohost.organizerEmail}</p>
								</div>
								<Button variant="ghost" size="sm" onclick={() => removeCoHost(cohost.id)}>
									Remove
								</Button>
							</div>
						{/each}
					</div>
				{/if}

				{#if cohosts.length < 10}
					<form
						onsubmit={(e) => { e.preventDefault(); addCoHost(); }}
						class="mt-4 flex gap-2"
					>
						<div class="flex-1">
							<Input
								name="cohostEmail"
								bind:value={cohostEmail}
								placeholder="Co-host email address"
								type="email"
							/>
						</div>
						<Button type="submit" size="sm" loading={addingCohost}>Add</Button>
					</form>
				{:else}
					<p class="text-xs text-neutral-400 mt-4">Maximum 10 co-hosts reached.</p>
				{/if}
			</Card>
		{/if}

		<!-- Comments -->
		{#if event}
			<Card class="mt-6">
				{#snippet header()}
					<h2 class="text-lg font-semibold font-display text-neutral-900">Comments ({eventComments.length})</h2>
				{/snippet}
				{#if eventComments.length === 0}
					<p class="text-sm text-neutral-500 text-center py-8">No comments yet.</p>
				{:else}
					<div class="divide-y divide-neutral-200 -mx-6 -mb-4">
						{#each eventComments as comment (comment.id)}
							<div class="px-6 py-3 flex items-start justify-between">
								<div class="min-w-0 flex-1">
									<div class="flex items-center gap-2">
										<p class="text-sm font-medium text-neutral-900">{comment.authorName}</p>
										<span class="text-xs text-neutral-400">{new Date(comment.createdAt).toLocaleDateString()}</span>
									</div>
									<p class="text-sm text-neutral-700 mt-1 whitespace-pre-wrap">{comment.body}</p>
								</div>
								<Button size="sm" variant="ghost" onclick={() => deleteComment(comment.id)}>Delete</Button>
							</div>
						{/each}
					</div>
				{/if}
			</Card>
		{/if}

		<!-- Email Delivery Stats -->
		{#if emailStats && emailStats.totalSent > 0}
			<Card class="mt-6">
				{#snippet header()}
					<h2 class="text-lg font-semibold font-display text-neutral-900">Email Delivery</h2>
				{/snippet}
				<div class="grid grid-cols-2 sm:grid-cols-4 gap-4">
					<div class="text-center">
						<p class="text-2xl font-bold font-display text-neutral-900">{emailStats.totalSent}</p>
						<p class="text-xs text-neutral-500">Sent</p>
					</div>
					<div class="text-center">
						<p class="text-2xl font-bold font-mono text-success">{emailStats.delivered}</p>
						<p class="text-xs text-neutral-500">Delivered</p>
					</div>
					<div class="text-center">
						<p class="text-2xl font-bold font-mono text-info">{emailStats.opened}</p>
						<p class="text-xs text-neutral-500">Opened</p>
					</div>
					<div class="text-center">
						<p class="text-2xl font-bold font-mono text-error">{emailStats.bounced}</p>
						<p class="text-xs text-neutral-500">Bounced</p>
					</div>
				</div>
			</Card>
		{/if}
	{:else}
		<Card>
			<p class="text-center text-neutral-500 py-8">Event not found.</p>
		</Card>
	{/if}

	{#if event}
		<Modal bind:open={showCancelModal} title="Cancel Event">
			<p class="text-sm text-neutral-600">
				Are you sure you want to cancel <strong>{event.title}</strong>? Attendees will no longer be able to RSVP.
			</p>
			<label class="flex items-center gap-3 mt-4 cursor-pointer">
				<input
					type="checkbox"
					bind:checked={notifyOnCancel}
					class="rounded border-neutral-300 text-primary focus:ring-primary/40"
				/>
				<span class="text-sm text-neutral-700">Notify attending and maybe attendees about cancellation</span>
			</label>
			{#snippet actions()}
				<Button variant="outline" size="sm" onclick={() => showCancelModal = false}>Keep Event</Button>
				<Button variant="danger" size="sm" onclick={cancelEvent}>Cancel Event</Button>
			{/snippet}
		</Modal>
	{/if}

	{#if removeAttendeeTarget}
		{@const target = removeAttendeeTarget}
		<Modal bind:open={showRemoveAttendeeModal} title="Remove Attendee">
			<p class="text-sm text-neutral-600">
				Are you sure you want to remove <strong>{target.name}</strong>? This action cannot be undone.
			</p>
			{#snippet actions()}
				<Button variant="outline" size="sm" onclick={() => showRemoveAttendeeModal = false}>Keep Attendee</Button>
				<Button variant="danger" size="sm" onclick={() => removeAttendee(target.id)}>Remove</Button>
			{/snippet}
		</Modal>
	{/if}

	{#if cancelReminderTarget}
		{@const target = cancelReminderTarget}
		<Modal bind:open={showCancelReminderModal} title="Cancel Reminder">
			<p class="text-sm text-neutral-600">
				Are you sure you want to cancel the reminder scheduled for <strong>{formatDateTime(target.remindAt)}</strong>?
			</p>
			{#snippet actions()}
				<Button variant="outline" size="sm" onclick={() => showCancelReminderModal = false}>Keep Reminder</Button>
				<Button variant="danger" size="sm" onclick={() => cancelReminder(target.id)}>Cancel Reminder</Button>
			{/snippet}
		</Modal>
	{/if}
</AppShell>
