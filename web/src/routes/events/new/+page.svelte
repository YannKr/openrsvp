<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import { api } from '$lib/api/client';
	import { currentUser } from '$lib/stores/auth';
	import { toast } from '$lib/stores/toast';
	import { smsEnabled, loadAppConfig } from '$lib/stores/config';
	import { toISOLocal } from '$lib/utils/dates';
	import { getTimezoneOptions, getTimezoneLabel } from '$lib/utils/timezones';
	import type { Event } from '$lib/types';
	import AppShell from '$lib/components/layout/AppShell.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Textarea from '$lib/components/ui/Textarea.svelte';
	import DateTimePicker from '$lib/components/ui/DateTimePicker.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import Card from '$lib/components/ui/Card.svelte';

	// Auto-fill timezone from profile or browser detection.
	const defaultTz = $currentUser?.timezone
		|| Intl.DateTimeFormat().resolvedOptions().timeZone
		|| '';

	let step = $state(1);
	let submitting = $state(false);

	// Step 1 fields
	let title = $state('');
	let eventDate = $state('');
	let endDate = $state('');
	let location = $state('');
	let timezone = $state(defaultTz);

	// Step 2 fields
	let description = $state('');
	let contactRequirement = $state('email');
	let showHeadcount = $state(false);
	let showGuestList = $state(false);
	let rsvpDeadline = $state('');
	let maxCapacity = $state('');
	let retentionDays = $state('30');
	let showRetention = $state(false);

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

	onMount(() => {
		loadAppConfig();
	});

	// Validation errors
	let errors: Record<string, string> = $state({});

	const tzOptions = getTimezoneOptions(defaultTz);

	const minDate = toISOLocal(new Date());

	function validateStep1(): boolean {
		errors = {};
		if (!title.trim()) errors.title = 'Title is required';
		if (!eventDate) errors.eventDate = 'Event date is required';
		if (!timezone) errors.timezone = 'Timezone is required';
		return Object.keys(errors).length === 0;
	}

	function validateStep2(): boolean {
		errors = {};
		if (showRetention) {
			const days = parseInt(retentionDays);
			if (isNaN(days) || days < 1 || days > 365) {
				errors.retentionDays = 'Retention days must be between 1 and 365';
			}
		}
		if (maxCapacity && (parseInt(maxCapacity) < 1 || isNaN(parseInt(maxCapacity)))) {
			errors.maxCapacity = 'Max attendees must be at least 1';
		}
		return Object.keys(errors).length === 0;
	}

	function nextStep() {
		if (step === 1 && validateStep1()) {
			step = 2;
		} else if (step === 2 && validateStep2()) {
			step = 3;
		}
	}

	function prevStep() {
		if (step > 1) step -= 1;
	}

	async function handleSubmit() {
		submitting = true;
		try {
			const body: Record<string, unknown> = {
				title: title.trim(),
				eventDate,
				location: location.trim(),
				timezone,
				description: description.trim(),
				contactRequirement,
				showHeadcount,
				showGuestList,
				retentionDays: parseInt(retentionDays)
			};
			if (endDate) body.endDate = endDate;
			if (rsvpDeadline) body.rsvpDeadline = rsvpDeadline;
			if (maxCapacity) body.maxCapacity = parseInt(maxCapacity);

			const result = await api.post<{ data: Event }>('/events', body);
			toast.success('Event created successfully!');
			goto(`/events/${result.data.id}/invite`);
		} catch (err: unknown) {
			const apiErr = err as { message?: string };
			toast.error(apiErr.message || 'Failed to create event');
		} finally {
			submitting = false;
		}
	}
</script>

<svelte:head>
	<title>Create Event -- OpenRSVP</title>
</svelte:head>

<AppShell>
	<div class="max-w-3xl mx-auto">
		<div class="mb-8">
			<a href="/events" class="text-sm text-indigo-600 hover:text-indigo-500">&larr; Back to events</a>
			<h1 class="mt-2 text-2xl font-bold text-slate-900">Create New Event</h1>
		</div>

		<!-- Step indicator -->
		<div class="mb-8">
			<div class="flex items-center">
				{#each [1, 2, 3] as s}
					<div class="flex items-center {s < 3 ? 'flex-1' : ''}">
						<div
							class="flex h-8 w-8 items-center justify-center rounded-full text-sm font-medium {s <= step
								? 'bg-indigo-600 text-white'
								: 'bg-slate-200 text-slate-600'}"
						>
							{s}
						</div>
						{#if s < 3}
							<div class="flex-1 mx-2 h-0.5 {s < step ? 'bg-indigo-600' : 'bg-slate-200'}"></div>
						{/if}
					</div>
				{/each}
			</div>
			<div class="flex justify-between mt-2 text-xs text-slate-500">
				<span>Details</span>
				<span>Description</span>
				<span>Review</span>
			</div>
		</div>

		<Card>
			{#if step === 1}
				<div class="space-y-6">
					<h2 class="text-lg font-semibold text-slate-900">Event Details</h2>

					<Input
						label="Event Title"
						name="title"
						bind:value={title}
						placeholder="Birthday Party, Team Lunch, etc."
						error={errors.title || ''}
						required
					/>

					<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
						<DateTimePicker
							label="Event Date"
							name="eventDate"
							bind:value={eventDate}
							min={minDate}
							error={errors.eventDate || ''}
							required
						/>
						<DateTimePicker
							label="End Date (optional)"
							name="endDate"
							bind:value={endDate}
							min={eventDate || minDate}
						/>
					</div>

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
				</div>

			{:else if step === 2}
				<div class="space-y-6">
					<h2 class="text-lg font-semibold text-slate-900">Description & Settings</h2>

					<Textarea
						label="Description"
						name="description"
						bind:value={description}
						placeholder="Tell your guests what the event is about..."
						rows={6}
					/>

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
						<DateTimePicker
							label="RSVP Deadline (optional)"
							name="rsvpDeadline"
							bind:value={rsvpDeadline}
							min={minDate}
							max={eventDate || undefined}
							helper="Guests won't be able to RSVP or change their response after this date."
						/>
						<Input
							label="Max Attendees (optional)"
							name="maxCapacity"
							type="number"
							bind:value={maxCapacity}
							placeholder="Leave empty for unlimited"
							helper="Total headcount including plus-ones. Leave empty for no limit."
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
								helper="Guest data is automatically deleted this many days after the event (1-365)."
								error={errors.retentionDays || ''}
							/>
						{:else}
							<p class="text-xs text-slate-400">
								Guest data will be automatically deleted 30 days after the event.
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

			{:else if step === 3}
				<div class="space-y-6">
					<h2 class="text-lg font-semibold text-slate-900">Review Your Event</h2>

					<dl class="divide-y divide-slate-200">
						<div class="py-3 sm:grid sm:grid-cols-3 sm:gap-4">
							<dt class="text-sm font-medium text-slate-500">Title</dt>
							<dd class="mt-1 text-sm text-slate-900 sm:col-span-2 sm:mt-0">{title}</dd>
						</div>
						<div class="py-3 sm:grid sm:grid-cols-3 sm:gap-4">
							<dt class="text-sm font-medium text-slate-500">Event Date</dt>
							<dd class="mt-1 text-sm text-slate-900 sm:col-span-2 sm:mt-0">{eventDate || 'Not set'}</dd>
						</div>
						{#if endDate}
							<div class="py-3 sm:grid sm:grid-cols-3 sm:gap-4">
								<dt class="text-sm font-medium text-slate-500">End Date</dt>
								<dd class="mt-1 text-sm text-slate-900 sm:col-span-2 sm:mt-0">{endDate}</dd>
							</div>
						{/if}
						<div class="py-3 sm:grid sm:grid-cols-3 sm:gap-4">
							<dt class="text-sm font-medium text-slate-500">Location</dt>
							<dd class="mt-1 text-sm text-slate-900 sm:col-span-2 sm:mt-0">{location || 'Not specified'}</dd>
						</div>
						<div class="py-3 sm:grid sm:grid-cols-3 sm:gap-4">
							<dt class="text-sm font-medium text-slate-500">Timezone</dt>
							<dd class="mt-1 text-sm text-slate-900 sm:col-span-2 sm:mt-0">{getTimezoneLabel(timezone)}</dd>
						</div>
						{#if description}
							<div class="py-3 sm:grid sm:grid-cols-3 sm:gap-4">
								<dt class="text-sm font-medium text-slate-500">Description</dt>
								<dd class="mt-1 text-sm text-slate-900 sm:col-span-2 sm:mt-0 whitespace-pre-wrap">{description}</dd>
							</div>
						{/if}
						{#if contactRequirement !== 'email_or_phone'}
							<div class="py-3 sm:grid sm:grid-cols-3 sm:gap-4">
								<dt class="text-sm font-medium text-slate-500">Contact Requirement</dt>
								<dd class="mt-1 text-sm text-slate-900 sm:col-span-2 sm:mt-0">{contactRequirementOptions.find(o => o.value === contactRequirement)?.label}</dd>
							</div>
						{/if}
						{#if showHeadcount || showGuestList}
							<div class="py-3 sm:grid sm:grid-cols-3 sm:gap-4">
								<dt class="text-sm font-medium text-slate-500">Guest Visibility</dt>
								<dd class="mt-1 text-sm text-slate-900 sm:col-span-2 sm:mt-0">
									{#if showHeadcount && showGuestList}
										Attendance count and guest names visible
									{:else if showHeadcount}
										Attendance count visible
									{:else}
										Guest names visible
									{/if}
								</dd>
							</div>
						{/if}
						{#if rsvpDeadline}
							<div class="py-3 sm:grid sm:grid-cols-3 sm:gap-4">
								<dt class="text-sm font-medium text-slate-500">RSVP Deadline</dt>
								<dd class="mt-1 text-sm text-slate-900 sm:col-span-2 sm:mt-0">{rsvpDeadline}</dd>
							</div>
						{/if}
						{#if maxCapacity}
							<div class="py-3 sm:grid sm:grid-cols-3 sm:gap-4">
								<dt class="text-sm font-medium text-slate-500">Max Attendees</dt>
								<dd class="mt-1 text-sm text-slate-900 sm:col-span-2 sm:mt-0">{maxCapacity}</dd>
							</div>
						{/if}
						{#if retentionDays !== '30'}
							<div class="py-3 sm:grid sm:grid-cols-3 sm:gap-4">
								<dt class="text-sm font-medium text-slate-500">Retention</dt>
								<dd class="mt-1 text-sm text-slate-900 sm:col-span-2 sm:mt-0">{retentionDays} days</dd>
							</div>
						{/if}
					</dl>
				</div>
			{/if}

			<!-- Navigation buttons -->
			<div class="mt-8 flex items-center justify-between border-t border-slate-200 pt-6">
				<div>
					{#if step > 1}
						<Button variant="outline" onclick={prevStep}>Back</Button>
					{/if}
				</div>
				<div>
					{#if step < 3}
						<Button onclick={nextStep}>Next</Button>
					{:else}
						<Button onclick={handleSubmit} loading={submitting}>Create Event</Button>
					{/if}
				</div>
			</div>
		</Card>
	</div>
</AppShell>
