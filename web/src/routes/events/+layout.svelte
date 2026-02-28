<script lang="ts">
	import { goto } from '$app/navigation';
	import { currentUser, isLoading } from '$lib/stores/auth';

	$effect(() => {
		if (!$isLoading && !$currentUser) {
			goto('/auth/login');
		}
	});

	let { children } = $props();
</script>

{#if $currentUser}
	{@render children()}
{/if}
