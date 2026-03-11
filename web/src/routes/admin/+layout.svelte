<script lang="ts">
	import { goto } from '$app/navigation';
	import { currentUser, isLoading, isAdmin } from '$lib/stores/auth';

	$effect(() => {
		if (!$isLoading && (!$currentUser || !$isAdmin)) {
			goto('/events');
		}
	});

	let { children } = $props();
</script>

{#if $isLoading}
	<div class="flex items-center justify-center min-h-screen">
		<div class="loading-spinner"></div>
	</div>
{:else if $currentUser && $isAdmin}
	{@render children()}
{/if}
