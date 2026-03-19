<script lang="ts">
	import { onMount } from 'svelte';
	import { currentUser, isAdmin } from '$lib/stores/auth';
	import Button from '$lib/components/ui/Button.svelte';

	let mobileMenuOpen = $state(false);
	let isDark = $state(false);

	onMount(() => {
		isDark = document.documentElement.getAttribute('data-theme') === 'dark';
	});

	function toggleTheme() {
		isDark = !isDark;
		if (isDark) {
			document.documentElement.setAttribute('data-theme', 'dark');
			localStorage.setItem('theme', 'dark');
		} else {
			document.documentElement.removeAttribute('data-theme');
			localStorage.setItem('theme', 'light');
		}
	}
</script>

<nav class="bg-neutral-50 border-b border-neutral-200">
	<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
		<div class="flex items-center justify-between h-16">
			<!-- Logo + Nav Links -->
			<div class="flex items-center gap-8">
				<a href="/" class="text-xl font-display font-bold text-primary">OpenRSVP</a>
				<div class="hidden md:flex items-center gap-1">
					<a
						href="/events"
						class="px-3 py-2 rounded-md text-sm font-medium text-neutral-600 hover:text-neutral-900 hover:bg-neutral-100 transition-colors duration-short ease-out"
					>
						Dashboard
					</a>
					<a
						href="/events/new"
						class="px-3 py-2 rounded-md text-sm font-medium text-neutral-600 hover:text-neutral-900 hover:bg-neutral-100 transition-colors duration-short ease-out"
					>
						Create Event
					</a>
					{#if $isAdmin}
						<a
							href="/admin"
							class="px-3 py-2 rounded-md text-sm font-medium text-primary hover:text-primary-hover hover:bg-primary-light transition-colors duration-short ease-out"
						>
							Admin
						</a>
					{/if}
				</div>
			</div>

			<!-- Theme toggle + User menu (desktop) -->
			<div class="hidden md:flex items-center gap-4">
				<button
					type="button"
					class="rounded-md p-2 text-neutral-400 hover:text-neutral-700 transition-colors duration-short ease-out"
					onclick={toggleTheme}
					aria-label={isDark ? 'Switch to light mode' : 'Switch to dark mode'}
				>
					{#if isDark}
						<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="w-5 h-5"><path d="M10 2a.75.75 0 01.75.75v1.5a.75.75 0 01-1.5 0v-1.5A.75.75 0 0110 2zM10 15a.75.75 0 01.75.75v1.5a.75.75 0 01-1.5 0v-1.5A.75.75 0 0110 15zM10 7a3 3 0 100 6 3 3 0 000-6zM15.657 5.404a.75.75 0 10-1.06-1.06l-1.061 1.06a.75.75 0 001.06 1.06l1.06-1.06zM6.464 14.596a.75.75 0 10-1.06-1.06l-1.06 1.06a.75.75 0 001.06 1.06l1.06-1.06zM18 10a.75.75 0 01-.75.75h-1.5a.75.75 0 010-1.5h1.5A.75.75 0 0118 10zM5 10a.75.75 0 01-.75.75h-1.5a.75.75 0 010-1.5h1.5A.75.75 0 015 10zM14.596 15.657a.75.75 0 001.06-1.06l-1.06-1.061a.75.75 0 10-1.06 1.06l1.06 1.06zM5.404 6.464a.75.75 0 001.06-1.06l-1.06-1.06a.75.75 0 10-1.06 1.06l1.06 1.06z" /></svg>
					{:else}
						<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="w-5 h-5"><path fill-rule="evenodd" d="M7.455 2.004a.75.75 0 01.26.77 7 7 0 009.958 7.967.75.75 0 011.067.853A8.5 8.5 0 116.647 1.921a.75.75 0 01.808.083z" clip-rule="evenodd" /></svg>
					{/if}
				</button>
				{#if $currentUser}
					<span class="text-sm text-neutral-600">{$currentUser.email}</span>
				{/if}
				<Button variant="ghost" size="sm" href="/auth/logout">Logout</Button>
			</div>

			<!-- Mobile hamburger -->
			<button
				type="button"
				class="md:hidden p-2 rounded-md text-neutral-600 hover:bg-neutral-100 transition-colors duration-short ease-out"
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
		<div class="md:hidden border-t border-neutral-200">
			<div class="px-4 py-3 space-y-1">
				<a
					href="/events"
					class="block px-3 py-2 rounded-md text-sm font-medium text-neutral-600 hover:text-neutral-900 hover:bg-neutral-100"
				>
					Dashboard
				</a>
				<a
					href="/events/new"
					class="block px-3 py-2 rounded-md text-sm font-medium text-neutral-600 hover:text-neutral-900 hover:bg-neutral-100"
				>
					Create Event
				</a>
				{#if $isAdmin}
					<a
						href="/admin"
						class="block px-3 py-2 rounded-md text-sm font-medium text-primary hover:text-primary-hover hover:bg-primary-light"
					>
						Admin
					</a>
				{/if}
			</div>
			<div class="border-t border-neutral-200 px-4 py-3 flex items-center justify-between">
				<div>
					{#if $currentUser}
						<p class="text-sm text-neutral-600 mb-2">{$currentUser.email}</p>
					{/if}
					<a
						href="/auth/logout"
						class="block px-3 py-2 rounded-md text-sm font-medium text-neutral-600 hover:text-neutral-900 hover:bg-neutral-100"
					>
						Logout
					</a>
				</div>
				<button
					type="button"
					class="rounded-md p-2 text-neutral-400 hover:text-neutral-700 transition-colors duration-short ease-out"
					onclick={toggleTheme}
					aria-label={isDark ? 'Switch to light mode' : 'Switch to dark mode'}
				>
					{#if isDark}
						<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="w-5 h-5"><path d="M10 2a.75.75 0 01.75.75v1.5a.75.75 0 01-1.5 0v-1.5A.75.75 0 0110 2zM10 15a.75.75 0 01.75.75v1.5a.75.75 0 01-1.5 0v-1.5A.75.75 0 0110 15zM10 7a3 3 0 100 6 3 3 0 000-6zM15.657 5.404a.75.75 0 10-1.06-1.06l-1.061 1.06a.75.75 0 001.06 1.06l1.06-1.06zM6.464 14.596a.75.75 0 10-1.06-1.06l-1.06 1.06a.75.75 0 001.06 1.06l1.06-1.06zM18 10a.75.75 0 01-.75.75h-1.5a.75.75 0 010-1.5h1.5A.75.75 0 0118 10zM5 10a.75.75 0 01-.75.75h-1.5a.75.75 0 010-1.5h1.5A.75.75 0 015 10zM14.596 15.657a.75.75 0 001.06-1.06l-1.06-1.061a.75.75 0 10-1.06 1.06l1.06 1.06zM5.404 6.464a.75.75 0 001.06-1.06l-1.06-1.06a.75.75 0 10-1.06 1.06l1.06 1.06z" /></svg>
					{:else}
						<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="w-5 h-5"><path fill-rule="evenodd" d="M7.455 2.004a.75.75 0 01.26.77 7 7 0 009.958 7.967.75.75 0 011.067.853A8.5 8.5 0 116.647 1.921a.75.75 0 01.808.083z" clip-rule="evenodd" /></svg>
					{/if}
				</button>
			</div>
		</div>
	{/if}
</nav>
