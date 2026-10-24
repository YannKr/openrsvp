<script lang="ts">
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import { api } from '#lib/api/client.js';
	import type { ApiError } from '#lib/types/index.js';

	type View = 'loading' | 'invalid' | 'confirm' | 'success' | 'error';

	let view = $state<View>('loading');
	let errorMessage = $state('');
	let email = $state('');
	let eventId = $state('');
	let submitting = $state(false);

	let token = '';

	// Wording: GET only resolves an eventId (not a name), so scope is phrased
	// generically — event-scoped vs. all OpenRSVP emails.
	const scopeLabel = $derived(eventId ? "this event's emails" : 'all OpenRSVP emails');

	onMount(async () => {
		const t = page.url.searchParams.get('token');

		if (!t) {
			view = 'invalid';
			errorMessage = 'This unsubscribe link is invalid or incomplete.';
			return;
		}
		token = t;

		try {
			const result = await api.get<{ data: { email: string; eventId?: string } }>(
				`/unsubscribe?token=${encodeURIComponent(token)}`
			);
			email = result.data.email;
			eventId = result.data.eventId ?? '';
			view = 'confirm';
		} catch (err) {
			const apiErr = err as ApiError;
			view = 'invalid';
			if (apiErr.status === 404 || apiErr.status === 400) {
				errorMessage = 'This unsubscribe link is invalid or has expired.';
			} else {
				errorMessage = apiErr.message || 'We could not load this unsubscribe link. Please try again later.';
			}
		}
	});

	async function handleConfirm() {
		submitting = true;
		errorMessage = '';
		try {
			await api.post<{ data: { status: string } }>('/unsubscribe', { token });
			view = 'success';
		} catch (err) {
			const apiErr = err as ApiError;
			view = 'error';
			errorMessage = apiErr.message || 'We could not complete your request. Please try again.';
		} finally {
			submitting = false;
		}
	}
</script>

<svelte:head>
	<title>Unsubscribe — OpenRSVP</title>
</svelte:head>

<div
	class="min-h-screen flex items-center justify-center px-4 py-12"
	style="background: linear-gradient(135deg, #FAFAF9 0%, #FFF1F3 50%, #FDE8EC 100%);"
>
	<div class="w-full max-w-md">
		<div class="text-center mb-6">
			<a href="/" class="text-2xl font-bold text-primary">OpenRSVP</a>
		</div>

		<div class="bg-surface rounded-xl shadow-lg border border-neutral-200 p-8 text-center">
			{#if view === 'loading'}
				<div class="flex flex-col items-center gap-4 py-4">
					<div class="animate-spin rounded-full h-10 w-10 border-b-2 border-primary"></div>
					<p class="text-neutral-500 text-sm">Checking your unsubscribe link…</p>
				</div>
			{:else if view === 'invalid'}
				<div class="w-16 h-16 rounded-full bg-error-light flex items-center justify-center mx-auto mb-4">
					<svg class="w-8 h-8 text-error" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
						<path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m9-.75a9 9 0 11-18 0 9 9 0 0118 0zm-9 3.75h.008v.008H12v-.008z" />
					</svg>
				</div>
				<h1 class="font-display text-xl font-semibold text-neutral-900 mb-2">Invalid unsubscribe link</h1>
				<p class="text-neutral-600 text-sm">{errorMessage}</p>
			{:else if view === 'confirm'}
				<div class="w-16 h-16 rounded-full bg-primary-light flex items-center justify-center mx-auto mb-4">
					<svg class="w-8 h-8 text-primary" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
						<path stroke-linecap="round" stroke-linejoin="round" d="M21.75 6.75v10.5a2.25 2.25 0 01-2.25 2.25h-15a2.25 2.25 0 01-2.25-2.25V6.75m19.5 0A2.25 2.25 0 0019.5 4.5h-15a2.25 2.25 0 00-2.25 2.25m19.5 0v.243a2.25 2.25 0 01-1.07 1.916l-7.5 4.615a2.25 2.25 0 01-2.36 0L3.32 8.91a2.25 2.25 0 01-1.07-1.916V6.75" />
					</svg>
				</div>
				<h1 class="font-display text-xl font-semibold text-neutral-900 mb-2">Unsubscribe from emails?</h1>
				<p class="text-neutral-600 text-sm mb-1">
					Unsubscribe <span class="font-medium text-neutral-900">{email}</span> from {scopeLabel}?
				</p>
				<p class="text-neutral-400 text-xs mb-6">You won't receive any further emails to this address.</p>

				{#if errorMessage}
					<div class="rounded-md bg-error-light border border-error/20 px-4 py-3 text-sm text-error mb-4 text-left">
						{errorMessage}
					</div>
				{/if}

				<button
					type="button"
					onclick={handleConfirm}
					disabled={submitting}
					class="w-full rounded-lg bg-primary px-4 py-2.5 text-sm font-semibold text-white hover:bg-primary-hover transition-colors disabled:opacity-50 disabled:cursor-not-allowed shadow-sm"
				>
					{#if submitting}
						<span class="inline-flex items-center gap-2">
							<svg class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
								<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
								<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
							</svg>
							Unsubscribing…
						</span>
					{:else}
						Unsubscribe
					{/if}
				</button>
				<p class="text-neutral-400 text-xs mt-4">
					Changed your mind later? You can re-subscribe by RSVPing again or contacting the event organizer.
				</p>
			{:else if view === 'success'}
				<div class="w-16 h-16 rounded-full bg-success-light flex items-center justify-center mx-auto mb-4">
					<svg class="w-8 h-8 text-success" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
						<path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75L11.25 15 15 9.75M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
					</svg>
				</div>
				<h1 class="font-display text-xl font-semibold text-neutral-900 mb-2">You've been unsubscribed</h1>
				<p class="text-neutral-600 text-sm">
					{#if email}
						<span class="font-medium text-neutral-900">{email}</span> will no longer receive {scopeLabel}.
					{:else}
						You will no longer receive these emails.
					{/if}
				</p>
				<p class="text-neutral-400 text-xs mt-4">
					You can re-subscribe at any time by RSVPing again or contacting the event organizer.
				</p>
			{:else if view === 'error'}
				<div class="w-16 h-16 rounded-full bg-error-light flex items-center justify-center mx-auto mb-4">
					<svg class="w-8 h-8 text-error" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
						<path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m9-.75a9 9 0 11-18 0 9 9 0 0118 0zm-9 3.75h.008v.008H12v-.008z" />
					</svg>
				</div>
				<h1 class="font-display text-xl font-semibold text-neutral-900 mb-2">Something went wrong</h1>
				<p class="text-neutral-600 text-sm mb-6">{errorMessage}</p>
				<button
					type="button"
					onclick={handleConfirm}
					disabled={submitting}
					class="w-full rounded-lg bg-primary px-4 py-2.5 text-sm font-semibold text-white hover:bg-primary-hover transition-colors disabled:opacity-50 disabled:cursor-not-allowed shadow-sm"
				>
					{submitting ? 'Trying again…' : 'Try again'}
				</button>
			{/if}
		</div>

		<div class="text-center mt-8">
			<a href="/" class="text-xs text-neutral-400 hover:text-neutral-500 transition-colors">
				Powered by OpenRSVP
			</a>
		</div>
	</div>
</div>
