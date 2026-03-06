<script lang="ts">
	import { api } from '$lib/api/client';
	import { toast } from '$lib/stores/toast';
	import { formatDateTime } from '$lib/utils/dates';
	import type { EventSeries, Event } from '$lib/types';
	import AppShell from '$lib/components/layout/AppShell.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import Spinner from '$lib/components/ui/Spinner.svelte';
	import { onMount } from 'svelte';

	let loading = $state(true);
	let seriesList: EventSeries[] = $state([]);

	const recurrenceLabels: Record<string, string> = {
		weekly: 'Weekly',
		biweekly: 'Every 2 weeks',
		monthly: 'Monthly'
	};

	onMount(async () => {
		try {
			const result = await api.get<{ data: EventSeries[] }>('/events/series');
			seriesList = result.data;
		} catch (err: unknown) {
			const apiErr = err as { message?: string };
			toast.error(apiErr.message || 'Failed to load series');
		} finally {
			loading = false;
		}
	});
</script>

<svelte:head>
	<title>Recurring Series -- OpenRSVP</title>
</svelte:head>

<AppShell>
	<div class="flex items-center justify-between mb-8">
		<div>
			<a href="/events" class="text-sm text-indigo-600 hover:text-indigo-500">&larr; Back to events</a>
			<h1 class="mt-2 text-2xl font-bold text-slate-900">Recurring Series</h1>
		</div>
		<Button href="/events/series/new">Create Series</Button>
	</div>

	{#if loading}
		<div class="flex items-center justify-center py-16">
			<Spinner size="lg" class="text-indigo-500" />
		</div>
	{:else if seriesList.length === 0}
		<Card>
			<div class="text-center py-12">
				<svg
					class="mx-auto h-12 w-12 text-slate-400"
					fill="none"
					viewBox="0 0 24 24"
					stroke="currentColor"
				>
					<path
						stroke-linecap="round"
						stroke-linejoin="round"
						stroke-width="1.5"
						d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
					/>
				</svg>
				<h3 class="mt-4 text-lg font-medium text-slate-900">No recurring series yet</h3>
				<p class="mt-2 text-sm text-slate-500">Create a series to automatically generate recurring events.</p>
				<div class="mt-6">
					<Button href="/events/series/new">Create Your First Series</Button>
				</div>
			</div>
		</Card>
	{:else}
		<div class="grid grid-cols-1 md:grid-cols-2 gap-6">
			{#each seriesList as series (series.id)}
				<a href="/events/series/{series.id}" class="block group">
					<Card class="transition-shadow group-hover:shadow-md">
						<div class="flex items-start justify-between">
							<div class="flex-1 min-w-0">
								<h3
									class="text-lg font-semibold text-slate-900 group-hover:text-indigo-600 transition-colors truncate"
								>
									{series.title}
								</h3>
								<p class="mt-1 text-sm text-slate-600">
									{recurrenceLabels[series.recurrenceRule] || series.recurrenceRule} at {series.eventTime}
								</p>
								{#if series.location}
									<p class="mt-1 text-sm text-slate-500 flex items-center gap-1">
										<svg
											class="h-4 w-4 shrink-0"
											fill="none"
											viewBox="0 0 24 24"
											stroke="currentColor"
										>
											<path
												stroke-linecap="round"
												stroke-linejoin="round"
												stroke-width="2"
												d="M17.657 16.657L13.414 20.9a1.998 1.998 0 01-2.827 0l-4.244-4.243a8 8 0 1111.314 0z"
											/>
											<path
												stroke-linecap="round"
												stroke-linejoin="round"
												stroke-width="2"
												d="M15 11a3 3 0 11-6 0 3 3 0 016 0z"
											/>
										</svg>
										<span class="truncate">{series.location}</span>
									</p>
								{/if}
							</div>
							<Badge variant={series.seriesStatus === 'active' ? 'success' : 'neutral'}>
								{series.seriesStatus}
							</Badge>
						</div>
						<div class="mt-4 flex items-center justify-between text-xs text-slate-500">
							<span>{recurrenceLabels[series.recurrenceRule] || series.recurrenceRule}</span>
							{#if series.maxOccurrences}
								<span>{series.maxOccurrences} occurrences max</span>
							{:else if series.recurrenceEnd}
								<span>Until {formatDateTime(series.recurrenceEnd)}</span>
							{:else}
								<span>No end date</span>
							{/if}
						</div>
					</Card>
				</a>
			{/each}
		</div>
	{/if}
</AppShell>
