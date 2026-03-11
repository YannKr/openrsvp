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

{#if $isLoading}
	<div class="flex items-center justify-center min-h-screen">
		<div class="loading-spinner"></div>
	</div>
{:else if $currentUser}
	{@render children()}
{/if}
