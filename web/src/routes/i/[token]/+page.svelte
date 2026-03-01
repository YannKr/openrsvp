<script lang="ts">
	import { page } from '$app/stores';
	import { onMount } from 'svelte';
	import { api } from '$lib/api/client';
	import type { Event, InviteCard, ApiError } from '$lib/types';
	import InviteCardPreview from '$lib/components/invite/InviteCardPreview.svelte';

	interface PublicInviteData {
		event: Event;
		invite: InviteCard;
	}

	let loading = $state(true);
	let error = $state('');
	let eventData = $state<Event | null>(null);
	let inviteData = $state<InviteCard | null>(null);

	// RSVP form state
	let name = $state('');
	let email = $state('');
	let phone = $state('');
	let rsvpStatus = $state<'attending' | 'maybe' | 'declined'>('attending');
	let dietaryNotes = $state('');
	let plusOnes = $state(0);
	let honeypot = $state('');
	let submitting = $state(false);
	let submitError = $state('');
	let submitted = $state(false);
	let rsvpToken = $state('');

	const token = $derived($page.params.token);

	onMount(async () => {
		try {
			const result = await api.get<{ data: PublicInviteData }>(`/rsvp/public/${token}`);
			eventData = result.data.event;
			inviteData = result.data.invite;
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
	const emailRequired = $derived(contactReq === 'email' || contactReq === 'email_and_phone');
	const phoneRequired = $derived(contactReq === 'phone' || contactReq === 'email_and_phone');

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
			const result = await api.post<{ data: { rsvpToken: string } }>(`/rsvp/public/${token}`, {
				name: name.trim(),
				email: email.trim(),
				phone: phone.trim() || undefined,
				rsvpStatus,
				dietaryNotes: dietaryNotes.trim() || undefined,
				plusOnes
			});
			rsvpToken = result.data.rsvpToken;
			submitted = true;
		} catch (err) {
			const apiErr = err as ApiError;
			submitError = apiErr.message || 'Failed to submit RSVP. Please try again.';
		} finally {
			submitting = false;
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

		<!-- RSVP Form or Success -->
		{#if submitted}
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
								{:else if contactReq === 'email_or_phone'}
									<span class="text-slate-400 font-normal">(email or phone required)</span>
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
								{:else if contactReq === 'email_or_phone'}
									<span class="text-slate-400 font-normal">(email or phone required)</span>
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
								<label class="rsvp-option" class:rsvp-option-selected={rsvpStatus === 'attending'} class:rsvp-option-attending={rsvpStatus === 'attending'}>
									<input type="radio" name="rsvpStatus" value="attending" bind:group={rsvpStatus} class="sr-only" />
									<svg class="w-5 h-5 mb-1" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
										<path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
									</svg>
									<span class="text-xs sm:text-sm font-medium">I'll be there!</span>
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
								Send RSVP
							{/if}
						</button>
					</form>
				</div>
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
</style>
