<script lang="ts">
	import '../app.css';
	import { currentUser, isLoading } from '$lib/stores/auth';
	import { api } from '$lib/api/client';
	import { onMount } from 'svelte';
	import Toast from '$lib/components/ui/Toast.svelte';

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

<div class="min-h-screen bg-neutral-50">
	{@render children()}
	<Toast />
</div>
