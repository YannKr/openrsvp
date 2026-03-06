<script lang="ts">
	import { page } from '$app/stores';
	import { api } from '$lib/api/client';
	import { toast } from '$lib/stores/toast';
	import { formatDateTime } from '$lib/utils/dates';
	import type { Message, Event, Attendee } from '$lib/types';
	import AppShell from '$lib/components/layout/AppShell.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Textarea from '$lib/components/ui/Textarea.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import Spinner from '$lib/components/ui/Spinner.svelte';
	import { onMount, tick } from 'svelte';

	const eventId = $derived($page.params.eventId);

	let loading = $state(true);
	let sending = $state(false);
	let event: Event | null = $state(null);
	let messages: Message[] = $state([]);
	let attendeeMap: Record<string, string> = $state({});

	// Compose form
	let recipientType = $state('all');
	let subject = $state('');
	let body = $state('');

	// Reply state
	let replyToAttendeeId = $state('');
	let replyToAttendeeName = $state('');

	let composeErrors: Record<string, string> = $state({});

	let composeForm: HTMLFormElement | undefined = $state();

	const recipientOptions = [
		{ value: 'all', label: 'All Attendees' },
		{ value: 'attending', label: 'Attending' },
		{ value: 'maybe', label: 'Maybe' },
		{ value: 'declined', label: 'Declined' },
		{ value: 'pending', label: 'Pending RSVP' }
	];

	const recipientLabels: Record<string, string> = {
		all: 'All Attendees',
		attending: 'Attending',
		maybe: 'Maybe',
		declined: 'Declined',
		pending: 'Pending RSVP'
	};

	function attendeeName(id: string): string {
		return attendeeMap[id] || 'Unknown';
	}

	function messageLabel(msg: Message): string {
		if (msg.senderType === 'attendee') {
			return 'From: ' + attendeeName(msg.senderId);
		}
		if (msg.recipientType === 'attendee') {
			return 'To: ' + attendeeName(msg.recipientId);
		}
		return 'To: ' + (recipientLabels[msg.recipientId] || msg.recipientId);
	}

	function isIncoming(msg: Message): boolean {
		return msg.senderType === 'attendee';
	}

	async function handleReply(msg: Message) {
		replyToAttendeeId = msg.senderId;
		replyToAttendeeName = attendeeName(msg.senderId);
		subject = msg.subject.startsWith('Re: ') ? msg.subject : 'Re: ' + msg.subject;
		body = '';
		await tick();
		composeForm?.scrollIntoView({ behavior: 'smooth', block: 'start' });
		// Focus the body textarea after scrolling
		setTimeout(() => {
			const textarea = composeForm?.querySelector('textarea');
			textarea?.focus();
		}, 300);
	}

	function cancelReply() {
		replyToAttendeeId = '';
		replyToAttendeeName = '';
		subject = '';
		body = '';
	}

	onMount(async () => {
		try {
			const [eventResult, messagesResult, rsvpResult] = await Promise.all([
				api.get<{ data: Event }>(`/events/${eventId}`),
				api.get<{ data: Message[] }>(`/messages/event/${eventId}`).catch(() => ({ data: [] })),
				api.get<{ data: Attendee[] }>(`/rsvp/event/${eventId}`).catch(() => ({ data: [] }))
			]);
			event = eventResult.data;
			messages = messagesResult.data;
			// Build attendee lookup map
			const map: Record<string, string> = {};
			for (const a of rsvpResult.data) {
				map[a.id] = a.name;
			}
			attendeeMap = map;
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
			const payload = replyToAttendeeId
				? {
						recipientType: 'attendee',
						recipientId: replyToAttendeeId,
						subject: subject.trim(),
						body: body.trim()
					}
				: {
						recipientType: 'group',
						recipientId: recipientType,
						subject: subject.trim(),
						body: body.trim()
					};

			const result = await api.post<{ data: Message }>(`/messages/event/${eventId}`, payload);
			messages = [result.data, ...messages];
			subject = '';
			body = '';
			replyToAttendeeId = '';
			replyToAttendeeName = '';
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
					bind:this={composeForm}
					onsubmit={(e) => {
						e.preventDefault();
						handleSend();
					}}
					class="space-y-4"
				>
					{#if replyToAttendeeId}
						<div class="flex items-center gap-2">
							<span class="inline-flex items-center gap-1 rounded-full bg-indigo-100 px-3 py-1 text-sm font-medium text-indigo-800">
								Replying to {replyToAttendeeName}
								<button
									type="button"
									onclick={cancelReply}
									class="ml-1 inline-flex h-4 w-4 items-center justify-center rounded-full text-indigo-600 hover:bg-indigo-200 hover:text-indigo-800"
									aria-label="Cancel reply"
								>
									&times;
								</button>
							</span>
						</div>
					{:else}
						<Select
							label="Send To"
							name="recipientType"
							bind:value={recipientType}
							options={recipientOptions}
						/>
					{/if}

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
					<h2 class="text-lg font-semibold text-slate-900">All Messages</h2>
				{/snippet}

				{#if messages.length === 0}
					<p class="text-sm text-slate-500 text-center py-8">No messages yet.</p>
				{:else}
					<div class="divide-y divide-slate-200 -mx-6 -mb-4">
						{#each messages as message (message.id)}
							<div class="px-6 py-4 {isIncoming(message) ? 'bg-indigo-50/50' : ''}">
								<div class="flex items-start justify-between">
									<div class="flex-1 min-w-0">
										<p class="text-sm font-medium text-slate-900">{message.subject}</p>
										<p class="text-xs text-slate-500 mt-0.5">
											{messageLabel(message)}
											&middot; {formatDateTime(message.createdAt)}
										</p>
									</div>
									{#if isIncoming(message)}
										<button
											type="button"
											onclick={() => handleReply(message)}
											class="ml-3 shrink-0 text-xs font-medium text-indigo-600 hover:text-indigo-500"
										>
											Reply
										</button>
									{/if}
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
