<script lang="ts">
	import '../app.css';
	import { currentUser, isLoading } from '#lib/stores/auth.js';
	import { api } from '#lib/api/client.js';
	import { onMount } from 'svelte';
	import Toast from '#lib/components/ui/Toast.svelte';

	onMount(async () => {
		try {
			const user = await api.get<import('#lib/types/index.js').Organizer>('/auth/me');
			$currentUser = user;

			// Auto-save browser timezone to profile if not set yet.
			if (!user.timezone) {
				const tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
				if (tz) {
					api.patch<import('#lib/types/index.js').Organizer>('/auth/me', { timezone: tz })
						.then((updated) => { $currentUser = updated; })
						.catch(() => {});
				}
			}
		} catch {
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
