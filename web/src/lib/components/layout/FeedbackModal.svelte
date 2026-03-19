<script lang="ts">
	import Modal from '$lib/components/ui/Modal.svelte';
	import { api } from '$lib/api/client';
	import { toast } from '$lib/stores/toast';

	let open = $state(false);
	let feedbackType = $state('bug');
	let message = $state('');
	let allowFollowUp = $state(true);
	let submitting = $state(false);

	async function handleSubmit() {
		if (!message.trim()) return;

		submitting = true;
		try {
			await api.post('/feedback', { type: feedbackType, message: message.trim(), allowFollowUp });
			toast.success('Feedback submitted — thank you!' + (allowFollowUp ? ' A confirmation has been sent to your email.' : ''));
			message = '';
			feedbackType = 'bug';
			allowFollowUp = true;
			open = false;
		} catch {
			toast.error('Failed to submit feedback. Please try again.');
		} finally {
			submitting = false;
		}
	}
</script>

<!-- Floating feedback button -->
<button
	type="button"
	onclick={() => (open = true)}
	class="fixed bottom-6 right-6 z-40 flex h-12 w-12 items-center justify-center rounded-full bg-primary text-white shadow-lg hover:bg-primary-hover transition-colors duration-short ease-out focus:outline-none focus:ring-2 focus:ring-primary focus:ring-offset-2"
	aria-label="Send feedback"
>
	<svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
		<path stroke-linecap="round" stroke-linejoin="round" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
	</svg>
</button>

<Modal bind:open title="Send Feedback">
	<form onsubmit={(e) => { e.preventDefault(); handleSubmit(); }}>
		<div class="space-y-4">
			<div>
				<label for="feedback-type" class="block text-sm font-medium text-neutral-700 mb-1">Type</label>
				<select
					id="feedback-type"
					bind:value={feedbackType}
					class="block w-full rounded-md border border-neutral-300 bg-surface px-3 py-2 text-sm shadow-sm focus:border-primary focus:ring-1 focus:ring-primary"
				>
					<option value="bug">Bug Report</option>
					<option value="feature">Feature Request</option>
					<option value="general">General</option>
				</select>
			</div>

			<div>
				<label for="feedback-message" class="block text-sm font-medium text-neutral-700 mb-1">Message</label>
				<textarea
					id="feedback-message"
					bind:value={message}
					rows="5"
					maxlength="2000"
					required
					placeholder="Describe your feedback..."
					class="block w-full rounded-md border border-neutral-300 px-3 py-2 text-sm shadow-sm placeholder:text-neutral-400 focus:border-primary focus:ring-1 focus:ring-primary"
				></textarea>
				<p class="mt-1 text-xs text-neutral-500">{message.length}/2000</p>
			</div>

			<label class="flex items-start gap-3 cursor-pointer">
				<input
					type="checkbox"
					bind:checked={allowFollowUp}
					class="mt-0.5 rounded border-neutral-300 text-primary focus:ring-primary/40"
				/>
				<span class="text-sm text-neutral-600">You can follow up with me about this feedback</span>
			</label>
		</div>

		<div class="mt-4 flex justify-end gap-3">
			<button
				type="button"
				onclick={() => (open = false)}
				class="rounded-md border border-neutral-300 px-4 py-2 text-sm font-medium text-neutral-700 hover:bg-neutral-50 transition-colors duration-short ease-out"
			>
				Cancel
			</button>
			<button
				type="submit"
				disabled={submitting || !message.trim()}
				class="rounded-md bg-primary px-4 py-2 text-sm font-medium text-white hover:bg-primary-hover disabled:opacity-50 disabled:cursor-not-allowed transition-colors duration-short ease-out"
			>
				{submitting ? 'Submitting...' : 'Submit'}
			</button>
		</div>
	</form>
</Modal>
