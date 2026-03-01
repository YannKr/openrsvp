<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { api } from '$lib/api/client';
	import { toast } from '$lib/stores/toast';
	import { toISOLocal } from '$lib/utils/dates';
	import { getTimezoneOptions } from '$lib/utils/timezones';
	import type { Event } from '$lib/types';
	import AppShell from '$lib/components/layout/AppShell.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Textarea from '$lib/components/ui/Textarea.svelte';
	import DateTimePicker from '$lib/components/ui/DateTimePicker.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import Spinner from '$lib/components/ui/Spinner.svelte';
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
	let retentionDays = $state('30');
	let showRetention = $state(false);

	const contactRequirementOptions = [
		{ value: 'email_or_phone', label: 'Email or Phone (at least one)' },
		{ value: 'email', label: 'Email only' },
		{ value: 'phone', label: 'Phone only' },
		{ value: 'email_and_phone', label: 'Email and Phone (both required)' }
	];

	let errors: Record<string, string> = $state({});

	let tzOptions = getTimezoneOptions();

	onMount(async () => {
		try {
			const result = await api.get<{ data: Event }>(`/events/${eventId}`);
			const e = result.data;
			title = e.title;
			eventDate = e.eventDate ? toISOLocal(new Date(e.eventDate)) : '';
			endDate = e.endDate ? toISOLocal(new Date(e.endDate)) : '';
			location = e.location;
			timezone = e.timezone;
			tzOptions = getTimezoneOptions(e.timezone);
			description = e.description;
			contactRequirement = e.contactRequirement || 'email_or_phone';
			retentionDays = String(e.retentionDays);
			showRetention = e.retentionDays !== 30;
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
		return Object.keys(errors).length === 0;
	}

	async function handleSave() {
		if (!validate()) return;

		saving = true;
		try {
			const body: Record<string, unknown> = {
				title: title.trim(),
				eventDate,
				location: location.trim(),
				timezone,
				description: description.trim(),
				contactRequirement,
				retentionDays: parseInt(retentionDays)
			};
			if (endDate) body.endDate = endDate;

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
			<a href="/events/{eventId}" class="text-sm text-indigo-600 hover:text-indigo-500">&larr; Back to event</a>
			<h1 class="mt-2 text-2xl font-bold text-slate-900">Edit Event</h1>
		</div>

		{#if loading}
			<div class="flex items-center justify-center py-16">
				<Spinner size="lg" class="text-indigo-500" />
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
						options={contactRequirementOptions}
					/>

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

					<div class="flex items-center justify-end gap-3 pt-4 border-t border-slate-200">
						<Button variant="outline" href="/events/{eventId}">Cancel</Button>
						<Button type="submit" loading={saving}>Save Changes</Button>
					</div>
				</form>
			</Card>
		{/if}
	</div>
</AppShell>
