<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { api } from '$lib/api/client';
	import { toast } from '$lib/stores/toast';
	import type { InviteCard, Event } from '$lib/types';
	import AppShell from '$lib/components/layout/AppShell.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Textarea from '$lib/components/ui/Textarea.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import Spinner from '$lib/components/ui/Spinner.svelte';
	import InviteCardPreview from '$lib/components/invite/InviteCardPreview.svelte';
	import { onMount } from 'svelte';

	const eventId = $derived($page.params.eventId);

	let loading = $state(true);
	let saving = $state(false);
	let saved = $state(false);
	let event: Event | null = $state(null);

	// Template selection
	let selectedTemplate = $state('balloon-party');

	// Customization fields
	let heading = $state('You\'re Invited!');
	let body = $state('Join us for a wonderful celebration.');
	let footer = $state('We hope to see you there!');
	let primaryColor = $state('#4F46E5');
	let secondaryColor = $state('#EC4899');
	let font = $state('Inter');

	// Background image
	let backgroundImageUrl = $state('');
	let uploading = $state(false);

	const templates = [
		{
			id: 'balloon-party',
			name: 'Balloon Party',
			description: 'Colorful balloons and confetti',
			emoji: '\u{1F388}'
		},
		{
			id: 'confetti',
			name: 'Confetti',
			description: 'Colorful and celebratory',
			emoji: '\u{1F38A}'
		},
		{
			id: 'unicorn-magic',
			name: 'Unicorn Magic',
			description: 'Purple and pink dreamscape',
			emoji: '\u{1F984}'
		},
		{
			id: 'superhero',
			name: 'Superhero',
			description: 'Bold and action-packed',
			emoji: '\u{26A1}'
		},
		{
			id: 'garden-picnic',
			name: 'Garden Picnic',
			description: 'Green and floral',
			emoji: '\u{1F33F}'
		},
		{
			id: 'elegant-affair',
			name: 'Elegant Affair',
			description: 'Refined and sophisticated',
			emoji: '\u{1F48E}'
		},
		{
			id: 'clean-minimal',
			name: 'Clean Minimal',
			description: 'Simple and modern',
			emoji: '\u{25FE}'
		},
		{
			id: 'tropical-vibes',
			name: 'Tropical Vibes',
			description: 'Warm and beachy',
			emoji: '\u{1F334}'
		},
		{
			id: 'vintage-retro',
			name: 'Vintage Retro',
			description: 'Classic sepia tones',
			emoji: '\u{1F4F7}'
		},
		{
			id: 'chalkboard',
			name: 'Chalkboard',
			description: 'Dark and handwritten',
			emoji: '\u{270D}'
		}
	];

	const fontOptions = [
		{ value: 'Inter', label: 'Inter (Modern)' },
		{ value: 'Georgia', label: 'Georgia (Serif)' },
		{ value: 'Courier New', label: 'Courier New (Mono)' },
		{ value: 'Comic Sans MS', label: 'Comic Sans (Fun)' },
		{ value: 'Arial', label: 'Arial (Clean)' }
	];

	const customDataJSON = $derived(
		backgroundImageUrl
			? JSON.stringify({ backgroundImage: backgroundImageUrl })
			: '{}'
	);

	onMount(async () => {
		try {
			const [eventResult, inviteResult] = await Promise.all([
				api.get<{ data: Event }>(`/events/${eventId}`),
				api.get<{ data: InviteCard }>(`/invite/event/${eventId}`).catch(() => null)
			]);
			event = eventResult.data;

			if (inviteResult) {
				const invite = inviteResult.data;
				selectedTemplate = invite.templateId || 'balloon-party';
				heading = invite.heading || heading;
				body = invite.body || body;
				footer = invite.footer || footer;
				primaryColor = invite.primaryColor || primaryColor;
				secondaryColor = invite.secondaryColor || secondaryColor;
				font = invite.font || font;

				// Load existing background image from customData
				try {
					const cd = typeof invite.customData === 'string'
						? JSON.parse(invite.customData || '{}')
						: invite.customData || {};
					if (cd.backgroundImage) {
						backgroundImageUrl = cd.backgroundImage;
					}
				} catch {
					// ignore parse errors
				}
			}
		} catch (err: unknown) {
			const apiErr = err as { message?: string };
			toast.error(apiErr.message || 'Failed to load invite data');
		} finally {
			loading = false;
		}
	});

	async function handleSave() {
		saving = true;
		try {
			await api.put(`/invite/event/${eventId}`, {
				templateId: selectedTemplate,
				heading,
				body,
				footer,
				primaryColor,
				secondaryColor,
				font,
				customData: customDataJSON
			});
			saved = true;
			toast.success('Invite design saved!');
		} catch (err: unknown) {
			const apiErr = err as { message?: string };
			toast.error(apiErr.message || 'Failed to save invite design');
		} finally {
			saving = false;
		}
	}

	async function handleImageUpload(e: globalThis.Event & { currentTarget: HTMLInputElement }) {
		const file = (e.currentTarget as HTMLInputElement).files?.[0];
		if (!file) return;

		if (file.size > 2 * 1024 * 1024) {
			toast.error('Image must be under 2MB');
			return;
		}

		uploading = true;
		try {
			const result = await api.upload<{ data: { url: string } }>(`/invite/event/${eventId}/image`, file);
			backgroundImageUrl = result.data.url;
			toast.success('Background image uploaded!');
		} catch (err: unknown) {
			const apiErr = err as { message?: string };
			toast.error(apiErr.message || 'Failed to upload image');
		} finally {
			uploading = false;
		}
	}

	let publishing = $state(false);

	async function publishAndGo(destination: 'share' | 'dashboard') {
		if (event && event.status === 'draft') {
			publishing = true;
			try {
				const result = await api.post<{ data: Event }>(`/events/${eventId}/publish`);
				event = result.data;
				toast.success('Event published!');
			} catch (err: unknown) {
				const apiErr = err as { message?: string };
				toast.error(apiErr.message || 'Failed to publish event');
				publishing = false;
				return;
			}
			publishing = false;
		}
		if (destination === 'share') {
			goto(`/events/${eventId}/share`);
		} else {
			goto(`/events/${eventId}`);
		}
	}

	function removeBackgroundImage() {
		backgroundImageUrl = '';
	}

	function handleDrop(e: DragEvent) {
		e.preventDefault();
		const file = e.dataTransfer?.files?.[0];
		if (!file || !file.type.startsWith('image/')) return;
		// Trigger the same upload flow
		const dt = new DataTransfer();
		dt.items.add(file);
		const input = document.getElementById('bg-image-input') as HTMLInputElement;
		if (input) {
			input.files = dt.files;
			input.dispatchEvent(new window.Event('change', { bubbles: true }));
		}
	}
</script>

<svelte:head>
	<title>Invite Designer -- OpenRSVP</title>
</svelte:head>

<AppShell>
	<div class="mb-6">
		<a href="/events/{eventId}" class="text-sm text-indigo-600 hover:text-indigo-500">&larr; Back to event</a>
		<h1 class="mt-2 text-2xl font-bold text-slate-900">Invite Designer</h1>
		{#if event}
			<p class="text-sm text-slate-500">{event.title}</p>
		{/if}
	</div>

	{#if loading}
		<div class="flex items-center justify-center py-16">
			<Spinner size="lg" class="text-indigo-500" />
		</div>
	{:else}
		<div class="grid grid-cols-1 lg:grid-cols-2 gap-8">
			<!-- Left: Template picker + customization -->
			<div class="space-y-6">
				<!-- Template picker -->
				<Card>
					{#snippet header()}
						<h2 class="text-lg font-semibold text-slate-900">Choose a Template</h2>
					{/snippet}

					<div class="grid grid-cols-2 sm:grid-cols-3 gap-3">
						{#each templates as template}
							<button
								type="button"
								class="relative rounded-lg border-2 p-3 text-center transition-all {selectedTemplate === template.id
									? 'border-indigo-600 ring-2 ring-indigo-200'
									: 'border-slate-200 hover:border-slate-300'}"
								onclick={() => (selectedTemplate = template.id)}
							>
								<div class="text-2xl mb-1">{template.emoji}</div>
								<p class="text-xs font-medium text-slate-900">{template.name}</p>
								<p class="text-xs text-slate-500 mt-0.5">{template.description}</p>
							</button>
						{/each}
					</div>
				</Card>

				<!-- Customization form -->
				<Card>
					{#snippet header()}
						<h2 class="text-lg font-semibold text-slate-900">Customize</h2>
					{/snippet}

					<div class="space-y-4">
						<Input label="Heading" name="heading" bind:value={heading} placeholder="You're Invited!" />

						<Textarea
							label="Body Text"
							name="body"
							bind:value={body}
							placeholder="Join us for a wonderful celebration..."
							rows={3}
						/>

						<Input label="Footer Text" name="footer" bind:value={footer} placeholder="We hope to see you there!" />

						<div class="grid grid-cols-2 gap-4">
							<div class="space-y-1">
								<label for="primaryColor" class="block text-sm font-medium text-slate-700">Primary Color</label>
								<input
									id="primaryColor"
									type="color"
									bind:value={primaryColor}
									class="h-10 w-full rounded-lg border border-slate-300 cursor-pointer"
								/>
							</div>
							<div class="space-y-1">
								<label for="secondaryColor" class="block text-sm font-medium text-slate-700">Secondary Color</label>
								<input
									id="secondaryColor"
									type="color"
									bind:value={secondaryColor}
									class="h-10 w-full rounded-lg border border-slate-300 cursor-pointer"
								/>
							</div>
						</div>

						<Select label="Font" name="font" bind:value={font} options={fontOptions} />

						<!-- Background Image Upload -->
						<div class="space-y-2">
							<label for="bg-image-input" class="block text-sm font-medium text-slate-700">Background Image</label>
							{#if backgroundImageUrl}
								<div class="relative rounded-lg border border-slate-200 overflow-hidden">
									<img src={backgroundImageUrl} alt="Background preview" class="w-full h-24 object-cover" />
									<button
										type="button"
										onclick={removeBackgroundImage}
										class="absolute top-1 right-1 rounded-full bg-white/90 p-1 text-slate-500 hover:text-red-500 shadow-sm transition-colors"
										aria-label="Remove background image"
									>
										<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
											<path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
										</svg>
									</button>
								</div>
							{:else}
								<!-- svelte-ignore a11y_no_static_element_interactions -->
								<div
									class="rounded-lg border-2 border-dashed border-slate-300 p-4 text-center hover:border-indigo-400 transition-colors cursor-pointer"
									ondragover={(e) => e.preventDefault()}
									ondrop={handleDrop}
								>
									<input
										id="bg-image-input"
										type="file"
										accept="image/jpeg,image/png,image/webp"
										class="hidden"
										onchange={handleImageUpload}
									/>
									<label for="bg-image-input" class="cursor-pointer">
										{#if uploading}
											<div class="flex items-center justify-center gap-2 text-indigo-500">
												<Spinner size="sm" />
												<span class="text-sm">Uploading...</span>
											</div>
										{:else}
											<svg class="w-8 h-8 mx-auto text-slate-400 mb-1" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
												<path stroke-linecap="round" stroke-linejoin="round" d="M2.25 15.75l5.159-5.159a2.25 2.25 0 013.182 0l5.159 5.159m-1.5-1.5l1.409-1.409a2.25 2.25 0 013.182 0l2.909 2.909M3.75 21h16.5A2.25 2.25 0 0022.5 18.75V5.25A2.25 2.25 0 0020.25 3H3.75A2.25 2.25 0 001.5 5.25v13.5A2.25 2.25 0 003.75 21z" />
											</svg>
											<p class="text-xs text-slate-500">Drop an image or click to upload</p>
											<p class="text-xs text-slate-400 mt-0.5">JPEG, PNG, or WebP (max 2MB)</p>
										{/if}
									</label>
								</div>
							{/if}
						</div>
					</div>
				</Card>

				<Button onclick={handleSave} loading={saving} class="w-full">Save Invite Design</Button>

				{#if saved}
					<Card class="mt-4">
						<div class="text-center space-y-3">
							<div class="flex items-center justify-center gap-2 text-green-600">
								<svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
									<path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
								</svg>
								<span class="text-sm font-medium">Design saved</span>
							</div>
							<p class="text-xs text-slate-500">What would you like to do next?</p>
							<div class="flex flex-col gap-2">
								{#if event && event.status === 'draft'}
									<Button onclick={() => publishAndGo('share')} loading={publishing} variant="primary" size="sm" class="w-full">Publish & Share</Button>
									<Button onclick={() => publishAndGo('dashboard')} variant="outline" size="sm" class="w-full">Publish & View Dashboard</Button>
								{:else}
									<Button href="/events/{eventId}/share" variant="primary" size="sm" class="w-full">Share & Get QR Code</Button>
									<Button href="/events/{eventId}" variant="outline" size="sm" class="w-full">View Event Dashboard</Button>
								{/if}
							</div>
						</div>
					</Card>
				{/if}
			</div>

			<!-- Right: Live preview -->
			<div>
				<div class="sticky top-8">
					<h2 class="text-lg font-semibold text-slate-900 mb-4">Preview</h2>
					<InviteCardPreview
						templateId={selectedTemplate}
						{heading}
						{body}
						{footer}
						{primaryColor}
						{secondaryColor}
						{font}
						eventTitle={event?.title || ''}
						eventDate={event?.eventDate || ''}
						eventLocation={event?.location || ''}
						customData={customDataJSON}
						timezone={event?.timezone}
					/>
				</div>
			</div>
		</div>
	{/if}
</AppShell>
