<script lang="ts">
	import { page } from '$app/stores';
	import { onMount } from 'svelte';
	import { api } from '$lib/api/client';
	import { smsEnabled, loadAppConfig } from '$lib/stores/config';
	import { formatDateTime } from '$lib/utils/dates';
	import type { PublicEvent, InviteCard, PublicAttendance, EventQuestion, ApiError } from '$lib/types';
	import InviteCardPreview from '$lib/components/invite/InviteCardPreview.svelte';
	import QuestionRenderer from '$lib/components/questions/QuestionRenderer.svelte';
	import AddToCalendar from '$lib/components/ui/AddToCalendar.svelte';

	interface PublicInviteData {
		event: PublicEvent;
		invite: InviteCard;
		attendance?: PublicAttendance;
		questions?: EventQuestion[];
	}

	let loading = $state(true);
	let error = $state('');
	let eventData = $state<PublicEvent | null>(null);
	let inviteData = $state<InviteCard | null>(null);
	let attendance = $state<PublicAttendance | null>(null);
	let eventQuestions = $state<EventQuestion[]>([]);
	let showAllNames = $state(false);
	const displayNames = $derived(
		attendance?.names
			? (showAllNames ? attendance.names : attendance.names.slice(0, 50))
			: []
	);

	// RSVP form state
	let name = $state('');
	let email = $state('');
	let phone = $state('');
	let rsvpStatus = $state<'attending' | 'maybe' | 'declined'>('attending');
	let dietaryNotes = $state('');
	let plusOnes = $state(0);
	let answers: Record<string, string> = $state({});
	let honeypot = $state('');
	let submitting = $state(false);
	let submitError = $state('');
	let submitted = $state(false);
	let rsvpToken = $state('');

	const token = $derived($page.params.token ?? '');

	onMount(async () => {
		await loadAppConfig();
		try {
			const result = await api.get<{ data: PublicInviteData }>(`/rsvp/public/${token}`);
			eventData = result.data.event;
			inviteData = result.data.invite;
			attendance = result.data.attendance ?? null;
			eventQuestions = result.data.questions ?? [];
		} catch (err) {
			const apiErr = err as ApiError;
			if (apiErr.status === 404) {
				error = 'This invitation could not be found. It may have expired or been removed.';
			} else {
				error = apiErr.message || 'Failed to load invitation. Please try again later.';
			}
		} finally {
			loading = false;
		}
	});

	const contactReq = $derived(eventData?.contactRequirement ?? 'email_or_phone');
	const emailRequired = $derived(
		!$smsEnabled || contactReq === 'email' || contactReq === 'email_and_phone' || contactReq === 'email_or_phone'
	);
	const phoneRequired = $derived(
		$smsEnabled && (contactReq === 'phone' || contactReq === 'email_and_phone')
	);

	// RSVP deadline display logic
	const deadlineText = $derived.by(() => {
		if (!eventData?.rsvpDeadline) return '';
		const deadline = new Date(eventData.rsvpDeadline);
		const now = new Date();
		const hoursLeft = Math.max(0, (deadline.getTime() - now.getTime()) / (1000 * 60 * 60));

		if (hoursLeft <= 0) return '';
		if (hoursLeft < 1) return 'Less than 1 hour left to RSVP';
		if (hoursLeft < 24) return `${Math.ceil(hoursLeft)} hours left to RSVP`;
		if (hoursLeft < 48) return 'About 1 day left to RSVP';
		return `RSVP by ${formatDateTime(eventData.rsvpDeadline)}`;
	});

	// Capacity display logic
	const capacityPercent = $derived(
		eventData?.maxCapacity && eventData?.spotsLeft !== undefined
			? Math.min(100, Math.round(((eventData.maxCapacity - eventData.spotsLeft) / eventData.maxCapacity) * 100))
			: 0
	);

	// When at capacity, default to 'maybe' instead of 'attending'
	const attendingDisabled = $derived(eventData?.atCapacity === true && !eventData?.waitlistEnabled);

	// Waitlist mode: at capacity but waitlist is enabled
	const showWaitlist = $derived(eventData?.atCapacity === true && eventData?.waitlistEnabled === true);

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();

		// Honeypot check
		if (honeypot) return;

		if (!name.trim()) {
			submitError = 'Please fill in your name.';
			return;
		}

		const hasEmail = !!email.trim();
		const hasPhone = !!phone.trim();

		// When SMS is disabled, email is always required.
		if (!$smsEnabled && !hasEmail) {
			submitError = 'Email is required.';
			return;
		}

		if (contactReq === 'email' && !hasEmail) {
			submitError = 'Email is required.';
			return;
		}
		if (contactReq === 'phone' && !hasPhone) {
			submitError = 'Phone number is required.';
			return;
		}
		if (contactReq === 'email_and_phone' && (!hasEmail || !hasPhone)) {
			submitError = 'Both email and phone are required.';
			return;
		}
		if (contactReq === 'email_or_phone' && !hasEmail && !hasPhone) {
			submitError = 'Please provide an email or phone number.';
			return;
		}

		submitting = true;
		submitError = '';

		try {
			const payload: Record<string, unknown> = {
				name: name.trim(),
				email: email.trim(),
				phone: phone.trim() || undefined,
				rsvpStatus,
				dietaryNotes: dietaryNotes.trim() || undefined,
				plusOnes
			};
			if (Object.keys(answers).length > 0) {
				payload.answers = answers;
			}
			const result = await api.post<{ data: { rsvpToken: string } }>(`/rsvp/public/${token}`, payload);
			rsvpToken = result.data.rsvpToken;
			submitted = true;

			// Re-fetch invite data to update attendance and capacity display.
			try {
				const refreshed = await api.get<{ data: PublicInviteData }>(`/rsvp/public/${token}`);
				eventData = refreshed.data.event;
				attendance = refreshed.data.attendance ?? null;
			} catch {
				// Non-critical; attendance display will use previous data.
			}
		} catch (err) {
			const apiErr = err as ApiError;
			submitError = apiErr.message || 'Failed to submit RSVP. Please try again.';
		} finally {
			submitting = false;
		}
	}

	// RSVP Lookup state
	let showLookup = $state(false);
	let lookupEmail = $state('');
	let lookupLoading = $state(false);
	let lookupError = $state('');
	let lookupSuccess = $state(false);

	async function handleLookup(e: SubmitEvent) {
		e.preventDefault();
		if (!lookupEmail.trim()) {
			lookupError = 'Please enter your email address.';
			return;
		}
		lookupLoading = true;
		lookupError = '';
		try {
			await api.post(`/rsvp/public/${token}/lookup`, {
				email: lookupEmail.trim()
			});
			lookupSuccess = true;
		} catch (err) {
			const apiErr = err as ApiError;
			lookupError = apiErr.message || 'Something went wrong. Please try again.';
		} finally {
			lookupLoading = false;
		}
	}

	const statusLabel = $derived.by(() => {
		switch (rsvpStatus) {
			case 'attending': return "I'll be there!";
			case 'maybe': return 'Maybe';
			case 'declined': return "Can't make it";
			default: return '';
		}
	});
</script>

<svelte:head>
	<title>{eventData ? `${eventData.title} — You're Invited` : "You're Invited"} — OpenRSVP</title>
</svelte:head>

<div class="invite-page min-h-screen flex flex-col items-center justify-start px-4 py-8 sm:py-12"
	style="background: linear-gradient(135deg, #f8fafc 0%, #eef2ff 50%, #fdf2f8 100%);"
>
	{#if loading}
		<div class="flex items-center justify-center min-h-[60vh]">
			<div class="flex flex-col items-center gap-4">
				<div class="animate-spin rounded-full h-10 w-10 border-b-2 border-indigo-500"></div>
				<p class="text-slate-500 text-sm">Loading your invitation...</p>
			</div>
		</div>
	{:else if error}
		<div class="flex items-center justify-center min-h-[60vh]">
			<div class="bg-white rounded-2xl shadow-lg border border-slate-200 p-8 max-w-md text-center">
				<div class="w-16 h-16 rounded-full bg-red-50 flex items-center justify-center mx-auto mb-4">
					<svg class="w-8 h-8 text-red-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
						<path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m9-.75a9 9 0 11-18 0 9 9 0 0118 0zm-9 3.75h.008v.008H12v-.008z" />
					</svg>
				</div>
				<h2 class="text-xl font-semibold text-slate-900 mb-2">Invitation Not Found</h2>
				<p class="text-slate-600">{error}</p>
			</div>
		</div>
	{:else if eventData && inviteData}
		<!-- Invite Card -->
		<div class="w-full max-w-lg mb-8 sm:mb-10">
			<InviteCardPreview
				templateId={inviteData.templateId}
				heading={inviteData.heading}
				body={inviteData.body}
				footer={inviteData.footer}
				primaryColor={inviteData.primaryColor}
				secondaryColor={inviteData.secondaryColor}
				font={inviteData.font}
				eventTitle={eventData.title}
				eventDate={eventData.eventDate}
				eventLocation={eventData.location}
				customData={typeof inviteData.customData === 'string' ? inviteData.customData : JSON.stringify(inviteData.customData || {})}
			/>
		</div>

		<!-- Capacity Display -->
		{#if showWaitlist}
			<div class="w-full max-w-lg mb-6">
				<div class="rounded-lg bg-blue-50 border border-blue-200 p-4 text-center">
					<p class="text-sm font-medium text-blue-800">This event is full. You can join the waitlist and we'll notify you if a spot opens up.</p>
				</div>
			</div>
		{:else if eventData.atCapacity}
			<div class="w-full max-w-lg mb-6">
				<div class="rounded-lg bg-red-50 border border-red-200 p-4 text-center">
					<p class="text-sm font-medium text-red-800">This event is at capacity</p>
					<p class="text-xs text-red-600 mt-1">
						You can still RSVP as "maybe" or "declined".
					</p>
				</div>
			</div>
		{:else if eventData.spotsLeft !== undefined && eventData.spotsLeft !== null && eventData.maxCapacity}
			<div class="w-full max-w-lg mb-6">
				<div class="bg-white/80 backdrop-blur-sm rounded-2xl shadow border border-slate-200/60 p-4">
					<div class="flex items-center justify-between text-xs text-slate-500 mb-1">
						<span>{eventData.spotsLeft} {eventData.spotsLeft === 1 ? 'spot' : 'spots'} remaining</span>
						<span>{eventData.maxCapacity - eventData.spotsLeft} / {eventData.maxCapacity}</span>
					</div>
					<div class="h-1.5 w-full rounded-full bg-slate-200 overflow-hidden">
						<div
							class="h-full rounded-full transition-all duration-300 {capacityPercent >= 90 ? 'bg-red-500' : capacityPercent >= 70 ? 'bg-amber-500' : 'bg-indigo-500'}"
							style="width: {capacityPercent}%"
							role="progressbar"
							aria-valuenow={capacityPercent}
							aria-valuemin={0}
							aria-valuemax={100}
							aria-label="Event capacity"
						></div>
					</div>
				</div>
			</div>
		{/if}

		<!-- RSVP Deadline Display -->
		{#if deadlineText && !eventData.rsvpsClosed}
			<div class="w-full max-w-lg mb-4 flex items-center justify-center gap-2">
				<svg class="h-3.5 w-3.5 text-amber-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path stroke-linecap="round" stroke-linejoin="round" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
				</svg>
				<p class="text-xs text-amber-600 font-medium">{deadlineText}</p>
			</div>
		{/if}

		<!-- Attendance Display -->
		{#if attendance && (attendance.headcount > 0 || (attendance.names && attendance.names.length > 0))}
			{#if !(submitted && rsvpStatus === 'declined')}
				<div class="w-full max-w-lg mb-8">
					<div class="bg-white/80 backdrop-blur-sm rounded-2xl shadow border border-slate-200/60 p-5">
						{#if attendance.headcount > 0}
							<div class="flex items-center gap-2 text-sm text-slate-700">
								<svg class="w-5 h-5 text-green-500 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
									<path stroke-linecap="round" stroke-linejoin="round" d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0z" />
								</svg>
								<span class="font-medium">{attendance.headcount} {attendance.headcount === 1 ? 'person' : 'people'} attending</span>
							</div>
						{/if}
						{#if attendance.names && attendance.names.length > 0}
							<div class="mt-3 flex flex-wrap gap-2">
								{#each displayNames as guestName}
									<span class="inline-flex items-center rounded-full bg-indigo-50 px-3 py-1 text-xs font-medium text-indigo-700 border border-indigo-100">
										{guestName}
									</span>
								{/each}
								{#if !showAllNames && attendance.names.length > 50}
									<button
										type="button"
										class="inline-flex items-center rounded-full bg-slate-100 px-3 py-1 text-xs font-medium text-slate-600 hover:bg-slate-200 transition-colors"
										onclick={() => (showAllNames = true)}
									>
										+{attendance.names.length - 50} more
									</button>
								{/if}
								{#if showAllNames && attendance.names.length > 50}
									<button
										type="button"
										class="inline-flex items-center rounded-full bg-slate-100 px-3 py-1 text-xs font-medium text-slate-600 hover:bg-slate-200 transition-colors"
										onclick={() => (showAllNames = false)}
									>
										Show less
									</button>
								{/if}
							</div>
						{/if}
					</div>
				</div>
			{/if}
		{:else if attendance && attendance.headcount === 0 && !(submitted && rsvpStatus === 'declined')}
			<div class="w-full max-w-lg mb-8">
				<div class="bg-white/80 backdrop-blur-sm rounded-2xl shadow border border-slate-200/60 p-5">
					<p class="text-sm text-slate-500 text-center">Be the first to RSVP!</p>
				</div>
			</div>
		{/if}

		<!-- RSVP Form or Success -->
		{#if eventData.rsvpsClosed}
			<div class="w-full max-w-lg">
				<div class="rounded-lg bg-amber-50 border border-amber-200 p-4 text-center">
					<p class="text-sm font-medium text-amber-800">RSVPs are closed</p>
					<p class="text-xs text-amber-600 mt-1">
						The RSVP deadline for this event has passed.
					</p>
				</div>
			</div>
		{:else if submitted}
			<div class="w-full max-w-lg">
				<div class="bg-white rounded-2xl shadow-lg border border-slate-200 p-8 text-center">
					<div class="w-16 h-16 rounded-full bg-green-50 flex items-center justify-center mx-auto mb-4">
						<svg class="w-8 h-8 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
							<path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
						</svg>
					</div>
					<h2 class="text-2xl font-bold text-slate-900 mb-2">RSVP Received!</h2>
					<p class="text-slate-600 mb-4">
						Thank you, <strong>{name}</strong>! Your response has been recorded.
					</p>
					<div class="inline-flex items-center gap-2 bg-slate-50 rounded-lg px-4 py-2 text-sm text-slate-600 mb-4">
						<span>Status:</span>
						<span class="font-semibold" class:text-green-600={rsvpStatus === 'attending'} class:text-amber-600={rsvpStatus === 'maybe'} class:text-red-600={rsvpStatus === 'declined'}>
							{statusLabel}
						</span>
					</div>
					{#if (rsvpStatus === 'attending' || rsvpStatus === 'maybe')}
						<div class="mt-4 flex justify-center">
							<AddToCalendar event={eventData} shareToken={token} />
						</div>
					{/if}
					{#if rsvpToken}
						<div class="mt-4 pt-4 border-t border-slate-100">
							<p class="text-sm text-slate-500 mb-3">
								Need to change your response? Use this link:
							</p>
							<a
								href="/r/{rsvpToken}"
								class="inline-flex items-center gap-2 text-sm font-medium text-indigo-600 hover:text-indigo-700 transition-colors"
							>
								<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
									<path stroke-linecap="round" stroke-linejoin="round" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
								</svg>
								Modify Your RSVP
							</a>
						</div>
					{/if}
				</div>
			</div>
		{:else}
			<div class="w-full max-w-lg">
				<div class="bg-white rounded-2xl shadow-lg border border-slate-200 p-6 sm:p-8">
					<h2 class="text-xl font-bold text-slate-900 mb-6 text-center">Your Response</h2>

					<form onsubmit={handleSubmit} class="space-y-5">
						<!-- Honeypot -->
						<input
							type="text"
							name="website"
							autocomplete="off"
							tabindex="-1"
							aria-hidden="true"
							class="absolute -left-[9999px] opacity-0 h-0 w-0 overflow-hidden"
							bind:value={honeypot}
						/>

						<!-- Name -->
						<div>
							<label for="rsvp-name" class="block text-sm font-medium text-slate-700 mb-1.5">
								Your Name <span class="text-red-500">*</span>
							</label>
							<input
								id="rsvp-name"
								type="text"
								required
								bind:value={name}
								placeholder="Enter your full name"
								class="w-full rounded-lg border border-slate-300 px-4 py-2.5 text-slate-900 placeholder:text-slate-400 focus:outline-none focus:ring-2 focus:ring-indigo-500/40 focus:border-indigo-500 transition-colors"
							/>
						</div>

						<!-- Email -->
						<div>
							<label for="rsvp-email" class="block text-sm font-medium text-slate-700 mb-1.5">
								Email Address
								{#if emailRequired}
									<span class="text-red-500">*</span>
								{:else}
									<span class="text-slate-400 font-normal">(optional)</span>
								{/if}
							</label>
							<input
								id="rsvp-email"
								type="email"
								required={emailRequired}
								bind:value={email}
								placeholder="you@example.com"
								class="w-full rounded-lg border border-slate-300 px-4 py-2.5 text-slate-900 placeholder:text-slate-400 focus:outline-none focus:ring-2 focus:ring-indigo-500/40 focus:border-indigo-500 transition-colors"
							/>
						</div>

						<!-- Phone -->
						<div>
							<label for="rsvp-phone" class="block text-sm font-medium text-slate-700 mb-1.5">
								Phone Number
								{#if phoneRequired}
									<span class="text-red-500">*</span>
								{:else}
									<span class="text-slate-400 font-normal">(optional)</span>
								{/if}
							</label>
							<input
								id="rsvp-phone"
								type="tel"
								required={phoneRequired}
								bind:value={phone}
								placeholder="+1 (555) 123-4567"
								class="w-full rounded-lg border border-slate-300 px-4 py-2.5 text-slate-900 placeholder:text-slate-400 focus:outline-none focus:ring-2 focus:ring-indigo-500/40 focus:border-indigo-500 transition-colors"
							/>
						</div>

						<!-- RSVP Status -->
						<fieldset>
							<legend class="block text-sm font-medium text-slate-700 mb-3">
								Will you attend?
							</legend>
							<div class="grid grid-cols-3 gap-3">
								<label
									class="rsvp-option {attendingDisabled ? 'rsvp-option-disabled' : ''}"
									class:rsvp-option-selected={rsvpStatus === 'attending'}
									class:rsvp-option-attending={rsvpStatus === 'attending'}
								>
									<input type="radio" name="rsvpStatus" value="attending" bind:group={rsvpStatus} class="sr-only" disabled={attendingDisabled} />
									<svg class="w-5 h-5 mb-1" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
										<path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
									</svg>
									<span class="text-xs sm:text-sm font-medium">I'll be there!</span>
									{#if attendingDisabled}
										<span class="text-[10px] text-red-500 mt-0.5">Full</span>
									{/if}
								</label>
								<label class="rsvp-option" class:rsvp-option-selected={rsvpStatus === 'maybe'} class:rsvp-option-maybe={rsvpStatus === 'maybe'}>
									<input type="radio" name="rsvpStatus" value="maybe" bind:group={rsvpStatus} class="sr-only" />
									<svg class="w-5 h-5 mb-1" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
										<path stroke-linecap="round" stroke-linejoin="round" d="M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
									</svg>
									<span class="text-xs sm:text-sm font-medium">Maybe</span>
								</label>
								<label class="rsvp-option" class:rsvp-option-selected={rsvpStatus === 'declined'} class:rsvp-option-declined={rsvpStatus === 'declined'}>
									<input type="radio" name="rsvpStatus" value="declined" bind:group={rsvpStatus} class="sr-only" />
									<svg class="w-5 h-5 mb-1" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
										<path stroke-linecap="round" stroke-linejoin="round" d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z" />
									</svg>
									<span class="text-xs sm:text-sm font-medium">Can't make it</span>
								</label>
							</div>
						</fieldset>

						{#if rsvpStatus !== 'declined'}
							<!-- Dietary Notes -->
							<div>
								<label for="rsvp-dietary" class="block text-sm font-medium text-slate-700 mb-1.5">
									Dietary Notes <span class="text-slate-400 font-normal">(optional)</span>
								</label>
								<textarea
									id="rsvp-dietary"
									bind:value={dietaryNotes}
									placeholder="Any allergies or dietary requirements?"
									rows="2"
									class="w-full rounded-lg border border-slate-300 px-4 py-2.5 text-slate-900 placeholder:text-slate-400 focus:outline-none focus:ring-2 focus:ring-indigo-500/40 focus:border-indigo-500 transition-colors resize-none"
								></textarea>
							</div>

							<!-- Plus Ones -->
							<div>
								<label for="rsvp-plusones" class="block text-sm font-medium text-slate-700 mb-1.5">
									Additional Guests
								</label>
								<div class="flex items-center gap-3">
									<input
										id="rsvp-plusones"
										type="number"
										min="0"
										max="10"
										bind:value={plusOnes}
										class="w-20 rounded-lg border border-slate-300 px-3 py-2.5 text-slate-900 text-center focus:outline-none focus:ring-2 focus:ring-indigo-500/40 focus:border-indigo-500 transition-colors"
									/>
									<span class="text-sm text-slate-500">additional guest{plusOnes !== 1 ? 's' : ''}</span>
								</div>
							</div>
						{/if}

						<!-- Custom Questions -->
						{#if eventQuestions.length > 0}
							<QuestionRenderer questions={eventQuestions} bind:answers />
						{/if}

						<!-- Error -->
						{#if submitError}
							<div class="rounded-lg bg-red-50 border border-red-200 px-4 py-3 text-sm text-red-700">
								{submitError}
							</div>
						{/if}

						<!-- Submit -->
						<button
							type="submit"
							disabled={submitting}
							class="w-full rounded-xl bg-indigo-600 px-6 py-3 text-base font-semibold text-white hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-indigo-500/40 focus:ring-offset-2 transition-all disabled:opacity-50 disabled:cursor-not-allowed shadow-lg shadow-indigo-600/25"
						>
							{#if submitting}
								<span class="inline-flex items-center gap-2">
									<svg class="animate-spin h-4 w-4 text-white" fill="none" viewBox="0 0 24 24">
										<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
										<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
									</svg>
									Sending...
								</span>
							{:else}
								{showWaitlist ? 'Join Waitlist' : 'Send RSVP'}
							{/if}
						</button>
					</form>
				</div>
			</div>
		{/if}

		<!-- Lookup existing RSVP -->
		{#if !submitted && !eventData.rsvpsClosed}
			<div class="w-full max-w-lg mt-6">
				{#if !showLookup}
					<p class="text-center">
						<button type="button" onclick={() => (showLookup = true)} class="text-sm text-indigo-600 hover:text-indigo-700 underline underline-offset-2 transition-colors">
							Already RSVP'd? Look up your response
						</button>
					</p>
				{:else if lookupSuccess}
					<div class="bg-white rounded-2xl shadow-lg border border-slate-200 p-6 text-center">
						<div class="w-12 h-12 rounded-full bg-green-50 flex items-center justify-center mx-auto mb-3">
							<svg class="w-6 h-6 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
								<path stroke-linecap="round" stroke-linejoin="round" d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
							</svg>
						</div>
						<h3 class="text-lg font-semibold text-slate-900 mb-2">Check Your Email</h3>
						<p class="text-sm text-slate-600">
							If you have an RSVP, you'll receive an email shortly with a link to manage it.
						</p>
					</div>
				{:else}
					<div class="bg-white rounded-2xl shadow-lg border border-slate-200 p-6">
						<h3 class="text-lg font-semibold text-slate-900 mb-4">Find Your RSVP</h3>
						<form onsubmit={handleLookup} class="space-y-4">
							<div>
								<label for="lookup-email" class="block text-sm font-medium text-slate-700 mb-1.5">
									Email Address
								</label>
								<input
									id="lookup-email"
									type="email"
									required
									bind:value={lookupEmail}
									placeholder="you@example.com"
									class="w-full rounded-lg border border-slate-300 px-4 py-2.5 text-slate-900 placeholder:text-slate-400 focus:outline-none focus:ring-2 focus:ring-indigo-500/40 focus:border-indigo-500 transition-colors"
								/>
							</div>
							{#if lookupError}
								<div class="rounded-lg bg-red-50 border border-red-200 px-4 py-3 text-sm text-red-700">
									{lookupError}
								</div>
							{/if}
							<div class="flex items-center justify-between">
								<button type="button" onclick={() => (showLookup = false)} class="text-sm text-slate-500 hover:text-slate-700 transition-colors">
									Cancel
								</button>
								<button
									type="submit"
									disabled={lookupLoading}
									class="rounded-lg bg-indigo-600 px-5 py-2.5 text-sm font-semibold text-white hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-indigo-500/40 focus:ring-offset-2 transition-all disabled:opacity-50 disabled:cursor-not-allowed"
								>
									{#if lookupLoading}
										Sending...
									{:else}
										Send me a link
									{/if}
								</button>
							</div>
						</form>
					</div>
				{/if}
			</div>
		{/if}

		<!-- Powered by -->
		<div class="mt-8 text-center">
			<a href="/" class="text-xs text-slate-400 hover:text-slate-500 transition-colors">
				Powered by OpenRSVP
			</a>
		</div>
	{/if}
</div>

<style>
	.rsvp-option {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		padding: 0.75rem 0.5rem;
		border-radius: 0.75rem;
		border: 2px solid #e2e8f0;
		cursor: pointer;
		transition: all 0.15s ease;
		color: #64748b;
		text-align: center;
	}
	.rsvp-option:hover {
		border-color: #cbd5e1;
		background-color: #f8fafc;
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
		border-color: #e2e8f0;
		background-color: transparent;
	}
</style>
