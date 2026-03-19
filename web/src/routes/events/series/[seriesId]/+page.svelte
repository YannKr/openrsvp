<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { api } from '$lib/api/client';
	import { toast } from '$lib/stores/toast';
	import { smsEnabled, loadAppConfig } from '$lib/stores/config';
	import { currentUser } from '$lib/stores/auth';
	import { formatDateTime, isInPast } from '$lib/utils/dates';
	import { getTimezoneOptions, getTimezoneLabel } from '$lib/utils/timezones';
	import type { EventSeries, Event } from '$lib/types';
	import AppShell from '$lib/components/layout/AppShell.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Textarea from '$lib/components/ui/Textarea.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import Modal from '$lib/components/ui/Modal.svelte';
	import Spinner from '$lib/components/ui/Spinner.svelte';
	import { onMount } from 'svelte';

	let loading = $state(true);
	let series: EventSeries | null = $state(null);
	let occurrences: Event[] = $state([]);
	let showStopModal = $state(false);
	let showDeleteModal = $state(false);
	let stopping = $state(false);
	let deleting = $state(false);

	// Editing state
	let editing = $state(false);
	let saving = $state(false);
	let editTitle = $state('');
	let editDescription = $state('');
	let editLocation = $state('');
	let editTimezone = $state('');
	let editEventTime = $state('');
	let editDurationMinutes = $state('');
	let editContactRequirement = $state('email');
	let editShowHeadcount = $state(false);
	let editShowGuestList = $state(false);
	let editRsvpDeadlineOffsetHours = $state('');
	let editMaxCapacity = $state('');
	let editErrors: Record<string, string> = $state({});

	const seriesId = $derived($page.params.seriesId);

	const recurrenceLabels: Record<string, string> = {
		weekly: 'Weekly',
		biweekly: 'Every 2 weeks',
		monthly: 'Monthly'
	};

	const defaultTz = $currentUser?.timezone
		|| Intl.DateTimeFormat().resolvedOptions().timeZone
		|| '';
	const tzOptions = getTimezoneOptions(defaultTz);

	const contactRequirementOptions = [
		{ value: 'email_or_phone', label: 'Email or Phone (at least one)' },
		{ value: 'email', label: 'Email only' },
		{ value: 'phone', label: 'Phone only' },
		{ value: 'email_and_phone', label: 'Email and Phone (both required)' }
	];

	const filteredContactOptions = $derived(
		$smsEnabled
			? contactRequirementOptions
			: contactRequirementOptions.filter(o => o.value !== 'phone')
	);

	onMount(async () => {
		loadAppConfig();
		try {
			const result = await api.get<{ data: { series: EventSeries; occurrences: Event[] } }>(`/events/series/${seriesId}`);
			series = result.data.series;
			occurrences = result.data.occurrences;
		} catch (err: unknown) {
			const apiErr = err as { message?: string };
			toast.error(apiErr.message || 'Failed to load series');
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
			active: 'success',
			stopped: 'neutral'
		};
		return map[status] || 'neutral';
	}

	function startEdit() {
		if (!series) return;
		editTitle = series.title;
		editDescription = series.description;
		editLocation = series.location;
		editTimezone = series.timezone;
		editEventTime = series.eventTime;
		editDurationMinutes = series.durationMinutes != null ? String(series.durationMinutes) : '';
		editContactRequirement = series.contactRequirement;
		editShowHeadcount = series.showHeadcount;
		editShowGuestList = series.showGuestList;
		editRsvpDeadlineOffsetHours = series.rsvpDeadlineOffsetHours != null ? String(series.rsvpDeadlineOffsetHours) : '';
		editMaxCapacity = series.maxCapacity != null ? String(series.maxCapacity) : '';
		editErrors = {};
		editing = true;
	}

	function cancelEdit() {
		editing = false;
	}

	async function saveEdit() {
		editErrors = {};
		if (!editTitle.trim()) editErrors.title = 'Title is required';
		if (!editTimezone) editErrors.timezone = 'Timezone is required';
		if (!editEventTime) editErrors.eventTime = 'Event time is required';
		if (editDurationMinutes) {
			const d = parseInt(editDurationMinutes);
			if (isNaN(d) || d < 1) editErrors.durationMinutes = 'Must be at least 1';
		}
		if (editMaxCapacity) {
			const parsed = Number(editMaxCapacity);
			if (!Number.isInteger(parsed) || parsed < 1) editErrors.maxCapacity = 'Must be at least 1';
		}
		if (editRsvpDeadlineOffsetHours) {
			const h = parseInt(editRsvpDeadlineOffsetHours);
			if (isNaN(h) || h < 1) editErrors.rsvpDeadlineOffsetHours = 'Must be at least 1';
		}
		if (Object.keys(editErrors).length > 0) return;

		saving = true;
		try {
			const body: Record<string, unknown> = {
				title: editTitle.trim(),
				description: editDescription.trim(),
				location: editLocation.trim(),
				timezone: editTimezone,
				eventTime: editEventTime,
				contactRequirement: editContactRequirement,
				showHeadcount: editShowHeadcount,
				showGuestList: editShowGuestList
			};
			if (editDurationMinutes) body.durationMinutes = parseInt(editDurationMinutes);
			if (editRsvpDeadlineOffsetHours) body.rsvpDeadlineOffsetHours = parseInt(editRsvpDeadlineOffsetHours);
			if (editMaxCapacity) body.maxCapacity = parseInt(editMaxCapacity);

			const result = await api.put<{ data: EventSeries }>(`/events/series/${seriesId}`, body);
			series = result.data;
			editing = false;
			toast.success('Series updated');
		} catch (err: unknown) {
			const apiErr = err as { message?: string };
			toast.error(apiErr.message || 'Failed to update series');
		} finally {
			saving = false;
		}
	}

	async function stopSeries() {
		stopping = true;
		try {
			await api.post(`/events/series/${seriesId}/stop`);
			if (series) series = { ...series, seriesStatus: 'stopped' };
			showStopModal = false;
			toast.success('Series stopped. No new occurrences will be generated.');
		} catch (err: unknown) {
			const apiErr = err as { message?: string };
			toast.error(apiErr.message || 'Failed to stop series');
		} finally {
			stopping = false;
		}
	}

	async function deleteSeries() {
		deleting = true;
		try {
			await api.delete(`/events/series/${seriesId}`);
			toast.success('Series deleted');
			goto('/events/series');
		} catch (err: unknown) {
			const apiErr = err as { message?: string };
			toast.error(apiErr.message || 'Failed to delete series');
		} finally {
			deleting = false;
		}
	}
</script>

<svelte:head>
	<title>{series?.title || 'Series Details'} -- OpenRSVP</title>
</svelte:head>

<AppShell>
	{#if loading}
		<div class="flex items-center justify-center py-16">
			<Spinner size="lg" class="text-primary" />
		</div>
	{:else if series}
		<div class="mb-6 flex items-center justify-between">
			<a href="/events/series" class="text-sm text-primary hover:text-primary-hover">&larr; Back to series</a>
			<div class="flex items-center gap-2">
				{#if !editing}
					<Button variant="outline" size="sm" onclick={startEdit}>Edit</Button>
				{/if}
				{#if series.seriesStatus === 'active'}
					<Button variant="outline" size="sm" onclick={() => showStopModal = true}>Stop Series</Button>
				{/if}
				<Button variant="danger" size="sm" onclick={() => showDeleteModal = true}>Delete</Button>
			</div>
		</div>

		{#if editing}
			<!-- Edit form -->
			<Card class="mb-6">
				<form
					onsubmit={(e) => { e.preventDefault(); saveEdit(); }}
					class="space-y-6"
				>
					<h2 class="text-lg font-semibold font-display text-neutral-900">Edit Series</h2>

					<Input
						label="Title"
						name="editTitle"
						bind:value={editTitle}
						error={editErrors.title || ''}
						required
					/>

					<Textarea
						label="Description"
						name="editDescription"
						bind:value={editDescription}
						rows={4}
					/>

					<Input
						label="Location"
						name="editLocation"
						bind:value={editLocation}
					/>

					<Select
						label="Timezone"
						name="editTimezone"
						bind:value={editTimezone}
						options={tzOptions}
						error={editErrors.timezone || ''}
						required
					/>

					<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
						<div class="space-y-1">
							<label for="editEventTime" class="block text-sm font-medium text-neutral-700">
								Event Time <span class="text-error">*</span>
							</label>
							<input
								id="editEventTime"
								type="time"
								bind:value={editEventTime}
								required
								class="block w-full rounded-lg border px-3 py-2 text-sm shadow-sm transition-colors focus:outline-none focus:ring-2 focus:ring-offset-0 {editErrors.eventTime
									? 'border-error text-error focus:border-error focus:ring-error'
									: 'border-neutral-300 text-neutral-900 focus:border-primary focus:ring-primary'}"
							/>
							{#if editErrors.eventTime}
								<p class="text-sm text-error">{editErrors.eventTime}</p>
							{/if}
						</div>
						<Input
							label="Duration (minutes)"
							name="editDurationMinutes"
							type="number"
							bind:value={editDurationMinutes}
							error={editErrors.durationMinutes || ''}
						/>
					</div>

					<Select
						label="RSVP Contact Requirement"
						name="editContactRequirement"
						bind:value={editContactRequirement}
						options={filteredContactOptions}
					/>

					<fieldset class="pt-2">
						<legend class="text-sm font-medium text-neutral-700 mb-3">Guest Visibility</legend>
						<div class="space-y-2">
							<label class="flex items-center gap-3 cursor-pointer">
								<input
									type="checkbox"
									bind:checked={editShowHeadcount}
									class="rounded border-neutral-300 text-primary focus:ring-primary/40"
								/>
								<span class="text-sm text-neutral-700">Show attendance count</span>
							</label>
							<label class="flex items-center gap-3 cursor-pointer">
								<input
									type="checkbox"
									bind:checked={editShowGuestList}
									class="rounded border-neutral-300 text-primary focus:ring-primary/40"
								/>
								<span class="text-sm text-neutral-700">Show guest names</span>
							</label>
						</div>
					</fieldset>

					<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
						<Input
							label="RSVP Deadline Offset (hours)"
							name="editRsvpDeadlineOffsetHours"
							type="number"
							bind:value={editRsvpDeadlineOffsetHours}
							error={editErrors.rsvpDeadlineOffsetHours || ''}
						/>
						<Input
							label="Max Attendees"
							name="editMaxCapacity"
							type="number"
							bind:value={editMaxCapacity}
							error={editErrors.maxCapacity || ''}
						/>
					</div>

					<div class="flex items-center justify-end gap-2 border-t border-neutral-200 pt-4">
						<Button variant="outline" onclick={cancelEdit}>Cancel</Button>
						<Button type="submit" loading={saving}>Save Changes</Button>
					</div>
				</form>
			</Card>
		{:else}
			<!-- Series info card -->
			<Card class="mb-6">
				<div class="flex items-start justify-between">
					<div>
						<h1 class="text-2xl font-bold font-display text-neutral-900">{series.title}</h1>
						<p class="mt-2 text-sm text-neutral-600">
							{recurrenceLabels[series.recurrenceRule] || series.recurrenceRule} at {series.eventTime}
						</p>
						{#if series.location}
							<p class="mt-1 text-sm text-neutral-500 flex items-center gap-1">
								<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17.657 16.657L13.414 20.9a1.998 1.998 0 01-2.827 0l-4.244-4.243a8 8 0 1111.314 0z" />
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 11a3 3 0 11-6 0 3 3 0 016 0z" />
								</svg>
								{series.location}
							</p>
						{/if}
						{#if series.description}
							<p class="mt-3 text-sm text-neutral-700 whitespace-pre-wrap">{series.description}</p>
						{/if}
					</div>
					<Badge variant={statusVariant(series.seriesStatus)}>
						{series.seriesStatus}
					</Badge>
				</div>
				<div class="mt-4 pt-4 border-t border-neutral-200">
					<dl class="grid grid-cols-2 sm:grid-cols-3 gap-4 text-sm">
						<div>
							<dt class="text-neutral-500">Timezone</dt>
							<dd class="font-medium text-neutral-900">{getTimezoneLabel(series.timezone)}</dd>
						</div>
						{#if series.durationMinutes}
							<div>
								<dt class="text-neutral-500">Duration</dt>
								<dd class="font-medium text-neutral-900">{series.durationMinutes} min</dd>
							</div>
						{/if}
						{#if series.maxOccurrences}
							<div>
								<dt class="text-neutral-500">Max Occurrences</dt>
								<dd class="font-medium text-neutral-900">{series.maxOccurrences}</dd>
							</div>
						{/if}
						{#if series.recurrenceEnd}
							<div>
								<dt class="text-neutral-500">Ends</dt>
								<dd class="font-medium text-neutral-900">{formatDateTime(series.recurrenceEnd, series.timezone)}</dd>
							</div>
						{/if}
						{#if series.maxCapacity}
							<div>
								<dt class="text-neutral-500">Max Capacity</dt>
								<dd class="font-medium text-neutral-900">{series.maxCapacity} per occurrence</dd>
							</div>
						{/if}
						{#if series.rsvpDeadlineOffsetHours}
							<div>
								<dt class="text-neutral-500">RSVP Closes</dt>
								<dd class="font-medium text-neutral-900">{series.rsvpDeadlineOffsetHours}h before event</dd>
							</div>
						{/if}
					</dl>
				</div>
			</Card>
		{/if}

		<!-- Occurrences list -->
		<Card>
			{#snippet header()}
				<h2 class="text-lg font-semibold font-display text-neutral-900">Occurrences ({occurrences.length})</h2>
			{/snippet}

			{#if occurrences.length === 0}
				<p class="text-sm text-neutral-500 text-center py-8">
					No occurrences generated yet. They will appear here as they are created.
				</p>
			{:else}
				<div class="divide-y divide-neutral-200 -mx-6 -mb-4">
					{#each occurrences as occurrence (occurrence.id)}
						<a
							href="/events/{occurrence.id}"
							class="block px-6 py-4 hover:bg-neutral-50 transition-colors {isInPast(occurrence.eventDate) ? 'opacity-50' : ''}"
						>
							<div class="flex items-center justify-between">
								<div class="flex-1 min-w-0">
									<div class="flex items-center gap-2">
										<p class="text-sm font-medium text-neutral-900 truncate">{occurrence.title}</p>
										{#if occurrence.seriesOverride}
											<Badge variant="warning">Modified</Badge>
										{/if}
									</div>
									<p class="mt-0.5 text-xs text-neutral-500">
										{formatDateTime(occurrence.eventDate, series.timezone)}
										{#if occurrence.seriesIndex != null}
											<span class="text-neutral-400 ml-2">#{occurrence.seriesIndex}</span>
										{/if}
									</p>
								</div>
								<Badge variant={statusVariant(occurrence.status)}>{occurrence.status}</Badge>
							</div>
						</a>
					{/each}
				</div>
			{/if}
		</Card>

		<!-- Stop Series Modal -->
		<Modal bind:open={showStopModal} title="Stop Series">
			<p class="text-sm text-neutral-600">
				Are you sure you want to stop <strong>{series.title}</strong>? No new occurrences will be generated, but existing occurrences will remain.
			</p>
			{#snippet actions()}
				<Button variant="outline" size="sm" onclick={() => showStopModal = false}>Keep Running</Button>
				<Button variant="danger" size="sm" onclick={stopSeries} loading={stopping}>Stop Series</Button>
			{/snippet}
		</Modal>

		<!-- Delete Series Modal -->
		<Modal bind:open={showDeleteModal} title="Delete Series">
			<p class="text-sm text-neutral-600">
				Are you sure you want to delete <strong>{series.title}</strong>? The series will be removed, but existing events will remain as standalone events.
			</p>
			{#snippet actions()}
				<Button variant="outline" size="sm" onclick={() => showDeleteModal = false}>Keep Series</Button>
				<Button variant="danger" size="sm" onclick={deleteSeries} loading={deleting}>Delete Series</Button>
			{/snippet}
		</Modal>
	{:else}
		<Card>
			<p class="text-center text-neutral-500 py-8">Series not found.</p>
		</Card>
	{/if}
</AppShell>
