<script lang="ts">
	import { page } from '$app/stores';
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

	const templates = [
		{
			id: 'balloon-party',
			name: 'Balloon Party',
			description: 'Colorful balloons and confetti',
			emoji: '🎈',
			bgClass: 'from-yellow-100 via-pink-100 to-blue-100',
			accentClass: 'text-pink-600'
		},
		{
			id: 'confetti',
			name: 'Confetti',
			description: 'Colorful and celebratory',
			emoji: '🎊',
			bgClass: 'from-purple-100 via-pink-100 to-orange-100',
			accentClass: 'text-purple-600'
		},
		{
			id: 'unicorn-magic',
			name: 'Unicorn Magic',
			description: 'Purple and pink dreamscape',
			emoji: '🦄',
			bgClass: 'from-purple-200 via-pink-200 to-indigo-200',
			accentClass: 'text-purple-700'
		},
		{
			id: 'superhero',
			name: 'Superhero',
			description: 'Bold and action-packed',
			emoji: '⚡',
			bgClass: 'from-red-100 via-yellow-100 to-blue-100',
			accentClass: 'text-red-600'
		},
		{
			id: 'garden-picnic',
			name: 'Garden Picnic',
			description: 'Green and floral',
			emoji: '🌿',
			bgClass: 'from-green-100 via-emerald-50 to-lime-100',
			accentClass: 'text-green-700'
		}
	];

	const fontOptions = [
		{ value: 'Inter', label: 'Inter (Modern)' },
		{ value: 'Georgia', label: 'Georgia (Serif)' },
		{ value: 'Courier New', label: 'Courier New (Mono)' },
		{ value: 'Comic Sans MS', label: 'Comic Sans (Fun)' },
		{ value: 'Arial', label: 'Arial (Clean)' }
	];

	let currentTemplate = $derived(templates.find((t) => t.id === selectedTemplate) || templates[0]);

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
				font
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
									<Button href="/events/{eventId}" variant="primary" size="sm" class="w-full">Publish Event</Button>
								{/if}
								<Button href="/events/{eventId}/share" variant="outline" size="sm" class="w-full">Share & Get QR Code</Button>
								<Button href="/events/{eventId}" variant="outline" size="sm" class="w-full">View Event Dashboard</Button>
							</div>
						</div>
					</Card>
				{/if}
			</div>

			<!-- Right: Live preview -->
			<div>
				<div class="sticky top-8">
					<h2 class="text-lg font-semibold text-slate-900 mb-4">Preview</h2>
					<div
						class="rounded-2xl overflow-hidden shadow-lg border border-slate-200 bg-gradient-to-br {currentTemplate.bgClass}"
						style="font-family: {font}, sans-serif;"
					>
						<div class="p-8 text-center">
							<!-- Template emoji decoration -->
							<div class="text-4xl mb-4">{currentTemplate.emoji}{currentTemplate.emoji}{currentTemplate.emoji}</div>

							<!-- Heading -->
							<h3 class="text-2xl font-bold mb-4" style="color: {primaryColor};">
								{heading || 'Your Heading'}
							</h3>

							<!-- Event details -->
							{#if event}
								<p class="text-lg font-semibold text-slate-800 mb-2">{event.title}</p>
								{#if event.location}
									<p class="text-sm text-slate-600 mb-1">{event.location}</p>
								{/if}
							{/if}

							<!-- Divider -->
							<div class="my-6 mx-auto w-16 h-0.5" style="background-color: {secondaryColor};"></div>

							<!-- Body -->
							<p class="text-slate-700 whitespace-pre-wrap mb-6">{body || 'Your message here...'}</p>

							<!-- RSVP Button mock -->
							<div
								class="inline-block rounded-full px-8 py-3 text-white font-medium text-sm shadow-md"
								style="background-color: {primaryColor};"
							>
								RSVP Now
							</div>

							<!-- Footer -->
							<p class="mt-6 text-xs" style="color: {secondaryColor};">
								{footer || 'Footer text'}
							</p>
						</div>
					</div>
				</div>
			</div>
		</div>
	{/if}
</AppShell>
