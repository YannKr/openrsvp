<script lang="ts">
	import { api } from '$lib/api/client';
	import { toast } from '$lib/stores/toast';
	import { isValidEmail } from '$lib/utils/validation';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';

	let email = $state('');
	let loading = $state(false);
	let sent = $state(false);
	let emailError = $state('');

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		emailError = '';

		if (!email.trim()) {
			emailError = 'Email is required';
			return;
		}

		if (!isValidEmail(email)) {
			emailError = 'Please enter a valid email address';
			return;
		}

		loading = true;
		try {
			await api.post('/auth/magic-link', { email });
			sent = true;
		} catch (err: unknown) {
			const apiErr = err as { message?: string };
			toast.error(apiErr.message || 'Failed to send magic link. Please try again.');
		} finally {
			loading = false;
		}
	}
</script>

<svelte:head>
	<title>Sign In -- OpenRSVP</title>
</svelte:head>

<div class="min-h-screen flex items-center justify-center px-4">
	<div class="w-full max-w-md">
		<div class="text-center mb-8">
			<a href="/" class="text-2xl font-bold text-indigo-600">OpenRSVP</a>
			<h1 class="mt-4 text-2xl font-semibold text-slate-900">Sign in to your account</h1>
			<p class="mt-2 text-slate-600">Enter your email to receive a magic link</p>
		</div>

		<div class="bg-white rounded-xl shadow-sm border border-slate-200 p-8">
			{#if sent}
				<!-- Success state -->
				<div class="text-center">
					<div
						class="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-green-100 mb-4"
					>
						<svg
							class="h-6 w-6 text-green-600"
							fill="none"
							viewBox="0 0 24 24"
							stroke="currentColor"
						>
							<path
								stroke-linecap="round"
								stroke-linejoin="round"
								stroke-width="2"
								d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
							/>
						</svg>
					</div>
					<h2 class="text-lg font-semibold text-slate-900 mb-2">Check your email</h2>
					<p class="text-sm text-slate-600 mb-4">
						We sent a magic link to <strong>{email}</strong>. Click the link in the email to sign
						in.
					</p>
					<p class="text-xs text-slate-500 mb-6">
						Did not receive the email? Check your spam folder or try again.
					</p>
					<Button
						variant="outline"
						onclick={() => {
							sent = false;
							email = '';
						}}
					>
						Try a different email
					</Button>
				</div>
			{:else}
				<!-- Login form -->
				<form onsubmit={handleSubmit} class="space-y-6">
					<Input
						label="Email address"
						name="email"
						type="email"
						bind:value={email}
						placeholder="you@example.com"
						error={emailError}
						required
					/>

					<Button type="submit" {loading} class="w-full">
						{loading ? 'Sending...' : 'Send Magic Link'}
					</Button>
				</form>

				<div class="mt-6 text-center">
					<a href="/" class="text-sm text-indigo-600 hover:text-indigo-500">
						Back to home
					</a>
				</div>
			{/if}
		</div>
	</div>
</div>
