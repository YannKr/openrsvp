<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import { api } from '$lib/api/client';
	import { currentUser } from '$lib/stores/auth';
	import { toast } from '$lib/stores/toast';
	import { smsEnabled, loadAppConfig } from '$lib/stores/config';
	import { getTimezoneOptions, getTimezoneLabel } from '$lib/utils/timezones';
	import type { EventSeries } from '$lib/types';
	import AppShell from '$lib/components/layout/AppShell.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Textarea from '$lib/components/ui/Textarea.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import Card from '$lib/components/ui/Card.svelte';

	const defaultTz = $currentUser?.timezone
		|| Intl.DateTimeFormat().resolvedOptions().timeZone
		|| '';

	let submitting = $state(false);

	// Form fields
	let title = $state('');
	let description = $state('');
	let location = $state('');
	let timezone = $state(defaultTz);
	let startDate = $state('');
	let eventTime = $state('');
	let durationMinutes = $state('');
	let recurrenceRule = $state('weekly');
	let endCondition = $state<'none' | 'count' | 'date'>('none');
	let maxOccurrences = $state('');
	let recurrenceEnd = $state('');
	let contactRequirement = $state('email');
	let showHeadcount = $state(false);
	let showGuestList = $state(false);
	let rsvpDeadlineOffsetHours = $state('');
	let maxCapacity = $state('');
	let retentionDays = $state('30');
	let showRetention = $state(false);

	// Validation
	let errors: Record<string, string> = $state({});

	const tzOptions = getTimezoneOptions(defaultTz);

	const recurrenceOptions = [
		{ value: 'weekly', label: 'Weekly' },
		{ value: 'biweekly', label: 'Every 2 weeks' },
		{ value: 'monthly', label: 'Monthly' }
	];

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

	// Today's date for the min attribute on the date picker
	const today = new Date().toISOString().split('T')[0];

	onMount(() => {
		loadAppConfig();
	});

	function validate(): boolean {
		errors = {};
		if (!title.trim()) errors.title = 'Title is required';
		if (!startDate) errors.startDate = 'Start date is required';
		if (!eventTime) errors.eventTime = 'Event time is required';
		if (!timezone) errors.timezone = 'Timezone is required';

		if (endCondition === 'count') {
			const n = parseInt(maxOccurrences);
			if (isNaN(n) || n < 1) errors.maxOccurrences = 'Must be at least 1';
		}
		if (endCondition === 'date' && !recurrenceEnd) {
			errors.recurrenceEnd = 'End date is required';
		}
		if (durationMinutes) {
			const d = parseInt(durationMinutes);
			if (isNaN(d) || d < 1) errors.durationMinutes = 'Duration must be at least 1 minute';
		}
		if (maxCapacity) {
			const parsed = Number(maxCapacity);
			if (!Number.isInteger(parsed) || parsed < 1) {
				errors.maxCapacity = 'Max attendees must be a whole number of at least 1';
			}
		}
		if (rsvpDeadlineOffsetHours) {
			const h = parseInt(rsvpDeadlineOffsetHours);
			if (isNaN(h) || h < 1) errors.rsvpDeadlineOffsetHours = 'Must be at least 1 hour';
		}
		if (showRetention) {
			const days = parseInt(retentionDays);
			if (isNaN(days) || days < 1 || days > 365) {
				errors.retentionDays = 'Retention days must be between 1 and 365';
			}
		}
		return Object.keys(errors).length === 0;
	}

	async function handleSubmit() {
		if (!validate()) return;

		submitting = true;
		try {
			const body: Record<string, unknown> = {
				title: title.trim(),
				description: description.trim(),
				location: location.trim(),
				timezone,
				startDate,
				eventTime,
				recurrenceRule,
				contactRequirement,
				showHeadcount,
				showGuestList,
				retentionDays: parseInt(retentionDays)
			};
			if (durationMinutes) body.durationMinutes = parseInt(durationMinutes);
			if (endCondition === 'count' && maxOccurrences) body.maxOccurrences = parseInt(maxOccurrences);
			if (endCondition === 'date' && recurrenceEnd) body.recurrenceEnd = new Date(recurrenceEnd + 'T23:59:59').toISOString();
			if (rsvpDeadlineOffsetHours) body.rsvpDeadlineOffsetHours = parseInt(rsvpDeadlineOffsetHours);
			if (maxCapacity) body.maxCapacity = parseInt(maxCapacity);

			const result = await api.post<{ data: EventSeries }>('/events/series', body);
			toast.success('Series created successfully!');
			goto(`/events/series/${result.data.id}`);
		} catch (err: unknown) {
			const apiErr = err as { message?: string };
			toast.error(apiErr.message || 'Failed to create series');
		} finally {
			submitting = false;
		}
	}
</script>

<svelte:head>
	<title>Create Recurring Series -- OpenRSVP</title>
</svelte:head>

<AppShell>
	<div class="max-w-3xl mx-auto">
		<div class="mb-8">
			<a href="/events/series" class="text-sm text-indigo-600 hover:text-indigo-500">&larr; Back to series</a>
			<h1 class="mt-2 text-2xl font-bold text-slate-900">Create Recurring Series</h1>
			<p class="mt-1 text-sm text-slate-500">Set up a template that automatically generates recurring events.</p>
		</div>

		<form
			onsubmit={(e) => { e.preventDefault(); handleSubmit(); }}
		>
			<Card class="mb-6">
				<div class="space-y-6">
					<h2 class="text-lg font-semibold text-slate-900">Event Details</h2>

					<Input
						label="Series Title"
						name="title"
						bind:value={title}
						placeholder="Weekly Team Standup, Monthly Book Club, etc."
						error={errors.title || ''}
						required
					/>

					<Textarea
						label="Description"
						name="description"
						bind:value={description}
						placeholder="Tell your guests what the event is about..."
						rows={4}
					/>

					<Input
						label="Location"
						name="location"
						bind:value={location}
						placeholder="123 Main St, New York, NY"
					/>

					<Select
						label="Timezone"
						name="timezone"
						bind:value={timezone}
						options={tzOptions}
						error={errors.timezone || ''}
						required
					/>

					<div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
						<div class="space-y-1">
							<label for="startDate" class="block text-sm font-medium text-slate-700">
								Start Date <span class="text-red-500">*</span>
							</label>
							<input
								id="startDate"
								type="date"
								bind:value={startDate}
								min={today}
								required
								class="block w-full rounded-lg border px-3 py-2 text-sm shadow-sm transition-colors focus:outline-none focus:ring-2 focus:ring-offset-0 {errors.startDate
									? 'border-red-300 text-red-900 focus:border-red-500 focus:ring-red-500'
									: 'border-slate-300 text-slate-900 focus:border-indigo-500 focus:ring-indigo-500'}"
							/>
							{#if errors.startDate}
								<p class="text-sm text-red-600">{errors.startDate}</p>
							{/if}
						</div>

						<div class="space-y-1">
							<label for="eventTime" class="block text-sm font-medium text-slate-700">
								Event Time <span class="text-red-500">*</span>
							</label>
							<input
								id="eventTime"
								type="time"
								bind:value={eventTime}
								required
								class="block w-full rounded-lg border px-3 py-2 text-sm shadow-sm transition-colors focus:outline-none focus:ring-2 focus:ring-offset-0 {errors.eventTime
									? 'border-red-300 text-red-900 focus:border-red-500 focus:ring-red-500'
									: 'border-slate-300 text-slate-900 focus:border-indigo-500 focus:ring-indigo-500'}"
							/>
							{#if errors.eventTime}
								<p class="text-sm text-red-600">{errors.eventTime}</p>
							{/if}
						</div>

						<Input
							label="Duration (minutes)"
							name="durationMinutes"
							type="number"
							bind:value={durationMinutes}
							placeholder="e.g. 60"
							error={errors.durationMinutes || ''}
						/>
					</div>
				</div>
			</Card>

			<Card class="mb-6">
				<div class="space-y-6">
					<h2 class="text-lg font-semibold text-slate-900">Recurrence</h2>

					<Select
						label="Repeat"
						name="recurrenceRule"
						bind:value={recurrenceRule}
						options={recurrenceOptions}
						required
					/>

					<fieldset>
						<legend class="text-sm font-medium text-slate-700 mb-3">End Condition</legend>
						<div class="space-y-3">
							<label class="flex items-center gap-3 cursor-pointer">
								<input
									type="radio"
									name="endCondition"
									value="none"
									bind:group={endCondition}
									class="text-indigo-600 focus:ring-indigo-500/40"
								/>
								<span class="text-sm text-slate-700">No end date (runs indefinitely)</span>
							</label>
							<label class="flex items-center gap-3 cursor-pointer">
								<input
									type="radio"
									name="endCondition"
									value="count"
									bind:group={endCondition}
									class="text-indigo-600 focus:ring-indigo-500/40"
								/>
								<span class="text-sm text-slate-700">End after a number of occurrences</span>
							</label>
							{#if endCondition === 'count'}
								<div class="ml-7">
									<Input
										name="maxOccurrences"
										type="number"
										bind:value={maxOccurrences}
										placeholder="e.g. 12"
										error={errors.maxOccurrences || ''}
									/>
								</div>
							{/if}
							<label class="flex items-center gap-3 cursor-pointer">
								<input
									type="radio"
									name="endCondition"
									value="date"
									bind:group={endCondition}
									class="text-indigo-600 focus:ring-indigo-500/40"
								/>
								<span class="text-sm text-slate-700">End on a specific date</span>
							</label>
							{#if endCondition === 'date'}
								<div class="ml-7 space-y-1">
									<input
										type="date"
										bind:value={recurrenceEnd}
										min={startDate || today}
										class="block w-full rounded-lg border px-3 py-2 text-sm shadow-sm transition-colors focus:outline-none focus:ring-2 focus:ring-offset-0 {errors.recurrenceEnd
											? 'border-red-300 text-red-900 focus:border-red-500 focus:ring-red-500'
											: 'border-slate-300 text-slate-900 focus:border-indigo-500 focus:ring-indigo-500'}"
									/>
									{#if errors.recurrenceEnd}
										<p class="text-sm text-red-600">{errors.recurrenceEnd}</p>
									{/if}
								</div>
							{/if}
						</div>
					</fieldset>
				</div>
			</Card>

			<Card class="mb-6">
				<div class="space-y-6">
					<h2 class="text-lg font-semibold text-slate-900">RSVP Settings</h2>

					<Select
						label="RSVP Contact Requirement"
						name="contactRequirement"
						bind:value={contactRequirement}
						options={filteredContactOptions}
					/>

					<fieldset class="pt-2">
						<legend class="text-sm font-medium text-slate-700 mb-3">Guest Visibility</legend>
						<p class="text-xs text-slate-400 mb-3">Control what attendance info is shown on the public invite page.</p>
						<div class="space-y-2">
							<label class="flex items-center gap-3 cursor-pointer">
								<input
									type="checkbox"
									bind:checked={showHeadcount}
									class="rounded border-slate-300 text-indigo-600 focus:ring-indigo-500/40"
								/>
								<span class="text-sm text-slate-700">Show attendance count</span>
							</label>
							<label class="flex items-center gap-3 cursor-pointer">
								<input
									type="checkbox"
									bind:checked={showGuestList}
									class="rounded border-slate-300 text-indigo-600 focus:ring-indigo-500/40"
								/>
								<span class="text-sm text-slate-700">Show guest names</span>
							</label>
						</div>
					</fieldset>

					<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
						<Input
							label="RSVP Deadline Offset (hours before event)"
							name="rsvpDeadlineOffsetHours"
							type="number"
							bind:value={rsvpDeadlineOffsetHours}
							placeholder="e.g. 24"
							helper="How many hours before each occurrence RSVPs close."
							error={errors.rsvpDeadlineOffsetHours || ''}
						/>
						<Input
							label="Max Attendees (optional)"
							name="maxCapacity"
							type="number"
							bind:value={maxCapacity}
							placeholder="Leave empty for unlimited"
							helper="Applied to each occurrence."
							error={errors.maxCapacity || ''}
						/>
					</div>

					<div class="pt-2">
						{#if showRetention}
							<Input
								label="Data Retention (days)"
								name="retentionDays"
								type="number"
								bind:value={retentionDays}
								helper="Guest data is automatically deleted this many days after each occurrence (1-365)."
								error={errors.retentionDays || ''}
							/>
						{:else}
							<p class="text-xs text-slate-400">
								Guest data will be automatically deleted 30 days after each occurrence.
								<button
									type="button"
									class="text-indigo-500 hover:text-indigo-600 underline underline-offset-2"
									onclick={() => (showRetention = true)}
								>
									Specify custom data retention
								</button>
							</p>
						{/if}
					</div>
				</div>
			</Card>

			<div class="flex items-center justify-end gap-3">
				<Button variant="outline" href="/events/series">Cancel</Button>
				<Button type="submit" loading={submitting}>Create Series</Button>
			</div>
		</form>
	</div>
</AppShell>
