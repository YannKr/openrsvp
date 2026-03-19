<script lang="ts">
	import { page } from '$app/stores';
	import { onMount } from 'svelte';
	import { api } from '$lib/api/client';
	import type { PublicEvent, Attendee, Message, PublicAttendance, EventQuestion, QuestionAnswer, ApiError } from '$lib/types';
	import QuestionRenderer from '$lib/components/questions/QuestionRenderer.svelte';
	import AddToCalendar from '$lib/components/ui/AddToCalendar.svelte';

	interface RsvpData {
		attendee: Attendee;
		event: PublicEvent;
		attendance?: PublicAttendance;
		shareToken?: string;
		questions?: EventQuestion[];
		answers?: QuestionAnswer[];
		waitlistPosition?: number;
	}

	let loading = $state(true);
	let error = $state('');
	let attendee = $state<Attendee | null>(null);
	let eventData = $state<PublicEvent | null>(null);
	let attendance = $state<PublicAttendance | null>(null);
	let shareToken = $state('');
	let showAllNames = $state(false);
	let eventQuestions = $state<EventQuestion[]>([]);
	let editAnswers: Record<string, string> = $state({});
	let waitlistPosition = $state<number | null>(null);
	const displayNames = $derived(
		attendance?.names
			? (showAllNames ? attendance.names : attendance.names.slice(0, 50))
			: []
	);

	// Edit form
	let editing = $state(false);
	let editName = $state('');
	let editStatus = $state<'attending' | 'maybe' | 'declined'>('attending');
	let editDietary = $state('');
	let editPlusOnes = $state(0);
	let saving = $state(false);
	let saveError = $state('');
	let saveSuccess = $state(false);

	// Message form
	let showMessageForm = $state(false);
	let msgSubject = $state('');
	let msgBody = $state('');
	let sendingMessage = $state(false);
	let messageError = $state('');
	let messageSent = $state(false);

	// Messages list
	let messages = $state<Message[]>([]);
	let loadingMessages = $state(false);

	const token = $derived($page.params.token);

	// RSVP closed check
	const rsvpsClosed = $derived(eventData?.rsvpsClosed === true);

	// When at capacity and user is NOT already attending, disable the attending option.
	const attendingDisabled = $derived(
		eventData?.atCapacity === true && attendee?.rsvpStatus !== 'attending'
	);

	onMount(async () => {
		await loadRsvp();
		loadMessages();
	});

	async function loadRsvp() {
		try {
			const result = await api.get<{ data: RsvpData }>(`/rsvp/public/token/${token}`);
			attendee = result.data.attendee;
			eventData = result.data.event;
			attendance = result.data.attendance ?? null;
			shareToken = result.data.shareToken ?? '';
			eventQuestions = result.data.questions ?? [];
			waitlistPosition = result.data.waitlistPosition ?? null;
			// Pre-fill answers from previous submission
			if (result.data.answers && result.data.answers.length > 0) {
				const answerMap: Record<string, string> = {};
				for (const a of result.data.answers) {
					answerMap[a.questionId] = a.answer;
				}
				editAnswers = answerMap;
			}
			populateEditForm();
		} catch (err) {
			const apiErr = err as ApiError;
			if (apiErr.status === 404) {
				error = 'This RSVP could not be found. The link may be invalid or expired.';
			} else {
				error = apiErr.message || 'Failed to load your RSVP. Please try again later.';
			}
		} finally {
			loading = false;
		}
	}

	function populateEditForm() {
		if (!attendee) return;
		editName = attendee.name;
		editStatus = (attendee.rsvpStatus === 'pending' || attendee.rsvpStatus === 'waitlisted') ? 'attending' : attendee.rsvpStatus;
		editDietary = attendee.dietaryNotes || '';
		editPlusOnes = attendee.plusOnes;
	}

	async function handleSave(e: SubmitEvent) {
		e.preventDefault();
		if (!editName.trim()) {
			saveError = 'Name is required.';
			return;
		}

		saving = true;
		saveError = '';
		saveSuccess = false;

		try {
			const payload: Record<string, unknown> = {
				name: editName.trim(),
				rsvpStatus: editStatus,
				dietaryNotes: editDietary.trim() || undefined,
				plusOnes: editPlusOnes
			};
			if (Object.keys(editAnswers).length > 0) {
				payload.answers = editAnswers;
			}
			const result = await api.patch<{ data: Attendee }>(`/rsvp/public/token/${token}`, payload);
			attendee = result.data;
			saveSuccess = true;
			editing = false;
			setTimeout(() => { saveSuccess = false; }, 4000);

			// Re-fetch attendance data to reflect changes.
			try {
				const refreshed = await api.get<{ data: RsvpData }>(`/rsvp/public/token/${token}`);
				attendance = refreshed.data.attendance ?? null;
			} catch {
				// Non-critical; attendance display will use previous data.
			}
		} catch (err) {
			const apiErr = err as ApiError;
			saveError = apiErr.message || 'Failed to update RSVP. Please try again.';
		} finally {
			saving = false;
		}
	}

	async function loadMessages() {
		loadingMessages = true;
		try {
			const result = await api.get<{ data: Message[] }>(`/messages/attendee/${token}`);
			messages = result.data;
		} catch {
			// Messages may not be available; silently ignore
		} finally {
			loadingMessages = false;
		}
	}

	async function handleSendMessage(e: SubmitEvent) {
		e.preventDefault();
		if (!msgSubject.trim() || !msgBody.trim()) {
			messageError = 'Please fill in both the subject and message.';
			return;
		}

		sendingMessage = true;
		messageError = '';
		messageSent = false;

		try {
			await api.post(`/messages/attendee/${token}`, {
				subject: msgSubject.trim(),
				body: msgBody.trim()
			});
			messageSent = true;
			msgSubject = '';
			msgBody = '';
			loadMessages();
			setTimeout(() => { messageSent = false; }, 4000);
		} catch (err) {
			const apiErr = err as ApiError;
			messageError = apiErr.message || 'Failed to send message. Please try again.';
		} finally {
			sendingMessage = false;
		}
	}

	function formatDate(dateStr: string, timezone?: string): string {
		if (!dateStr) return '';
		try {
			const date = new Date(dateStr);
			const opts: Intl.DateTimeFormatOptions = {
				weekday: 'long',
				year: 'numeric',
				month: 'long',
				day: 'numeric',
				hour: 'numeric',
				minute: '2-digit'
			};
			if (timezone) opts.timeZone = timezone;
			return date.toLocaleDateString('en-US', opts);
		} catch {
			return dateStr;
		}
	}

	function formatMessageDate(dateStr: string): string {
		if (!dateStr) return '';
		try {
			const date = new Date(dateStr);
			return date.toLocaleDateString('en-US', {
				month: 'short',
				day: 'numeric',
				hour: 'numeric',
				minute: '2-digit'
			});
		} catch {
			return dateStr;
		}
	}

	function statusBadgeClass(status: string): string {
		switch (status) {
			case 'attending': return 'bg-success-light text-success border-success/20';
			case 'maybe': return 'bg-warning-light text-warning border-warning/20';
			case 'declined': return 'bg-error-light text-error border-error/20';
			case 'waitlisted': return 'bg-info-light text-info border-info/20';
			default: return 'bg-neutral-50 text-neutral-700 border-neutral-200';
		}
	}

	function statusLabel(status: string): string {
		switch (status) {
			case 'attending': return "Attending";
			case 'maybe': return 'Maybe';
			case 'declined': return 'Declined';
			case 'pending': return 'Pending';
			case 'waitlisted': return 'Waitlisted';
			default: return status;
		}
	}

	// Waitlist: leave waitlist handler
	let leavingWaitlist = $state(false);
	async function leaveWaitlist() {
		leavingWaitlist = true;
		try {
			const result = await api.patch<{ data: Attendee }>(`/rsvp/public/token/${token}`, {
				rsvpStatus: 'declined'
			});
			attendee = result.data;
			waitlistPosition = null;
			saveSuccess = true;
			setTimeout(() => { saveSuccess = false; }, 4000);
		} catch (err) {
			const apiErr = err as ApiError;
			saveError = apiErr.message || 'Failed to leave waitlist. Please try again.';
		} finally {
			leavingWaitlist = false;
		}
	}
</script>

<svelte:head>
	<title>Manage Your RSVP{eventData ? ` — ${eventData.title}` : ''} — OpenRSVP</title>
</svelte:head>

<div class="min-h-screen px-4 py-8 sm:py-12" style="background: linear-gradient(135deg, #FAFAF9 0%, #FFF1F3 50%, #FDE8EC 100%);">
	<div class="max-w-lg mx-auto">
		{#if loading}
			<div class="flex items-center justify-center min-h-[60vh]">
				<div class="flex flex-col items-center gap-4">
					<div class="animate-spin rounded-full h-10 w-10 border-b-2 border-primary"></div>
					<p class="text-neutral-500 text-sm">Loading your RSVP...</p>
				</div>
			</div>
		{:else if error}
			<div class="flex items-center justify-center min-h-[60vh]">
				<div class="bg-surface rounded-xl shadow-lg border border-neutral-200 p-8 max-w-md text-center">
					<div class="w-16 h-16 rounded-full bg-error-light flex items-center justify-center mx-auto mb-4">
						<svg class="w-8 h-8 text-error" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
							<path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m9-.75a9 9 0 11-18 0 9 9 0 0118 0zm-9 3.75h.008v.008H12v-.008z" />
						</svg>
					</div>
					<h2 class="font-display text-xl font-semibold text-neutral-900 mb-2">RSVP Not Found</h2>
					<p class="text-neutral-600">{error}</p>
				</div>
			</div>
		{:else if attendee && eventData}
			<!-- Event Info Header -->
			<div class="text-center mb-6">
				<h1 class="font-display text-2xl sm:text-3xl font-bold text-neutral-900 mb-1">{eventData.title}</h1>
				<p class="text-neutral-500 text-sm">{formatDate(eventData.eventDate, eventData.timezone)}</p>
				{#if eventData.location}
					<p class="text-neutral-500 text-sm">{eventData.location}</p>
				{/if}
			</div>

			<!-- Add to Calendar (only for attending or maybe) -->
			{#if shareToken && (attendee.rsvpStatus === 'attending' || attendee.rsvpStatus === 'maybe')}
				<div class="mb-6 flex justify-center">
					<AddToCalendar event={eventData} {shareToken} />
				</div>
			{/if}

			<!-- Waitlist Status -->
			{#if attendee.rsvpStatus === 'waitlisted'}
				<div class="mb-4 rounded-md bg-info-light border border-info/20 p-4 text-center">
					<p class="text-sm font-medium text-info">
						{#if waitlistPosition}
							You're on the waitlist -- Position #{waitlistPosition} in line
						{:else}
							You're on the waitlist
						{/if}
					</p>
					<p class="text-xs text-info/80 mt-1">We'll email you if a spot opens up.</p>
					<button
						type="button"
						onclick={leaveWaitlist}
						disabled={leavingWaitlist}
						class="mt-3 inline-flex items-center gap-1.5 rounded-md border border-info/30 bg-surface px-4 py-2 text-sm font-medium text-info hover:bg-info-light transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
					>
						{#if leavingWaitlist}
							Leaving...
						{:else}
							Leave Waitlist
						{/if}
					</button>
				</div>
			{/if}

			<!-- Capacity Notice -->
			{#if eventData.atCapacity && attendee.rsvpStatus !== 'attending' && attendee.rsvpStatus !== 'waitlisted'}
				<div class="mb-4 rounded-md bg-error-light border border-error/20 px-4 py-3 text-sm text-error text-center">
					This event is at capacity. You can still RSVP as "maybe" or "declined".
				</div>
			{/if}

			<!-- RSVP Closed Notice -->
			{#if rsvpsClosed}
				<div class="mb-4 rounded-md bg-warning-light border border-warning/20 px-4 py-3 text-sm text-warning text-center">
					This RSVP can no longer be modified. The RSVP deadline has passed.
				</div>
			{/if}

			<!-- Success Toast -->
			{#if saveSuccess}
				<div class="mb-4 rounded-md bg-success-light border border-success/20 px-4 py-3 text-sm text-success flex items-center gap-2">
					<svg class="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
						<path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
					</svg>
					Your RSVP has been updated successfully.
				</div>
			{/if}

			<!-- RSVP Details Card -->
			<div class="bg-surface rounded-xl shadow-lg border border-neutral-200 p-6 sm:p-8 mb-6">
				<div class="flex items-center justify-between mb-6">
					<h2 class="font-display text-lg font-semibold text-neutral-900">Your RSVP</h2>
					{#if !editing && !rsvpsClosed && attendee.rsvpStatus !== 'waitlisted'}
						<button
							onclick={() => { populateEditForm(); editing = true; }}
							class="inline-flex items-center gap-1.5 text-sm font-medium text-primary hover:text-primary-hover transition-colors"
						>
							<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
								<path stroke-linecap="round" stroke-linejoin="round" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
							</svg>
							Edit
						</button>
					{/if}
				</div>

				{#if editing}
					<form onsubmit={handleSave} class="space-y-5">
						<!-- Name -->
						<div>
							<label for="edit-name" class="block text-sm font-medium text-neutral-700 mb-1.5">
								Name <span class="text-error">*</span>
							</label>
							<input
								id="edit-name"
								type="text"
								required
								bind:value={editName}
								class="w-full rounded-md border border-neutral-300 px-4 py-2.5 text-neutral-900 focus:outline-none focus:ring-2 focus:ring-primary/40 focus:border-primary transition-colors"
							/>
						</div>

						<!-- RSVP Status -->
						<fieldset>
							<legend class="block text-sm font-medium text-neutral-700 mb-3">
								Will you attend?
							</legend>
							<div class="grid grid-cols-3 gap-3">
								<label
									class="rsvp-option {attendingDisabled ? 'rsvp-option-disabled' : ''}"
									class:rsvp-option-selected={editStatus === 'attending'}
									class:rsvp-option-attending={editStatus === 'attending'}
								>
									<input type="radio" name="editStatus" value="attending" bind:group={editStatus} class="sr-only" disabled={attendingDisabled} />
									<svg class="w-5 h-5 mb-1" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
										<path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
									</svg>
									<span class="text-xs sm:text-sm font-medium">Attending</span>
									{#if attendingDisabled}
										<span class="text-[10px] text-error mt-0.5">Full</span>
									{/if}
								</label>
								<label class="rsvp-option" class:rsvp-option-selected={editStatus === 'maybe'} class:rsvp-option-maybe={editStatus === 'maybe'}>
									<input type="radio" name="editStatus" value="maybe" bind:group={editStatus} class="sr-only" />
									<svg class="w-5 h-5 mb-1" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
										<path stroke-linecap="round" stroke-linejoin="round" d="M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
									</svg>
									<span class="text-xs sm:text-sm font-medium">Maybe</span>
								</label>
								<label class="rsvp-option" class:rsvp-option-selected={editStatus === 'declined'} class:rsvp-option-declined={editStatus === 'declined'}>
									<input type="radio" name="editStatus" value="declined" bind:group={editStatus} class="sr-only" />
									<svg class="w-5 h-5 mb-1" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
										<path stroke-linecap="round" stroke-linejoin="round" d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z" />
									</svg>
									<span class="text-xs sm:text-sm font-medium">Can't make it</span>
								</label>
							</div>
						</fieldset>

						<!-- Dietary Notes -->
						<div>
							<label for="edit-dietary" class="block text-sm font-medium text-neutral-700 mb-1.5">
								Dietary Notes <span class="text-neutral-400 font-normal">(optional)</span>
							</label>
							<textarea
								id="edit-dietary"
								bind:value={editDietary}
								placeholder="Any allergies or dietary requirements?"
								rows="2"
								class="w-full rounded-md border border-neutral-300 px-4 py-2.5 text-neutral-900 placeholder:text-neutral-400 focus:outline-none focus:ring-2 focus:ring-primary/40 focus:border-primary transition-colors resize-none"
							></textarea>
						</div>

						<!-- Plus Ones -->
						<div>
							<label for="edit-plusones" class="block text-sm font-medium text-neutral-700 mb-1.5">
								Additional Guests
							</label>
							<div class="flex items-center gap-3">
								<input
									id="edit-plusones"
									type="number"
									min="0"
									max="10"
									bind:value={editPlusOnes}
									class="w-20 rounded-md border border-neutral-300 px-3 py-2.5 text-neutral-900 text-center focus:outline-none focus:ring-2 focus:ring-primary/40 focus:border-primary transition-colors"
								/>
								<span class="text-sm text-neutral-500">additional guest{editPlusOnes !== 1 ? 's' : ''}</span>
							</div>
						</div>

						<!-- Custom Questions -->
						{#if eventQuestions.length > 0}
							<QuestionRenderer questions={eventQuestions} bind:answers={editAnswers} />
						{/if}

						<!-- Error -->
						{#if saveError}
							<div class="rounded-md bg-error-light border border-error/20 px-4 py-3 text-sm text-error">
								{saveError}
							</div>
						{/if}

						<!-- Buttons -->
						<div class="flex gap-3">
							<button
								type="button"
								onclick={() => { editing = false; saveError = ''; }}
								class="flex-1 rounded-lg border border-neutral-300 px-4 py-2.5 text-sm font-medium text-neutral-700 hover:bg-neutral-50 transition-colors"
							>
								Cancel
							</button>
							<button
								type="submit"
								disabled={saving}
								class="flex-1 rounded-lg bg-primary px-4 py-2.5 text-sm font-semibold text-white hover:bg-primary-hover transition-colors disabled:opacity-50 disabled:cursor-not-allowed shadow-sm"
							>
								{#if saving}
									<span class="inline-flex items-center gap-2">
										<svg class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
											<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
											<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
										</svg>
										Saving...
									</span>
								{:else}
									Save Changes
								{/if}
							</button>
						</div>
					</form>
				{:else}
					<!-- Display mode -->
					<div class="space-y-4">
						<div class="flex items-center justify-between">
							<span class="text-sm text-neutral-500">Name</span>
							<span class="text-sm font-medium text-neutral-900">{attendee.name}</span>
						</div>
						{#if attendee.email}
							<div class="flex items-center justify-between">
								<span class="text-sm text-neutral-500">Email</span>
								<span class="text-sm font-medium text-neutral-900">{attendee.email}</span>
							</div>
						{/if}
						<div class="flex items-center justify-between">
							<span class="text-sm text-neutral-500">Status</span>
							<span class="inline-flex items-center rounded-full border px-3 py-0.5 text-xs font-semibold {statusBadgeClass(attendee.rsvpStatus)}">
								{statusLabel(attendee.rsvpStatus)}
							</span>
						</div>
						{#if attendee.dietaryNotes}
							<div class="flex items-center justify-between">
								<span class="text-sm text-neutral-500">Dietary Notes</span>
								<span class="text-sm font-medium text-neutral-900">{attendee.dietaryNotes}</span>
							</div>
						{/if}
						<div class="flex items-center justify-between">
							<span class="text-sm text-neutral-500">Additional Guests</span>
							<span class="text-sm font-medium text-neutral-900">{attendee.plusOnes}</span>
						</div>
					</div>
				{/if}
			</div>

			<!-- Attendance Display -->
			{#if attendance && (attendance.headcount > 0 || (attendance.names && attendance.names.length > 0)) && attendee.rsvpStatus !== 'declined'}
				<div class="bg-surface rounded-xl shadow-lg border border-neutral-200 p-6 sm:p-8 mb-6">
					<h2 class="font-display text-lg font-semibold text-neutral-900 mb-4">Who's Coming</h2>
					{#if attendance.headcount > 0}
						<div class="flex items-center gap-2 text-sm text-neutral-700 mb-3">
							<svg class="w-5 h-5 text-success flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
								<path stroke-linecap="round" stroke-linejoin="round" d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0z" />
							</svg>
							<span class="font-medium">{attendance.headcount} {attendance.headcount === 1 ? 'person' : 'people'} attending</span>
						</div>
					{/if}
					{#if attendance.names && attendance.names.length > 0}
						<div class="flex flex-wrap gap-2">
							{#each displayNames as guestName}
								<span class="inline-flex items-center rounded-full bg-primary-light px-3 py-1 text-xs font-medium text-primary border border-primary-light">
									{guestName}
								</span>
							{/each}
							{#if !showAllNames && attendance.names.length > 50}
								<button
									type="button"
									class="inline-flex items-center rounded-full bg-neutral-100 px-3 py-1 text-xs font-medium text-neutral-600 hover:bg-neutral-200 transition-colors"
									onclick={() => (showAllNames = true)}
								>
									+{attendance.names.length - 50} more
								</button>
							{/if}
							{#if showAllNames && attendance.names.length > 50}
								<button
									type="button"
									class="inline-flex items-center rounded-full bg-neutral-100 px-3 py-1 text-xs font-medium text-neutral-600 hover:bg-neutral-200 transition-colors"
									onclick={() => (showAllNames = false)}
								>
									Show less
								</button>
							{/if}
						</div>
					{/if}
				</div>
			{/if}

			<!-- Send Message to Organizer -->
			<div class="bg-surface rounded-xl shadow-lg border border-neutral-200 p-6 sm:p-8 mb-6">
				<div class="flex items-center justify-between mb-4">
					<h2 class="font-display text-lg font-semibold text-neutral-900">Message Organizer</h2>
					{#if !showMessageForm}
						<button
							onclick={() => { showMessageForm = true; }}
							class="inline-flex items-center gap-1.5 text-sm font-medium text-primary hover:text-primary-hover transition-colors"
						>
							<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
								<path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4" />
							</svg>
							New Message
						</button>
					{/if}
				</div>

				{#if messageSent}
					<div class="mb-4 rounded-md bg-success-light border border-success/20 px-4 py-3 text-sm text-success flex items-center gap-2">
						<svg class="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
							<path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
						</svg>
						Message sent to the organizer.
					</div>
				{/if}

				{#if showMessageForm}
					<form onsubmit={handleSendMessage} class="space-y-4">
						<div>
							<label for="msg-subject" class="block text-sm font-medium text-neutral-700 mb-1.5">
								Subject
							</label>
							<input
								id="msg-subject"
								type="text"
								required
								bind:value={msgSubject}
								placeholder="What is this about?"
								class="w-full rounded-md border border-neutral-300 px-4 py-2.5 text-neutral-900 placeholder:text-neutral-400 focus:outline-none focus:ring-2 focus:ring-primary/40 focus:border-primary transition-colors"
							/>
						</div>
						<div>
							<label for="msg-body" class="block text-sm font-medium text-neutral-700 mb-1.5">
								Message
							</label>
							<textarea
								id="msg-body"
								required
								bind:value={msgBody}
								placeholder="Write your message to the organizer..."
								rows="4"
								class="w-full rounded-md border border-neutral-300 px-4 py-2.5 text-neutral-900 placeholder:text-neutral-400 focus:outline-none focus:ring-2 focus:ring-primary/40 focus:border-primary transition-colors resize-none"
							></textarea>
						</div>

						{#if messageError}
							<div class="rounded-md bg-error-light border border-error/20 px-4 py-3 text-sm text-error">
								{messageError}
							</div>
						{/if}

						<div class="flex gap-3">
							<button
								type="button"
								onclick={() => { showMessageForm = false; messageError = ''; }}
								class="flex-1 rounded-lg border border-neutral-300 px-4 py-2.5 text-sm font-medium text-neutral-700 hover:bg-neutral-50 transition-colors"
							>
								Cancel
							</button>
							<button
								type="submit"
								disabled={sendingMessage}
								class="flex-1 rounded-lg bg-primary px-4 py-2.5 text-sm font-semibold text-white hover:bg-primary-hover transition-colors disabled:opacity-50 disabled:cursor-not-allowed shadow-sm"
							>
								{#if sendingMessage}
									<span class="inline-flex items-center gap-2">
										<svg class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
											<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
											<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
										</svg>
										Sending...
									</span>
								{:else}
									Send Message
								{/if}
							</button>
						</div>
					</form>
				{/if}

				<!-- Messages list -->
				{#if messages.length > 0}
					<div class="mt-6 pt-6 border-t border-neutral-100">
						<h3 class="text-sm font-medium text-neutral-500 mb-4">Conversation</h3>
						<div class="space-y-4">
							{#each messages as msg (msg.id)}
								<div class="rounded-md border p-4 {msg.senderType === 'attendee' ? 'border-primary-light bg-primary-lighter ml-4' : 'border-neutral-100 bg-neutral-50 mr-4'}">
									<div class="flex items-center justify-between mb-1">
										<span class="text-xs font-semibold {msg.senderType === 'attendee' ? 'text-primary' : 'text-neutral-600'}">
											{msg.senderType === 'attendee' ? 'You' : 'Organizer'}
										</span>
										<span class="text-xs text-neutral-400">{formatMessageDate(msg.createdAt)}</span>
									</div>
									{#if msg.subject}
										<p class="text-sm font-medium text-neutral-900 mb-0.5">{msg.subject}</p>
									{/if}
									<p class="text-sm text-neutral-700 whitespace-pre-line">{msg.body}</p>
								</div>
							{/each}
						</div>
					</div>
				{:else if !showMessageForm && !loadingMessages}
					<p class="text-sm text-neutral-400 text-center py-2">No messages yet.</p>
				{/if}
			</div>

			<!-- Powered by -->
			<div class="text-center mt-8">
				<a href="/" class="text-xs text-neutral-400 hover:text-neutral-500 transition-colors">
					Powered by OpenRSVP
				</a>
			</div>
		{/if}
	</div>
</div>

<style>
	.rsvp-option {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		padding: 0.75rem 0.5rem;
		border-radius: 10px;
		border: 2px solid #E7E5E4;
		cursor: pointer;
		transition: all 0.15s ease;
		color: #78716C;
		text-align: center;
	}
	.rsvp-option:hover {
		border-color: #D6D3D1;
		background-color: #FAFAF9;
	}
	.rsvp-option-selected {
		border-width: 2px;
	}
	.rsvp-option-attending {
		border-color: #22c55e;
		background-color: #f0fdf4;
		color: #16a34a;
	}
	.rsvp-option-maybe {
		border-color: #f59e0b;
		background-color: #fffbeb;
		color: #d97706;
	}
	.rsvp-option-declined {
		border-color: #ef4444;
		background-color: #fef2f2;
		color: #dc2626;
	}
	.rsvp-option-disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
	.rsvp-option-disabled:hover {
		border-color: #E7E5E4;
		background-color: transparent;
	}
</style>
