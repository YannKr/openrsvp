<script lang="ts">
	import '../app.css';
	import { currentUser, isLoading } from '$lib/stores/auth';
	import { toast } from '$lib/stores/toast';
	import { api } from '$lib/api/client';
	import { onMount } from 'svelte';

	onMount(async () => {
		if (!api.getToken()) {
			$currentUser = null;
			$isLoading = false;
			return;
		}
		try {
			const user = await api.get<import('$lib/types').Organizer>('/auth/me');
			$currentUser = user;

			// Auto-save browser timezone to profile if not set yet.
			if (!user.timezone) {
				const tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
				if (tz) {
					api.patch<import('$lib/types').Organizer>('/auth/me', { timezone: tz })
						.then((updated) => { $currentUser = updated; })
						.catch(() => {});
				}
			}
		} catch {
			api.setToken('');
			$currentUser = null;
		} finally {
			$isLoading = false;
		}
	});

	let { children } = $props();
</script>

<div class="min-h-screen bg-slate-50">
	{#if $isLoading}
		<div class="flex items-center justify-center min-h-screen">
			<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-500"></div>
		</div>
	{:else}
		{@render children()}
	{/if}

	<!-- Toast container -->
	{#if $toast.length > 0}
		<div class="fixed bottom-4 right-4 z-50 flex flex-col gap-2">
			{#each $toast as t (t.id)}
				<div
					class="px-4 py-3 rounded-lg shadow-lg text-white text-sm max-w-sm"
					class:bg-green-600={t.type === 'success'}
					class:bg-red-600={t.type === 'error'}
					class:bg-blue-600={t.type === 'info'}
					class:bg-yellow-600={t.type === 'warning'}
				>
					<div class="flex items-center justify-between gap-2">
						<span>{t.message}</span>
						<button onclick={() => toast.remove(t.id)} class="text-white/80 hover:text-white">
							x
						</button>
					</div>
				</div>
			{/each}
		</div>
	{/if}
</div>
