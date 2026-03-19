<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api/client';
	import { toast } from '$lib/stores/toast';
	import type { EventQuestion } from '$lib/types';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import Spinner from '$lib/components/ui/Spinner.svelte';

	interface Props {
		eventId: string;
	}

	let { eventId }: Props = $props();

	let questions: EventQuestion[] = $state([]);
	let loading = $state(true);

	// New question form
	let newLabel = $state('');
	let newType = $state<'text' | 'select' | 'checkbox'>('text');
	let newOptions: string[] = $state(['']);
	let newRequired = $state(false);
	let saving = $state(false);

	// Inline editing
	let editingId: string | null = $state(null);
	let editLabel = $state('');
	let editType = $state<'text' | 'select' | 'checkbox'>('text');
	let editOptions: string[] = $state([]);
	let editRequired = $state(false);
	let editSaving = $state(false);

	const typeOptions = [
		{ value: 'text', label: 'Text' },
		{ value: 'select', label: 'Multiple Choice' },
		{ value: 'checkbox', label: 'Checkboxes' }
	];

	const atLimit = $derived(questions.length >= 10);

	onMount(async () => {
		await loadQuestions();
	});

	async function loadQuestions() {
		try {
			const result = await api.get<{ data: EventQuestion[] }>(`/events/${eventId}/questions`);
			questions = result.data ?? [];
		} catch {
			// Questions endpoint may not exist yet or event has none
			questions = [];
		} finally {
			loading = false;
		}
	}

	function addOption() {
		newOptions = [...newOptions, ''];
	}

	function removeOption(index: number) {
		newOptions = newOptions.filter((_, i) => i !== index);
	}

	function updateOption(index: number, value: string) {
		newOptions = newOptions.map((o, i) => (i === index ? value : o));
	}

	function addEditOption() {
		editOptions = [...editOptions, ''];
	}

	function removeEditOption(index: number) {
		editOptions = editOptions.filter((_, i) => i !== index);
	}

	function updateEditOption(index: number, value: string) {
		editOptions = editOptions.map((o, i) => (i === index ? value : o));
	}

	async function handleAddQuestion() {
		if (!newLabel.trim()) {
			toast.error('Question label is required');
			return;
		}

		if ((newType === 'select' || newType === 'checkbox') && newOptions.filter((o) => o.trim()).length < 2) {
			toast.error('Please add at least 2 options');
			return;
		}

		saving = true;
		try {
			const body = {
				label: newLabel.trim(),
				type: newType,
				options: newType === 'text' ? [] : newOptions.filter((o) => o.trim()).map((o) => o.trim()),
				required: newRequired,
				sortOrder: questions.length
			};
			const result = await api.post<{ data: EventQuestion }>(`/events/${eventId}/questions`, body);
			questions = [...questions, result.data];
			// Reset form
			newLabel = '';
			newType = 'text';
			newOptions = [''];
			newRequired = false;
			toast.success('Question added');
		} catch (err: unknown) {
			const apiErr = err as { message?: string };
			toast.error(apiErr.message || 'Failed to add question');
		} finally {
			saving = false;
		}
	}

	function startEdit(q: EventQuestion) {
		editingId = q.id;
		editLabel = q.label;
		editType = q.type;
		editOptions = q.options && q.options.length > 0 ? [...q.options] : [''];
		editRequired = q.required;
	}

	function cancelEdit() {
		editingId = null;
	}

	async function saveEdit() {
		if (!editingId) return;
		if (!editLabel.trim()) {
			toast.error('Question label is required');
			return;
		}

		if ((editType === 'select' || editType === 'checkbox') && editOptions.filter((o) => o.trim()).length < 2) {
			toast.error('Please add at least 2 options');
			return;
		}

		editSaving = true;
		try {
			const body = {
				label: editLabel.trim(),
				type: editType,
				options: editType === 'text' ? [] : editOptions.filter((o) => o.trim()).map((o) => o.trim()),
				required: editRequired
			};
			const result = await api.put<{ data: EventQuestion }>(`/events/${eventId}/questions/${editingId}`, body);
			questions = questions.map((q) => (q.id === editingId ? result.data : q));
			editingId = null;
			toast.success('Question updated');
		} catch (err: unknown) {
			const apiErr = err as { message?: string };
			toast.error(apiErr.message || 'Failed to update question');
		} finally {
			editSaving = false;
		}
	}

	async function deleteQuestion(qId: string) {
		try {
			await api.delete(`/events/${eventId}/questions/${qId}`);
			questions = questions.filter((q) => q.id !== qId);
			toast.success('Question deleted');
		} catch (err: unknown) {
			const apiErr = err as { message?: string };
			toast.error(apiErr.message || 'Failed to delete question');
		}
	}

	async function moveQuestion(index: number, direction: 'up' | 'down') {
		const swapIndex = direction === 'up' ? index - 1 : index + 1;
		if (swapIndex < 0 || swapIndex >= questions.length) return;

		const reordered = [...questions];
		[reordered[index], reordered[swapIndex]] = [reordered[swapIndex], reordered[index]];
		const questionIds = reordered.map((q) => q.id);

		// Optimistically update
		questions = reordered;

		try {
			await api.put(`/events/${eventId}/questions/reorder`, { questionIds });
		} catch (err: unknown) {
			const apiErr = err as { message?: string };
			toast.error(apiErr.message || 'Failed to reorder questions');
			await loadQuestions();
		}
	}

	function typeLabel(type: string): string {
		switch (type) {
			case 'text': return 'Text';
			case 'select': return 'Multiple Choice';
			case 'checkbox': return 'Checkboxes';
			default: return type;
		}
	}
</script>

<div class="mt-8">
	<h2 class="text-lg font-display font-semibold text-neutral-900 mb-4">Custom RSVP Questions</h2>

	{#if loading}
		<div class="flex items-center justify-center py-8">
			<Spinner size="md" class="text-primary" />
		</div>
	{:else}
		<!-- Existing questions -->
		{#if questions.length > 0}
			<div class="space-y-3 mb-6">
				{#each questions as question, index (question.id)}
					{#if editingId === question.id}
						<!-- Inline edit mode -->
						<div class="bg-neutral-50 rounded-md border border-neutral-200 p-4 space-y-4">
							<Input
								label="Question Label"
								name="edit-question-label"
								bind:value={editLabel}
								placeholder="e.g. What song should we add to the playlist?"
								required
							/>

							<Select
								label="Question Type"
								name="edit-question-type"
								bind:value={editType}
								options={typeOptions}
							/>

							{#if editType === 'select' || editType === 'checkbox'}
								<div>
									<span class="block text-sm font-medium text-neutral-700 mb-2">Options</span>
									<div class="space-y-2">
										{#each editOptions as option, optIndex}
											<div class="flex items-center gap-2">
												<input
													type="text"
													value={option}
													oninput={(e) => updateEditOption(optIndex, (e.target as HTMLInputElement).value)}
													placeholder="Option {optIndex + 1}"
													class="flex-1 rounded-md border border-neutral-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary/40 focus:border-primary"
												/>
												{#if editOptions.length > 1}
													<button
														type="button"
														onclick={() => removeEditOption(optIndex)}
														class="text-neutral-400 hover:text-error transition-colors duration-short ease-out p-1"
														aria-label="Remove option"
													>
														<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
															<path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
														</svg>
													</button>
												{/if}
											</div>
										{/each}
									</div>
									<button
										type="button"
										onclick={addEditOption}
										class="mt-2 text-sm text-primary hover:text-primary-hover font-medium"
									>
										+ Add option
									</button>
								</div>
							{/if}

							<label class="flex items-center gap-3 cursor-pointer">
								<input
									type="checkbox"
									bind:checked={editRequired}
									class="rounded border-neutral-300 text-primary focus:ring-primary/40"
								/>
								<span class="text-sm text-neutral-700">Required</span>
							</label>

							<div class="flex items-center justify-end gap-2 pt-2">
								<Button variant="outline" size="sm" onclick={cancelEdit}>Cancel</Button>
								<Button size="sm" onclick={saveEdit} loading={editSaving}>Save</Button>
							</div>
						</div>
					{:else}
						<!-- Display mode -->
						<div class="bg-neutral-50 rounded-md border border-neutral-200 p-4">
							<div class="flex items-start justify-between">
								<div class="flex-1 min-w-0">
									<div class="flex items-center gap-2 mb-1">
										<p class="text-sm font-medium text-neutral-900">{question.label}</p>
										{#if question.required}
											<Badge variant="info">Required</Badge>
										{/if}
									</div>
									<p class="text-xs text-neutral-500">{typeLabel(question.type)}</p>
									{#if question.options && question.options.length > 0}
										<p class="text-xs text-neutral-400 mt-1">
											Options: {question.options.join(', ')}
										</p>
									{/if}
								</div>
								<div class="flex items-center gap-1 ml-4 flex-shrink-0">
									<button
										type="button"
										onclick={() => moveQuestion(index, 'up')}
										disabled={index === 0}
										class="p-1 text-neutral-400 hover:text-neutral-600 transition-colors duration-short ease-out disabled:opacity-30 disabled:cursor-not-allowed"
										aria-label="Move up"
									>
										<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
											<path stroke-linecap="round" stroke-linejoin="round" d="M5 15l7-7 7 7" />
										</svg>
									</button>
									<button
										type="button"
										onclick={() => moveQuestion(index, 'down')}
										disabled={index === questions.length - 1}
										class="p-1 text-neutral-400 hover:text-neutral-600 transition-colors duration-short ease-out disabled:opacity-30 disabled:cursor-not-allowed"
										aria-label="Move down"
									>
										<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
											<path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
										</svg>
									</button>
									<Button size="sm" variant="ghost" onclick={() => startEdit(question)}>Edit</Button>
									<Button size="sm" variant="ghost" onclick={() => deleteQuestion(question.id)}>
										<span class="text-error">Delete</span>
									</Button>
								</div>
							</div>
						</div>
					{/if}
				{/each}
			</div>
		{/if}

		<!-- Add question form -->
		{#if !atLimit}
			<div class="border border-dashed border-neutral-300 rounded-md p-4 space-y-4">
				<p class="text-sm font-medium text-neutral-700">Add a Question</p>

				<Input
					label="Question Label"
					name="new-question-label"
					bind:value={newLabel}
					placeholder="e.g. What song should we add to the playlist?"
					required
				/>

				<Select
					label="Question Type"
					name="new-question-type"
					bind:value={newType}
					options={typeOptions}
				/>

				{#if newType === 'select' || newType === 'checkbox'}
					<div>
						<span class="block text-sm font-medium text-neutral-700 mb-2">Options</span>
						<div class="space-y-2">
							{#each newOptions as option, optIndex}
								<div class="flex items-center gap-2">
									<input
										type="text"
										value={option}
										oninput={(e) => updateOption(optIndex, (e.target as HTMLInputElement).value)}
										placeholder="Option {optIndex + 1}"
										class="flex-1 rounded-md border border-neutral-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary/40 focus:border-primary"
									/>
									{#if newOptions.length > 1}
										<button
											type="button"
											onclick={() => removeOption(optIndex)}
											class="text-neutral-400 hover:text-error transition-colors duration-short ease-out p-1"
											aria-label="Remove option"
										>
											<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
												<path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
											</svg>
										</button>
									{/if}
								</div>
							{/each}
						</div>
						<button
							type="button"
							onclick={addOption}
							class="mt-2 text-sm text-primary hover:text-primary-hover font-medium"
						>
							+ Add option
						</button>
					</div>
				{/if}

				<label class="flex items-center gap-3 cursor-pointer">
					<input
						type="checkbox"
						bind:checked={newRequired}
						class="rounded border-neutral-300 text-primary focus:ring-primary/40"
					/>
					<span class="text-sm text-neutral-700">Required</span>
				</label>

				<div class="flex justify-end">
					<Button size="sm" onclick={handleAddQuestion} loading={saving}>Add Question</Button>
				</div>
			</div>
		{:else}
			<p class="text-xs text-neutral-400 text-center py-2">Maximum of 10 questions reached.</p>
		{/if}

		{#if questions.length > 0}
			<p class="text-xs text-neutral-400 mt-3">{questions.length} / 10 questions</p>
		{/if}
	{/if}
</div>
