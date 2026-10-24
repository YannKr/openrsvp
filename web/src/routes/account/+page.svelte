<script lang="ts">
	import { goto } from '$app/navigation';
	import { api } from '#lib/api/client.js';
	import { toast } from '#lib/stores/toast.js';
	import { currentUser, isLoading } from '#lib/stores/auth.js';
	import AppShell from '#lib/components/layout/AppShell.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import Card from '#lib/components/ui/Card.svelte';
	import Input from '#lib/components/ui/Input.svelte';
	import Modal from '#lib/components/ui/Modal.svelte';
	import Spinner from '#lib/components/ui/Spinner.svelte';

	// Mirror the events layout guard: redirect to login once auth has loaded
	// and there's no current user.
	$effect(() => {
		if (!$isLoading && !$currentUser) {
			goto('/auth/login');
		}
	});

	let exporting = $state(false);

	let deleteModalOpen = $state(false);
	let deleteConfirmText = $state('');
	let deleting = $state(false);

	const canDelete = $derived(deleteConfirmText.trim() === 'DELETE');

	async function handleExport() {
		exporting = true;
		try {
			await api.download('/auth/me/export', 'openrsvp-export.json');
		} catch (err: unknown) {
			const apiErr = err as { message?: string };
			toast.error(apiErr.message || 'Failed to export your data');
		} finally {
			exporting = false;
		}
	}

	function openDeleteModal() {
		deleteConfirmText = '';
		deleteModalOpen = true;
	}

	async function handleDelete() {
		if (!canDelete) return;

		deleting = true;
		try {
			await api.delete('/auth/me');
			deleteModalOpen = false;
			$currentUser = null;
			toast.success('Your account and all associated data have been deleted.');
			goto('/');
		} catch (err: unknown) {
			const apiErr = err as { message?: string };
			toast.error(apiErr.message || 'Failed to delete your account');
		} finally {
			deleting = false;
		}
	}
</script>

<svelte:head>
	<title>Account Settings -- OpenRSVP</title>
</svelte:head>

{#if $isLoading}
	<div class="flex items-center justify-center min-h-screen">
		<Spinner size="lg" class="text-primary" />
	</div>
{:else if $currentUser}
	<AppShell>
		<div class="max-w-3xl mx-auto">
			<div class="mb-8">
				<h1 class="text-2xl font-bold font-display text-neutral-900">Account Settings</h1>
				<p class="mt-1 text-sm text-neutral-500">{$currentUser.email}</p>
			</div>

			<!-- Section 1: Your data -->
			<Card>
				<div class="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-4">
					<div class="sm:pr-8">
						<h2 class="text-lg font-display font-semibold text-neutral-900">Your data</h2>
						<p class="mt-1 text-sm text-neutral-600">
							Download a complete copy of your data as a JSON file. This includes all of your
							events, guests, RSVPs, and messages.
						</p>
					</div>
					<div class="flex-shrink-0">
						<Button variant="outline" loading={exporting} onclick={handleExport}>
							Export my data
						</Button>
					</div>
				</div>
			</Card>

			<!-- Section 2: Danger zone -->
			<Card class="mt-6 border-error-light">
				<div class="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-4">
					<div class="sm:pr-8">
						<h2 class="text-lg font-display font-semibold text-error">Danger zone</h2>
						<p class="mt-1 text-sm text-neutral-600">
							Permanently delete your account along with all of your events, guests, RSVPs, and
							messages. This action cannot be undone.
						</p>
					</div>
					<div class="flex-shrink-0">
						<Button variant="danger" onclick={openDeleteModal}>Delete account</Button>
					</div>
				</div>
			</Card>
		</div>
	</AppShell>

	<Modal bind:open={deleteModalOpen} title="Delete account">
		<div class="space-y-4">
			<div class="rounded-lg bg-error-light border border-error px-4 py-3 text-sm text-error flex items-start gap-2">
				<svg class="h-4 w-4 text-error mt-0.5 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path stroke-linecap="round" stroke-linejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
				</svg>
				<span>
					This permanently deletes your account and <strong>all</strong> of your events, guests,
					RSVPs, and messages. This cannot be undone.
				</span>
			</div>
			<p class="text-sm text-neutral-600">
				To confirm, type <span class="font-mono font-semibold text-neutral-900">DELETE</span> in the box below.
			</p>
			<Input
				name="deleteConfirm"
				bind:value={deleteConfirmText}
				placeholder="DELETE"
			/>
		</div>

		{#snippet actions()}
			<Button variant="outline" onclick={() => (deleteModalOpen = false)} disabled={deleting}>
				Cancel
			</Button>
			<Button variant="danger" loading={deleting} disabled={!canDelete} onclick={handleDelete}>
				Delete my account
			</Button>
		{/snippet}
	</Modal>
{/if}
