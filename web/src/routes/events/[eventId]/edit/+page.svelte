<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { api } from '$lib/api/client';
	import { toast } from '$lib/stores/toast';
	import { smsEnabled, loadAppConfig } from '$lib/stores/config';
	import { toISOLocal, datetimeLocalToUTC, utcToDatetimeLocal } from '$lib/utils/dates';
	import { getTimezoneOptions } from '$lib/utils/timezones';
	import type { Event, RSVPStats } from '$lib/types';
	import AppShell from '$lib/components/layout/AppShell.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Textarea from '$lib/components/ui/Textarea.svelte';
	import DateTimePicker from '$lib/components/ui/DateTimePicker.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import Spinner from '$lib/components/ui/Spinner.svelte';
	import QuestionBuilder from '$lib/components/questions/QuestionBuilder.svelte';
	import { onMount } from 'svelte';

	const eventId = $derived($page.params.eventId);

	let loading = $state(true);
	let saving = $state(false);

	let title = $state('');
	let eventDate = $state('');
	let endDate = $state('');
	let location = $state('');
	let timezone = $state('');
	let description = $state('');
	let contactRequirement = $state('email_or_phone');
	let showHeadcount = $state(false);
	let showGuestList = $state(false);
	let rsvpDeadline = $state('');
	let maxCapacity = $state('');
	let retentionDays = $state('30');
	let showRetention = $state(false);
	let attendingHeadcount = $state(0);
	let waitlistEnabled = $state(false);

	const capacityWarning = $derived(
		maxCapacity && parseInt(maxCapacity) > 0 && attendingHeadcount > parseInt(maxCapacity)
	);

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

	let errors: Record<string, string> = $state({});

	let tzOptions = $state(getTimezoneOptions());

	onMount(async () => {
		loadAppConfig();
		try {
			const [eventResult, statsResult] = await Promise.all([
				api.get<{ data: Event }>(`/events/${eventId}`),
				api.get<{ data: RSVPStats }>(`/rsvp/event/${eventId}/stats`).catch(() => ({
					data: { attending: 0, attendingHeadcount: 0, maybe: 0, maybeHeadcount: 0, declined: 0, pending: 0, waitlisted: 0, total: 0, totalHeadcount: 0 }
				}))
			]);
			const e = eventResult.data;
			title = e.title;
			eventDate = e.eventDate ? utcToDatetimeLocal(e.eventDate, e.timezone) : '';
			endDate = e.endDate ? utcToDatetimeLocal(e.endDate, e.timezone) : '';
			location = e.location;
			timezone = e.timezone;
			tzOptions = getTimezoneOptions(e.timezone);
			description = e.description;
			contactRequirement = e.contactRequirement || 'email_or_phone';
			showHeadcount = e.showHeadcount ?? false;
			showGuestList = e.showGuestList ?? false;
			rsvpDeadline = e.rsvpDeadline ? utcToDatetimeLocal(e.rsvpDeadline, e.timezone) : '';
			maxCapacity = e.maxCapacity ? String(e.maxCapacity) : '';
			retentionDays = String(e.retentionDays);
			showRetention = e.retentionDays !== 30;
			waitlistEnabled = e.waitlistEnabled ?? false;
			attendingHeadcount = statsResult.data.attendingHeadcount;
		} catch (err: unknown) {
			const apiErr = err as { message?: string };
			toast.error(apiErr.message || 'Failed to load event');
		} finally {
			loading = false;
		}
	});

	function validate(): boolean {
		errors = {};
		if (!title.trim()) errors.title = 'Title is required';
		if (!eventDate) errors.eventDate = 'Event date is required';
		if (!timezone) errors.timezone = 'Timezone is required';
		if (showRetention) {
			const days = parseInt(retentionDays);
			if (isNaN(days) || days < 1 || days > 365) {
				errors.retentionDays = 'Retention days must be between 1 and 365';
			}
		}
		if (maxCapacity) {
			const parsed = Number(maxCapacity);
			if (!Number.isInteger(parsed) || parsed < 1) {
				errors.maxCapacity = 'Max attendees must be a whole number of at least 1';
			}
		}
		return Object.keys(errors).length === 0;
	}

	async function handleSave() {
		if (!validate()) return;

		saving = true;
		try {
			const body: Record<string, unknown> = {
				title: title.trim(),
				eventDate: eventDate ? datetimeLocalToUTC(eventDate, timezone) : eventDate,
				location: location.trim(),
				timezone,
				description: description.trim(),
				contactRequirement,
				showHeadcount,
				showGuestList,
				retentionDays: parseInt(retentionDays)
			};
			if (endDate) body.endDate = datetimeLocalToUTC(endDate, timezone);
			if (rsvpDeadline) body.rsvpDeadline = datetimeLocalToUTC(rsvpDeadline, timezone);
			else body.rsvpDeadline = '';
			if (maxCapacity) {
				body.maxCapacity = parseInt(maxCapacity);
				body.waitlistEnabled = waitlistEnabled;
			} else {
				body.maxCapacity = 0;
				body.waitlistEnabled = false;
			}

			await api.put(`/events/${eventId}`, body);
			toast.success('Event updated successfully');
			goto(`/events/${eventId}`);
		} catch (err: unknown) {
			const apiErr = err as { message?: string };
			toast.error(apiErr.message || 'Failed to update event');
		} finally {
			saving = false;
		}
	}
</script>

<svelte:head>
	<title>Edit Event -- OpenRSVP</title>
</svelte:head>

<AppShell>
	<div class="max-w-3xl mx-auto">
		<div class="mb-8">
			<a href="/events/{eventId}" class="text-sm text-primary hover:text-primary-hover">&larr; Back to event</a>
			<h1 class="mt-2 text-2xl font-bold font-display text-neutral-900">Edit Event</h1>
		</div>

		{#if loading}
			<div class="flex items-center justify-center py-16">
				<Spinner size="lg" class="text-primary" />
			</div>
		{:else}
			<Card>
				<form
					onsubmit={(e) => {
						e.preventDefault();
						handleSave();
					}}
					class="space-y-6"
				>
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
							error={errors.eventDate || ''}
							required
						/>
						<DateTimePicker
							label="End Date (optional)"
							name="endDate"
							bind:value={endDate}
							min={eventDate}
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
						<legend class="text-sm font-medium text-neutral-700 mb-3">Guest Visibility</legend>
						<p class="text-xs text-neutral-400 mb-3">Control what attendance info is shown on the public invite page.</p>
						<div class="space-y-2">
							<label class="flex items-center gap-3 cursor-pointer">
								<input
									type="checkbox"
									bind:checked={showHeadcount}
									class="rounded border-neutral-300 text-primary focus:ring-primary/40"
								/>
								<span class="text-sm text-neutral-700">Show attendance count</span>
							</label>
							<label class="flex items-center gap-3 cursor-pointer">
								<input
									type="checkbox"
									bind:checked={showGuestList}
									class="rounded border-neutral-300 text-primary focus:ring-primary/40"
								/>
								<span class="text-sm text-neutral-700">Show guest names</span>
							</label>
						</div>
					</fieldset>

					<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
						<DateTimePicker
							label="RSVP Deadline (optional)"
							name="rsvpDeadline"
							bind:value={rsvpDeadline}
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

					{#if capacityWarning}
						<div class="rounded-lg bg-warning-light border border-warning px-4 py-3 text-sm text-warning flex items-start gap-2">
							<svg class="h-4 w-4 text-warning mt-0.5 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
								<path stroke-linecap="round" stroke-linejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
							</svg>
							<span>
								Warning: Current attending headcount ({attendingHeadcount}) exceeds this limit. Existing RSVPs will not be removed, but no new attending RSVPs will be accepted.
							</span>
						</div>
					{/if}

					{#if maxCapacity}
						<label class="flex items-center gap-3 cursor-pointer">
							<input
								type="checkbox"
								bind:checked={waitlistEnabled}
								class="rounded border-neutral-300 text-primary focus:ring-primary/40"
							/>
							<div>
								<span class="text-sm text-neutral-700">Enable waitlist</span>
								<p class="text-xs text-neutral-400">When at capacity, guests can join a waitlist instead of being turned away.</p>
							</div>
						</label>
					{/if}

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
							<p class="text-xs text-neutral-400">
								Guest data will be automatically deleted 30 days after the event.
								<button
									type="button"
									class="text-primary hover:text-primary-hover underline underline-offset-2"
									onclick={() => (showRetention = true)}
								>
									Specify custom data retention
								</button>
							</p>
						{/if}
					</div>

					<div class="flex items-center justify-end gap-3 pt-4 border-t border-neutral-200">
						<Button variant="outline" href="/events/{eventId}">Cancel</Button>
						<Button type="submit" loading={saving}>Save Changes</Button>
					</div>
				</form>
			</Card>

			<Card class="mt-6">
				<QuestionBuilder eventId={eventId ?? ''} />
			</Card>
		{/if}
	</div>
</AppShell>
