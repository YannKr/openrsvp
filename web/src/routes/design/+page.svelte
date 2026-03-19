<script lang="ts">
	import { onMount } from 'svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Textarea from '$lib/components/ui/Textarea.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import Modal from '$lib/components/ui/Modal.svelte';
	import Spinner from '$lib/components/ui/Spinner.svelte';

	let darkMode = $state(false);
	let modalOpen = $state(false);

	// Form state for demos
	let inputNormal = $state('');
	let inputError = $state('');
	let inputDisabled = $state('Pre-filled value');
	let textareaValue = $state('');
	let selectValue = $state('');

	const selectOptions = [
		{ value: 'attending', label: 'Attending' },
		{ value: 'maybe', label: 'Maybe' },
		{ value: 'declined', label: 'Declined' }
	];

	onMount(() => {
		const saved = document.documentElement.getAttribute('data-theme');
		darkMode = saved === 'dark';
	});

	function toggleDarkMode() {
		darkMode = !darkMode;
		document.documentElement.setAttribute('data-theme', darkMode ? 'dark' : 'light');
	}

	// Toast icon paths (same as Toast.svelte)
	const toastIcons: Record<string, string> = {
		success: 'M5 13l4 4L19 7',
		error: 'M6 18L18 6M6 6l12 12',
		info: 'M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z',
		warning:
			'M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L4.082 16.5c-.77.833.192 2.5 1.732 2.5z'
	};

	const spacingScale = [
		{ name: '2xs', value: '4px' },
		{ name: 'xs', value: '8px' },
		{ name: 'sm', value: '12px' },
		{ name: 'md', value: '16px' },
		{ name: 'lg', value: '24px' },
		{ name: 'xl', value: '32px' },
		{ name: '2xl', value: '48px' },
		{ name: '3xl', value: '64px' }
	];
</script>

<svelte:head>
	<title>Design System | OpenRSVP</title>
</svelte:head>

<div class="min-h-screen bg-neutral-50 font-body">
	<!-- Sticky header -->
	<header
		class="sticky top-0 z-40 border-b border-neutral-200 bg-surface/80 backdrop-blur-sm"
	>
		<div class="mx-auto flex max-w-5xl items-center justify-between px-6 py-4">
			<div>
				<h1 class="font-display text-2xl font-bold text-neutral-900">OpenRSVP Design System</h1>
				<p class="text-sm text-neutral-500">Component gallery and token reference</p>
			</div>
			<button
				type="button"
				onclick={toggleDarkMode}
				class="inline-flex items-center gap-2 rounded-md border border-neutral-300 bg-surface px-3 py-2 text-sm font-medium text-neutral-700 shadow-sm transition-colors hover:bg-neutral-50"
				aria-label="Toggle dark mode"
			>
				{#if darkMode}
					<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
						<path stroke-linecap="round" stroke-linejoin="round" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
					</svg>
					Light
				{:else}
					<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
						<path stroke-linecap="round" stroke-linejoin="round" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" />
					</svg>
					Dark
				{/if}
			</button>
		</div>
	</header>

	<main class="mx-auto max-w-5xl px-6 py-12 space-y-16">
		<!-- ============================================================ -->
		<!-- Section 1: Color Palette -->
		<!-- ============================================================ -->
		<section>
			<h2 class="font-display text-2xl font-bold text-neutral-900 mb-2">Color Palette</h2>
			<p class="text-neutral-500 mb-8">Semantic color tokens that adapt to light and dark themes.</p>

			<!-- Primary -->
			<h3 class="font-display text-lg font-semibold text-neutral-800 mb-3">Primary (Rose)</h3>
			<div class="grid grid-cols-2 gap-3 sm:grid-cols-4 mb-8">
				{#each [
					{ bg: 'bg-primary', label: 'Primary', var: '--color-primary' },
					{ bg: 'bg-primary-hover', label: 'Primary Hover', var: '--color-primary-hover' },
					{ bg: 'bg-primary-light', label: 'Primary Light', var: '--color-primary-light' },
					{ bg: 'bg-primary-lighter', label: 'Primary Lighter', var: '--color-primary-lighter' }
				] as swatch}
					<div class="space-y-1.5">
						<div class="{swatch.bg} h-16 rounded-lg shadow-sm border border-neutral-200"></div>
						<p class="text-sm font-medium text-neutral-700">{swatch.label}</p>
						<p class="font-mono text-xs text-neutral-400">{swatch.var}</p>
					</div>
				{/each}
			</div>

			<!-- Secondary -->
			<h3 class="font-display text-lg font-semibold text-neutral-800 mb-3">Secondary (Blue)</h3>
			<div class="grid grid-cols-2 gap-3 sm:grid-cols-3 mb-8">
				{#each [
					{ bg: 'bg-secondary', label: 'Secondary', var: '--color-secondary' },
					{ bg: 'bg-secondary-hover', label: 'Secondary Hover', var: '--color-secondary-hover' },
					{ bg: 'bg-secondary-light', label: 'Secondary Light', var: '--color-secondary-light' }
				] as swatch}
					<div class="space-y-1.5">
						<div class="{swatch.bg} h-16 rounded-lg shadow-sm border border-neutral-200"></div>
						<p class="text-sm font-medium text-neutral-700">{swatch.label}</p>
						<p class="font-mono text-xs text-neutral-400">{swatch.var}</p>
					</div>
				{/each}
			</div>

			<!-- Semantic -->
			<h3 class="font-display text-lg font-semibold text-neutral-800 mb-3">Semantic</h3>
			<div class="grid grid-cols-2 gap-3 sm:grid-cols-4 mb-8">
				{#each [
					{ bg: 'bg-success', label: 'Success', var: '--color-success' },
					{ bg: 'bg-warning', label: 'Warning', var: '--color-warning' },
					{ bg: 'bg-error', label: 'Error', var: '--color-error' },
					{ bg: 'bg-info', label: 'Info', var: '--color-info' }
				] as swatch}
					<div class="space-y-1.5">
						<div class="{swatch.bg} h-16 rounded-lg shadow-sm border border-neutral-200"></div>
						<p class="text-sm font-medium text-neutral-700">{swatch.label}</p>
						<p class="font-mono text-xs text-neutral-400">{swatch.var}</p>
					</div>
				{/each}
			</div>

			<!-- Neutral Scale -->
			<h3 class="font-display text-lg font-semibold text-neutral-800 mb-3">Neutral Scale</h3>
			<div class="grid grid-cols-3 gap-3 sm:grid-cols-6 lg:grid-cols-11">
				{#each [
					{ bg: 'bg-neutral-50', label: '50' },
					{ bg: 'bg-neutral-100', label: '100' },
					{ bg: 'bg-neutral-200', label: '200' },
					{ bg: 'bg-neutral-300', label: '300' },
					{ bg: 'bg-neutral-400', label: '400' },
					{ bg: 'bg-neutral-500', label: '500' },
					{ bg: 'bg-neutral-600', label: '600' },
					{ bg: 'bg-neutral-700', label: '700' },
					{ bg: 'bg-neutral-800', label: '800' },
					{ bg: 'bg-neutral-900', label: '900' },
					{ bg: 'bg-neutral-950', label: '950' }
				] as swatch}
					<div class="space-y-1.5">
						<div class="{swatch.bg} h-16 rounded-lg shadow-sm border border-neutral-200"></div>
						<p class="text-center text-xs font-medium text-neutral-500">{swatch.label}</p>
					</div>
				{/each}
			</div>
		</section>

		<hr class="border-neutral-200" />

		<!-- ============================================================ -->
		<!-- Section 2: Typography -->
		<!-- ============================================================ -->
		<section>
			<h2 class="font-display text-2xl font-bold text-neutral-900 mb-2">Typography</h2>
			<p class="text-neutral-500 mb-8">Three font families for display, body, and code.</p>

			<!-- Display font -->
			<h3 class="font-display text-lg font-semibold text-neutral-800 mb-4">Display -- Satoshi</h3>
			<div class="space-y-4 mb-8 rounded-lg border border-neutral-200 bg-surface p-6">
				<div>
					<p class="font-mono text-xs text-neutral-400 mb-1">font-display text-6xl font-bold</p>
					<p class="font-display text-6xl font-bold text-neutral-900">OpenRSVP</p>
				</div>
				<div>
					<p class="font-mono text-xs text-neutral-400 mb-1">font-display text-4xl font-bold</p>
					<p class="font-display text-4xl font-bold text-neutral-900">Design System</p>
				</div>
				<div>
					<p class="font-mono text-xs text-neutral-400 mb-1">font-display text-2xl font-semibold</p>
					<p class="font-display text-2xl font-semibold text-neutral-900">Event Details</p>
				</div>
			</div>

			<!-- Body font -->
			<h3 class="font-display text-lg font-semibold text-neutral-800 mb-4">Body -- Plus Jakarta Sans</h3>
			<div class="space-y-4 mb-8 rounded-lg border border-neutral-200 bg-surface p-6">
				<div>
					<p class="font-mono text-xs text-neutral-400 mb-1">font-body text-base font-normal (400)</p>
					<p class="font-body text-base font-normal text-neutral-900">
						The quick brown fox jumps over the lazy dog. OpenRSVP is a self-hosted platform for
						creating beautiful event invitations and managing RSVPs with ease.
					</p>
				</div>
				<div>
					<p class="font-mono text-xs text-neutral-400 mb-1">font-body text-base font-medium (500)</p>
					<p class="font-body text-base font-medium text-neutral-900">
						Medium weight for emphasis. Perfect for labels, sub-headings, and important text.
					</p>
				</div>
				<div>
					<p class="font-mono text-xs text-neutral-400 mb-1">font-body text-base font-semibold (600)</p>
					<p class="font-body text-base font-semibold text-neutral-900">
						Semibold weight for stronger emphasis. Used in buttons and navigation items.
					</p>
				</div>
				<div>
					<p class="font-mono text-xs text-neutral-400 mb-1">font-body text-base font-bold (700)</p>
					<p class="font-body text-base font-bold text-neutral-900">
						Bold weight for maximum emphasis. Rarely needed in body text.
					</p>
				</div>
				<div>
					<p class="font-mono text-xs text-neutral-400 mb-1">font-body text-sm font-normal</p>
					<p class="font-body text-sm font-normal text-neutral-700">
						Smaller body text for secondary information, helper text, and metadata. This size is
						commonly used for descriptions beneath headings and form field helpers.
					</p>
				</div>
				<div>
					<p class="font-mono text-xs text-neutral-400 mb-1">font-body text-xs font-medium</p>
					<p class="font-body text-xs font-medium text-neutral-500">
						Extra small text for captions, timestamps, and tertiary information.
					</p>
				</div>
			</div>

			<!-- Mono font -->
			<h3 class="font-display text-lg font-semibold text-neutral-800 mb-4">Mono -- Geist Mono</h3>
			<div class="space-y-3 rounded-lg border border-neutral-200 bg-surface p-6">
				<div>
					<p class="font-mono text-xs text-neutral-400 mb-1">font-mono text-sm font-normal</p>
					<p class="font-mono text-sm text-neutral-900">const id = "01J5K8M2N3P4Q5R6"; // UUIDv7</p>
				</div>
				<div>
					<p class="font-mono text-xs text-neutral-400 mb-1">font-mono text-sm font-semibold</p>
					<p class="font-mono text-sm font-semibold text-neutral-900">--color-primary: #E54666;</p>
				</div>
			</div>
		</section>

		<hr class="border-neutral-200" />

		<!-- ============================================================ -->
		<!-- Section 3: Buttons -->
		<!-- ============================================================ -->
		<section>
			<h2 class="font-display text-2xl font-bold text-neutral-900 mb-2">Buttons</h2>
			<p class="text-neutral-500 mb-8">Five variants across three sizes, with disabled and loading states.</p>

			<!-- Variants -->
			<h3 class="font-display text-lg font-semibold text-neutral-800 mb-4">Variants</h3>
			<div class="flex flex-wrap items-center gap-3 mb-8">
				<Button variant="primary">Primary</Button>
				<Button variant="secondary">Secondary</Button>
				<Button variant="outline">Outline</Button>
				<Button variant="ghost">Ghost</Button>
				<Button variant="danger">Danger</Button>
			</div>

			<!-- Sizes -->
			<h3 class="font-display text-lg font-semibold text-neutral-800 mb-4">Sizes</h3>
			<div class="flex flex-wrap items-center gap-3 mb-8">
				<Button size="sm">Small</Button>
				<Button size="md">Medium</Button>
				<Button size="lg">Large</Button>
			</div>

			<!-- Disabled -->
			<h3 class="font-display text-lg font-semibold text-neutral-800 mb-4">Disabled</h3>
			<div class="flex flex-wrap items-center gap-3 mb-8">
				<Button variant="primary" disabled>Disabled Primary</Button>
				<Button variant="outline" disabled>Disabled Outline</Button>
				<Button variant="danger" disabled>Disabled Danger</Button>
			</div>

			<!-- Loading -->
			<h3 class="font-display text-lg font-semibold text-neutral-800 mb-4">Loading</h3>
			<div class="flex flex-wrap items-center gap-3">
				<Button variant="primary" loading>Saving...</Button>
				<Button variant="secondary" loading>Loading...</Button>
				<Button variant="danger" loading>Deleting...</Button>
			</div>
		</section>

		<hr class="border-neutral-200" />

		<!-- ============================================================ -->
		<!-- Section 4: Form Inputs -->
		<!-- ============================================================ -->
		<section>
			<h2 class="font-display text-2xl font-bold text-neutral-900 mb-2">Form Inputs</h2>
			<p class="text-neutral-500 mb-8">Input, Textarea, and Select components with validation states.</p>

			<div class="grid gap-8 sm:grid-cols-2">
				<!-- Input: Normal -->
				<Input
					label="Full Name"
					name="demo-name"
					placeholder="Jane Doe"
					helper="Enter your full name as it appears on your invitation."
					bind:value={inputNormal}
				/>

				<!-- Input: Required -->
				<Input
					label="Email Address"
					name="demo-email"
					type="email"
					placeholder="jane@example.com"
					required
					helper="We'll send your RSVP confirmation here."
				/>

				<!-- Input: Error -->
				<Input
					label="Phone Number"
					name="demo-phone"
					type="tel"
					placeholder="+1 (555) 000-0000"
					error="Please enter a valid phone number."
					bind:value={inputError}
				/>

				<!-- Input: Disabled -->
				<Input
					label="Event Code"
					name="demo-code"
					disabled
					bind:value={inputDisabled}
					helper="This field cannot be edited."
				/>

				<!-- Textarea -->
				<Textarea
					label="Dietary Restrictions"
					name="demo-dietary"
					placeholder="Let us know about any allergies or dietary preferences..."
					helper="Optional. This helps the host plan the menu."
					bind:value={textareaValue}
				/>

				<!-- Select -->
				<Select
					label="RSVP Status"
					name="demo-status"
					options={selectOptions}
					placeholder="Choose your response..."
					bind:value={selectValue}
				/>

				<!-- Select: Error -->
				<Select
					label="Meal Choice"
					name="demo-meal"
					options={[
						{ value: 'chicken', label: 'Chicken' },
						{ value: 'fish', label: 'Fish' },
						{ value: 'veg', label: 'Vegetarian' }
					]}
					error="Please select a meal option."
				/>

				<!-- Select: Disabled -->
				<Select
					label="Seating Area"
					name="demo-seating"
					options={[{ value: 'main', label: 'Main Hall' }]}
					disabled
				/>
			</div>

			<p class="mt-4 text-sm text-neutral-400 italic">Click into any input to see the focus ring.</p>
		</section>

		<hr class="border-neutral-200" />

		<!-- ============================================================ -->
		<!-- Section 5: Cards & Modal -->
		<!-- ============================================================ -->
		<section>
			<h2 class="font-display text-2xl font-bold text-neutral-900 mb-2">Cards & Modal</h2>
			<p class="text-neutral-500 mb-8">Content containers with optional headers and hover effects.</p>

			<div class="grid gap-6 sm:grid-cols-2 lg:grid-cols-3 mb-8">
				<Card>
					{#snippet header()}
						<h3 class="font-display font-semibold text-neutral-900">Basic Card</h3>
					{/snippet}
					<p class="text-sm text-neutral-600">
						A simple card with a header. Hover to see the subtle lift animation and shadow change.
					</p>
				</Card>

				<Card>
					<div class="text-center">
						<p class="font-display text-3xl font-bold text-primary">142</p>
						<p class="text-sm text-neutral-500 mt-1">Guests Attending</p>
					</div>
				</Card>

				<Card>
					{#snippet header()}
						<div class="flex items-center justify-between">
							<h3 class="font-display font-semibold text-neutral-900">With Actions</h3>
							<Badge variant="success">Active</Badge>
						</div>
					{/snippet}
					<p class="text-sm text-neutral-600 mb-3">
						Cards can contain any content, including other components like badges and buttons.
					</p>
					<Button variant="outline" size="sm">View Details</Button>
				</Card>
			</div>

			<!-- Modal -->
			<h3 class="font-display text-lg font-semibold text-neutral-800 mb-3">Modal</h3>
			<p class="text-sm text-neutral-500 mb-3">
				Modals use a backdrop overlay, escape-to-close, and outside-click-to-close.
			</p>
			<Button variant="outline" onclick={() => (modalOpen = true)}>Open Demo Modal</Button>

			<Modal bind:open={modalOpen} title="Confirm RSVP">
				<p class="text-sm text-neutral-600">
					You are about to confirm your attendance. This will notify the event organizer and reserve
					your spot. You can change your response at any time.
				</p>
				{#snippet actions()}
					<Button variant="ghost" onclick={() => (modalOpen = false)}>Cancel</Button>
					<Button variant="primary" onclick={() => (modalOpen = false)}>Confirm</Button>
				{/snippet}
			</Modal>
		</section>

		<hr class="border-neutral-200" />

		<!-- ============================================================ -->
		<!-- Section 6: Badges -->
		<!-- ============================================================ -->
		<section>
			<h2 class="font-display text-2xl font-bold text-neutral-900 mb-2">Badges</h2>
			<p class="text-neutral-500 mb-8">Status indicators with dot accents across five semantic variants.</p>

			<div class="flex flex-wrap items-center gap-3">
				<Badge variant="success">Confirmed</Badge>
				<Badge variant="warning">Pending</Badge>
				<Badge variant="error">Declined</Badge>
				<Badge variant="info">Invited</Badge>
				<Badge variant="neutral">Draft</Badge>
			</div>
		</section>

		<hr class="border-neutral-200" />

		<!-- ============================================================ -->
		<!-- Section 7: Alerts / Toasts -->
		<!-- ============================================================ -->
		<section>
			<h2 class="font-display text-2xl font-bold text-neutral-900 mb-2">Alerts / Toasts</h2>
			<p class="text-neutral-500 mb-8">
				Inline representations of the four toast types. In production, these appear as floating
				notifications in the bottom-right corner.
			</p>

			<div class="space-y-3 max-w-sm">
				{#each [
					{ type: 'success', bg: 'bg-success', message: 'RSVP confirmed successfully!' },
					{ type: 'warning', bg: 'bg-warning', message: 'Event capacity is almost full.' },
					{ type: 'error', bg: 'bg-error', message: 'Failed to send invitation email.' },
					{ type: 'info', bg: 'bg-info', message: 'New comment on your event.' }
				] as t}
					<div class="px-4 py-3 rounded-md shadow-lg text-white text-sm {t.bg}">
						<div class="flex items-center gap-2">
							<svg class="h-4 w-4 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
								<path
									stroke-linecap="round"
									stroke-linejoin="round"
									stroke-width="2"
									d={toastIcons[t.type]}
								/>
							</svg>
							<span>{t.message}</span>
						</div>
					</div>
				{/each}
			</div>
		</section>

		<hr class="border-neutral-200" />

		<!-- ============================================================ -->
		<!-- Section 8: Border Radius -->
		<!-- ============================================================ -->
		<section>
			<h2 class="font-display text-2xl font-bold text-neutral-900 mb-2">Border Radius</h2>
			<p class="text-neutral-500 mb-8">Consistent rounding tokens from subtle to pill.</p>

			<div class="flex flex-wrap items-end gap-6">
				{#each [
					{ class: 'rounded-sm', label: 'sm', value: '6px' },
					{ class: 'rounded-md', label: 'md', value: '10px' },
					{ class: 'rounded-lg', label: 'lg', value: '14px' },
					{ class: 'rounded-xl', label: 'xl', value: '20px' },
					{ class: 'rounded-full', label: 'full', value: '9999px' }
				] as r}
					<div class="text-center space-y-2">
						<div
							class="h-20 w-20 bg-primary-light border-2 border-primary {r.class}"
						></div>
						<p class="text-sm font-medium text-neutral-700">{r.label}</p>
						<p class="font-mono text-xs text-neutral-400">{r.value}</p>
					</div>
				{/each}
			</div>
		</section>

		<hr class="border-neutral-200" />

		<!-- ============================================================ -->
		<!-- Section 9: Shadows -->
		<!-- ============================================================ -->
		<section>
			<h2 class="font-display text-2xl font-bold text-neutral-900 mb-2">Shadows</h2>
			<p class="text-neutral-500 mb-8">Warm stone-tinted elevation levels for depth hierarchy.</p>

			<div class="grid grid-cols-2 gap-6 sm:grid-cols-4">
				{#each [
					{ class: 'shadow-sm', label: 'shadow-sm' },
					{ class: 'shadow-md', label: 'shadow-md' },
					{ class: 'shadow-lg', label: 'shadow-lg' },
					{ class: 'shadow-xl', label: 'shadow-xl' }
				] as s}
					<div class="text-center space-y-3">
						<div
							class="h-24 rounded-lg bg-surface border border-neutral-100 {s.class}"
						></div>
						<p class="font-mono text-xs text-neutral-500">{s.label}</p>
					</div>
				{/each}
			</div>
		</section>

		<hr class="border-neutral-200" />

		<!-- ============================================================ -->
		<!-- Section 10: Spacing Scale -->
		<!-- ============================================================ -->
		<section>
			<h2 class="font-display text-2xl font-bold text-neutral-900 mb-2">Spacing Scale</h2>
			<p class="text-neutral-500 mb-8">Consistent spacing tokens from compact to generous.</p>

			<div class="space-y-3">
				{#each spacingScale as sp}
					<div class="flex items-center gap-4">
						<span class="w-10 text-right font-mono text-xs text-neutral-400">{sp.name}</span>
						<div
							class="bg-primary rounded-sm"
							style="width: {sp.value}; height: {sp.value};"
						></div>
						<span class="font-mono text-xs text-neutral-500">{sp.value}</span>
					</div>
				{/each}
			</div>
		</section>

		<!-- ============================================================ -->
		<!-- Spinner (bonus) -->
		<!-- ============================================================ -->
		<hr class="border-neutral-200" />

		<section>
			<h2 class="font-display text-2xl font-bold text-neutral-900 mb-2">Spinner</h2>
			<p class="text-neutral-500 mb-8">Loading indicator in three sizes.</p>

			<div class="flex items-center gap-6">
				<div class="text-center space-y-2">
					<Spinner size="sm" class="text-primary" />
					<p class="text-xs text-neutral-500">sm</p>
				</div>
				<div class="text-center space-y-2">
					<Spinner size="md" class="text-primary" />
					<p class="text-xs text-neutral-500">md</p>
				</div>
				<div class="text-center space-y-2">
					<Spinner size="lg" class="text-primary" />
					<p class="text-xs text-neutral-500">lg</p>
				</div>
			</div>
		</section>
	</main>

	<!-- Footer -->
	<footer class="border-t border-neutral-200 py-8 text-center">
		<p class="text-sm text-neutral-400">
			OpenRSVP Design System -- dev-only reference page
		</p>
	</footer>
</div>
