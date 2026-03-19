<script lang="ts">
	import { goto } from '$app/navigation';
	import { api } from '$lib/api/client';
	import { currentUser } from '$lib/stores/auth';
	import Spinner from '$lib/components/ui/Spinner.svelte';
	import { onMount } from 'svelte';

	onMount(async () => {
		try {
			await api.post('/auth/logout');
		} catch {
			// Ignore errors on logout
		} finally {
			api.setToken('');
			$currentUser = null;
			goto('/');
		}
	});
</script>

<svelte:head>
	<title>Signing out -- OpenRSVP</title>
</svelte:head>

<div class="min-h-screen flex items-center justify-center px-4">
	<div class="text-center">
		<Spinner size="md" class="text-primary mx-auto" />
		<p class="mt-4 text-neutral-600">Signing you out...</p>
	</div>
</div>
