<script lang="ts">
	import { page } from '$app/stores';
	import { api } from '$lib/api/client';
	import { toast } from '$lib/stores/toast';
	import { formatDateTime } from '$lib/utils/dates';
	import type { Message, Event } from '$lib/types';
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
	let sending = $state(false);
	let event: Event | null = $state(null);
	let messages: Message[] = $state([]);

	// Compose form
	let recipientType = $state('all');
	let subject = $state('');
	let body = $state('');

	let composeErrors: Record<string, string> = $state({});

	const recipientOptions = [
		{ value: 'all', label: 'All Attendees' },
		{ value: 'attending', label: 'Attending' },
		{ value: 'maybe', label: 'Maybe' },
		{ value: 'declined', label: 'Declined' },
		{ value: 'pending', label: 'Pending RSVP' }
	];

	onMount(async () => {
		try {
			const [eventResult, messagesResult] = await Promise.all([
				api.get<{ data: Event }>(`/events/${eventId}`),
				api.get<{ data: Message[] }>(`/messages/event/${eventId}`).catch(() => ({ data: [] }))
			]);
			event = eventResult.data;
			messages = messagesResult.data;
		} catch (err: unknown) {
			const apiErr = err as { message?: string };
			toast.error(apiErr.message || 'Failed to load messages');
		} finally {
			loading = false;
		}
	});

	async function handleSend() {
		composeErrors = {};
		if (!subject.trim()) composeErrors.subject = 'Subject is required';
		if (!body.trim()) composeErrors.body = 'Message body is required';
		if (Object.keys(composeErrors).length > 0) return;

		sending = true;
		try {
			const result = await api.post<{ data: Message }>(`/messages/event/${eventId}`, {
				recipientType: 'group',
				recipientId: recipientType,
				subject: subject.trim(),
				body: body.trim()
			});
			messages = [result.data, ...messages];
			subject = '';
			body = '';
			toast.success('Message sent!');
		} catch (err: unknown) {
			const apiErr = err as { message?: string };
			toast.error(apiErr.message || 'Failed to send message');
		} finally {
			sending = false;
		}
	}
</script>

<svelte:head>
	<title>Messages -- OpenRSVP</title>
</svelte:head>

<AppShell>
	<div class="max-w-3xl mx-auto">
		<div class="mb-6">
			<a href="/events/{eventId}" class="text-sm text-indigo-600 hover:text-indigo-500">&larr; Back to event</a>
			<h1 class="mt-2 text-2xl font-bold text-slate-900">Messages</h1>
			{#if event}
				<p class="text-sm text-slate-500">{event.title}</p>
			{/if}
		</div>

		{#if loading}
			<div class="flex items-center justify-center py-16">
				<Spinner size="lg" class="text-indigo-500" />
			</div>
		{:else}
			<!-- Compose form -->
			<Card class="mb-6">
				{#snippet header()}
					<h2 class="text-lg font-semibold text-slate-900">Compose Message</h2>
				{/snippet}

				<form
					onsubmit={(e) => {
						e.preventDefault();
						handleSend();
					}}
					class="space-y-4"
				>
					<Select
						label="Send To"
						name="recipientType"
						bind:value={recipientType}
						options={recipientOptions}
					/>

					<Input
						label="Subject"
						name="subject"
						bind:value={subject}
						placeholder="Message subject"
						error={composeErrors.subject || ''}
						required
					/>

					<Textarea
						label="Message"
						name="body"
						bind:value={body}
						placeholder="Write your message..."
						rows={4}
						error={composeErrors.body || ''}
						required
					/>

					<div class="flex justify-end">
						<Button type="submit" loading={sending}>Send Message</Button>
					</div>
				</form>
			</Card>

			<!-- Message list -->
			<Card>
				{#snippet header()}
					<h2 class="text-lg font-semibold text-slate-900">Sent Messages</h2>
				{/snippet}

				{#if messages.length === 0}
					<p class="text-sm text-slate-500 text-center py-8">No messages sent yet.</p>
				{:else}
					<div class="divide-y divide-slate-200 -mx-6 -mb-4">
						{#each messages as message (message.id)}
							<div class="px-6 py-4">
								<div class="flex items-start justify-between">
									<div class="flex-1 min-w-0">
										<p class="text-sm font-medium text-slate-900">{message.subject}</p>
										<p class="text-xs text-slate-500 mt-0.5">
											To: {message.recipientType === 'group' ? message.recipientId : message.recipientType}
											&middot; {formatDateTime(message.createdAt)}
										</p>
									</div>
								</div>
								<p class="mt-2 text-sm text-slate-700 whitespace-pre-wrap">{message.body}</p>
							</div>
						{/each}
					</div>
				{/if}
			</Card>
		{/if}
	</div>
</AppShell>
