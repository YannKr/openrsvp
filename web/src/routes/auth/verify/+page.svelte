<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { api } from '$lib/api/client';
	import { currentUser } from '$lib/stores/auth';
	import { toast } from '$lib/stores/toast';
	import Spinner from '$lib/components/ui/Spinner.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import type { Organizer } from '$lib/types';
	import { onMount } from 'svelte';

	let verifying = $state(true);
	let error = $state('');

	onMount(async () => {
		const token = $page.url.searchParams.get('token');

		if (!token) {
			error = 'No verification token found. Please request a new magic link.';
			verifying = false;
			return;
		}

		try {
			const result = await api.post<{ token: string; organizer: Organizer }>('/auth/verify', { token });
			api.setToken(result.token);
			$currentUser = result.organizer;
			toast.success('Successfully signed in!');
			goto('/events');
		} catch (err: unknown) {
			const apiErr = err as { message?: string };
			error = apiErr.message || 'Verification failed. The link may have expired.';
			verifying = false;
		}
	});
</script>

<svelte:head>
	<title>Verify -- OpenRSVP</title>
</svelte:head>

<div class="min-h-screen flex items-center justify-center px-4">
	<div class="w-full max-w-md text-center">
		<a href="/" class="text-2xl font-bold text-primary">OpenRSVP</a>

		{#if verifying}
			<h1 class="font-display mt-4 text-2xl font-semibold text-neutral-900">Verifying your login</h1>
			<div class="mt-6 flex flex-col items-center">
				<Spinner size="md" class="text-primary" />
				<p class="mt-4 text-neutral-600">Please wait while we verify your magic link...</p>
			</div>
		{:else if error}
			<div class="mt-6">
				<div
					class="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-error-light mb-4"
				>
					<svg class="h-6 w-6 text-error" fill="none" viewBox="0 0 24 24" stroke="currentColor">
						<path
							stroke-linecap="round"
							stroke-linejoin="round"
							stroke-width="2"
							d="M6 18L18 6M6 6l12 12"
						/>
					</svg>
				</div>
				<h2 class="font-display text-lg font-semibold text-neutral-900 mb-2">Verification failed</h2>
				<p class="text-sm text-neutral-600 mb-6">{error}</p>
				<Button href="/auth/login">Try again</Button>
			</div>
		{/if}
	</div>
</div>
