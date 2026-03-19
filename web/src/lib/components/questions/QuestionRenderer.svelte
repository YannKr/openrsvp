<script lang="ts">
	import type { EventQuestion } from '$lib/types';

	interface Props {
		questions: EventQuestion[];
		answers: Record<string, string>;
	}

	let { questions, answers = $bindable({}) }: Props = $props();

	function handleTextInput(questionId: string, value: string) {
		answers = { ...answers, [questionId]: value };
	}

	function handleRadioChange(questionId: string, value: string) {
		answers = { ...answers, [questionId]: value };
	}

	function handleCheckboxChange(questionId: string, option: string, checked: boolean) {
		let current: string[] = [];
		try {
			current = JSON.parse(answers[questionId] || '[]');
		} catch {
			current = [];
		}

		if (checked) {
			current = [...current, option];
		} else {
			current = current.filter((v) => v !== option);
		}

		answers = { ...answers, [questionId]: JSON.stringify(current) };
	}

	function isCheckboxChecked(questionId: string, option: string): boolean {
		try {
			const current: string[] = JSON.parse(answers[questionId] || '[]');
			return current.includes(option);
		} catch {
			return false;
		}
	}
</script>

{#if questions.length > 0}
	<div class="space-y-5">
		{#each questions as question (question.id)}
			<div>
				{#if question.type === 'text'}
					<label for="question-{question.id}" class="block text-sm font-medium text-neutral-700 mb-1.5">
						{question.label}
						{#if question.required}
							<span class="text-error">*</span>
						{/if}
					</label>
					<input
						id="question-{question.id}"
						type="text"
						value={answers[question.id] || ''}
						oninput={(e) => handleTextInput(question.id, (e.target as HTMLInputElement).value)}
						maxlength={1000}
						required={question.required}
						placeholder="Your answer"
						class="w-full rounded-md border border-neutral-300 px-4 py-2.5 text-neutral-900 placeholder:text-neutral-400 focus:outline-none focus:ring-2 focus:ring-primary/40 focus:border-primary transition-colors duration-short ease-out"
					/>
				{:else if question.type === 'select'}
					<fieldset>
						<legend class="block text-sm font-medium text-neutral-700 mb-1.5">
							{question.label}
							{#if question.required}
								<span class="text-error">*</span>
							{/if}
						</legend>
						<div class="space-y-2 mt-1">
							{#each question.options as option}
								<label class="flex items-center gap-3 cursor-pointer">
									<input
										type="radio"
										name="question-{question.id}"
										value={option}
										checked={answers[question.id] === option}
										onchange={() => handleRadioChange(question.id, option)}
										required={question.required}
										class="border-neutral-300 text-primary focus:ring-primary/40"
									/>
									<span class="text-sm text-neutral-700">{option}</span>
								</label>
							{/each}
						</div>
					</fieldset>
				{:else if question.type === 'checkbox'}
					<fieldset>
						<legend class="block text-sm font-medium text-neutral-700 mb-1.5">
							{question.label}
							{#if question.required}
								<span class="text-error">*</span>
							{/if}
						</legend>
						<div class="space-y-2 mt-1">
							{#each question.options as option}
								<label class="flex items-center gap-3 cursor-pointer">
									<input
										type="checkbox"
										checked={isCheckboxChecked(question.id, option)}
										onchange={(e) => handleCheckboxChange(question.id, option, (e.target as HTMLInputElement).checked)}
										class="rounded border-neutral-300 text-primary focus:ring-primary/40"
									/>
									<span class="text-sm text-neutral-700">{option}</span>
								</label>
							{/each}
						</div>
					</fieldset>
				{/if}
			</div>
		{/each}
	</div>
{/if}
