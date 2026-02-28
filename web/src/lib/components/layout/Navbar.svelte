<script lang="ts">
	import { currentUser } from '$lib/stores/auth';
	import Button from '$lib/components/ui/Button.svelte';

	let mobileMenuOpen = $state(false);
</script>

<nav class="bg-white border-b border-slate-200">
	<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
		<div class="flex items-center justify-between h-16">
			<!-- Logo + Nav Links -->
			<div class="flex items-center gap-8">
				<a href="/events" class="text-xl font-bold text-indigo-600">OpenRSVP</a>
				<div class="hidden md:flex items-center gap-1">
					<a
						href="/events"
						class="px-3 py-2 rounded-lg text-sm font-medium text-slate-600 hover:text-slate-900 hover:bg-slate-50 transition-colors"
					>
						Dashboard
					</a>
					<a
						href="/events/new"
						class="px-3 py-2 rounded-lg text-sm font-medium text-slate-600 hover:text-slate-900 hover:bg-slate-50 transition-colors"
					>
						Create Event
					</a>
				</div>
			</div>

			<!-- User menu (desktop) -->
			<div class="hidden md:flex items-center gap-4">
				{#if $currentUser}
					<span class="text-sm text-slate-600">{$currentUser.email}</span>
				{/if}
				<Button variant="ghost" size="sm" href="/auth/logout">Logout</Button>
			</div>

			<!-- Mobile hamburger -->
			<button
				type="button"
				class="md:hidden p-2 rounded-lg text-slate-600 hover:bg-slate-100 transition-colors"
				onclick={() => (mobileMenuOpen = !mobileMenuOpen)}
				aria-label="Toggle menu"
			>
				{#if mobileMenuOpen}
					<svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
						<path
							stroke-linecap="round"
							stroke-linejoin="round"
							stroke-width="2"
							d="M6 18L18 6M6 6l12 12"
						/>
					</svg>
				{:else}
					<svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
						<path
							stroke-linecap="round"
							stroke-linejoin="round"
							stroke-width="2"
							d="M4 6h16M4 12h16M4 18h16"
						/>
					</svg>
				{/if}
			</button>
		</div>
	</div>

	<!-- Mobile menu -->
	{#if mobileMenuOpen}
		<div class="md:hidden border-t border-slate-200">
			<div class="px-4 py-3 space-y-1">
				<a
					href="/events"
					class="block px-3 py-2 rounded-lg text-sm font-medium text-slate-600 hover:text-slate-900 hover:bg-slate-50"
				>
					Dashboard
				</a>
				<a
					href="/events/new"
					class="block px-3 py-2 rounded-lg text-sm font-medium text-slate-600 hover:text-slate-900 hover:bg-slate-50"
				>
					Create Event
				</a>
			</div>
			<div class="border-t border-slate-200 px-4 py-3">
				{#if $currentUser}
					<p class="text-sm text-slate-600 mb-2">{$currentUser.email}</p>
				{/if}
				<a
					href="/auth/logout"
					class="block px-3 py-2 rounded-lg text-sm font-medium text-slate-600 hover:text-slate-900 hover:bg-slate-50"
				>
					Logout
				</a>
			</div>
		</div>
	{/if}
</nav>
