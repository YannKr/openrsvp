<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api/client';
	import type { InstanceStats, ApiResponse } from '$lib/types';
	import AppShell from '$lib/components/layout/AppShell.svelte';
	import Spinner from '$lib/components/ui/Spinner.svelte';

	let stats = $state<InstanceStats | null>(null);
	let loading = $state(true);
	let error = $state('');

	onMount(async () => {
		try {
			const res = await api.get<ApiResponse<InstanceStats>>('/admin/stats');
			stats = res.data;
		} catch (e: any) {
			error = e?.message || e?.error || 'Failed to load statistics';
		} finally {
			loading = false;
		}
	});

	function pct(value: number, total: number): string {
		if (total === 0) return '0';
		return Math.round((value / total) * 100).toString();
	}

	function barWidth(value: number, total: number): string {
		if (total === 0) return '0%';
		return `${Math.max(2, Math.round((value / total) * 100))}%`;
	}

	const eventStatusItems = $derived(stats ? [
		{ label: 'Published', value: stats.events.published, color: 'bg-emerald-500' },
		{ label: 'Draft', value: stats.events.draft, color: 'bg-slate-400' },
		{ label: 'Cancelled', value: stats.events.cancelled, color: 'bg-red-400' },
		{ label: 'Archived', value: stats.events.archived, color: 'bg-amber-400' },
	] : []);

	const rsvpItems = $derived(stats ? [
		{ label: 'Attending', value: stats.attendees.attending, color: 'bg-emerald-500' },
		{ label: 'Maybe', value: stats.attendees.maybe, color: 'bg-amber-400' },
		{ label: 'Declined', value: stats.attendees.declined, color: 'bg-red-400' },
		{ label: 'Pending', value: stats.attendees.pending, color: 'bg-blue-400' },
		{ label: 'Waitlisted', value: stats.attendees.waitlisted, color: 'bg-purple-400' },
	] : []);

	const notifItems = $derived(stats ? [
		{ label: 'Total', value: stats.notifications.total, color: 'text-slate-900' },
		{ label: 'Sent', value: stats.notifications.sent, color: 'text-blue-600' },
		{ label: 'Delivered', value: stats.notifications.delivered, color: 'text-emerald-600' },
		{ label: 'Opened', value: stats.notifications.opened, color: 'text-indigo-600' },
		{ label: 'Bounced', value: stats.notifications.bounced, color: 'text-red-600' },
		{ label: 'Complained', value: stats.notifications.complained, color: 'text-orange-600' },
		{ label: 'Failed', value: stats.notifications.failed, color: 'text-red-700' },
	] : []);

	const featureItems = $derived(stats ? [
		{ label: 'Waitlist', value: stats.features.waitlistEvents },
		{ label: 'Comments', value: stats.features.commentsEnabledEvents },
		{ label: 'Co-hosted', value: stats.features.cohostedEvents },
		{ label: 'Custom Questions', value: stats.features.eventsWithQuestions },
		{ label: 'Capacity Limit', value: stats.features.eventsWithCapacity },
		{ label: 'Series', value: stats.features.seriesEvents },
	] : []);
</script>

<AppShell>
	<div class="space-y-8">
		<div>
			<h1 class="text-2xl font-bold text-slate-900">Instance Admin</h1>
			<p class="text-sm text-slate-500 mt-1">Aggregate statistics across the entire OpenRSVP instance. All data is anonymous.</p>
		</div>

		{#if loading}
			<div class="flex items-center justify-center py-20">
				<Spinner />
			</div>
		{:else if error}
			<div class="bg-red-50 border border-red-200 rounded-xl p-4 text-red-800">
				{error}
			</div>
		{:else if stats}
			<!-- Metric Cards -->
			<div class="grid grid-cols-2 lg:grid-cols-4 gap-4">
				<div class="bg-white rounded-xl border border-slate-200 p-5">
					<p class="text-sm font-medium text-slate-500">Total Events</p>
					<p class="text-3xl font-bold text-slate-900 mt-1">{stats.events.total}</p>
				</div>
				<div class="bg-white rounded-xl border border-slate-200 p-5">
					<p class="text-sm font-medium text-slate-500">Total Guests</p>
					<p class="text-3xl font-bold text-slate-900 mt-1">{stats.attendees.totalHeadcount}</p>
					<p class="text-xs text-slate-400 mt-1">{stats.attendees.total} RSVPs + plus-ones</p>
				</div>
				<div class="bg-white rounded-xl border border-slate-200 p-5">
					<p class="text-sm font-medium text-slate-500">Organizers</p>
					<p class="text-3xl font-bold text-slate-900 mt-1">{stats.organizers.total}</p>
				</div>
				<div class="bg-white rounded-xl border border-slate-200 p-5">
					<p class="text-sm font-medium text-slate-500">Avg Guests / Event</p>
					<p class="text-3xl font-bold text-slate-900 mt-1">{stats.attendees.avgPerEvent.toFixed(1)}</p>
				</div>
			</div>

			<!-- Charts Row -->
			<div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
				<!-- Events by Status -->
				<div class="bg-white rounded-xl border border-slate-200 p-6">
					<h2 class="text-sm font-semibold text-slate-700 mb-4">Events by Status</h2>
					<div class="space-y-3">
						{#each eventStatusItems as item}
							<div class="flex items-center gap-3">
								<span class="text-sm text-slate-600 w-24 shrink-0">{item.label}</span>
								<div class="flex-1 bg-slate-100 rounded-full h-5 overflow-hidden">
									<div
										class="{item.color} h-full rounded-full transition-all"
										style="width: {barWidth(item.value, stats.events.total)}"
									></div>
								</div>
								<span class="text-sm font-medium text-slate-700 w-16 text-right">
									{item.value}
									<span class="text-slate-400 text-xs">({pct(item.value, stats.events.total)}%)</span>
								</span>
							</div>
						{/each}
					</div>
				</div>

				<!-- RSVP Distribution -->
				<div class="bg-white rounded-xl border border-slate-200 p-6">
					<h2 class="text-sm font-semibold text-slate-700 mb-4">RSVP Distribution</h2>
					<div class="space-y-3">
						{#each rsvpItems as item}
							<div class="flex items-center gap-3">
								<span class="text-sm text-slate-600 w-24 shrink-0">{item.label}</span>
								<div class="flex-1 bg-slate-100 rounded-full h-5 overflow-hidden">
									<div
										class="{item.color} h-full rounded-full transition-all"
										style="width: {barWidth(item.value, stats.attendees.total)}"
									></div>
								</div>
								<span class="text-sm font-medium text-slate-700 w-16 text-right">
									{item.value}
									<span class="text-slate-400 text-xs">({pct(item.value, stats.attendees.total)}%)</span>
								</span>
							</div>
						{/each}
					</div>
				</div>
			</div>

			<!-- Notification Health -->
			<div class="bg-white rounded-xl border border-slate-200 p-6">
				<h2 class="text-sm font-semibold text-slate-700 mb-4">Notification Health</h2>
				{#if stats.notifications.total === 0}
					<p class="text-sm text-slate-400">No notifications sent yet.</p>
				{:else}
					<div class="grid grid-cols-2 sm:grid-cols-4 lg:grid-cols-7 gap-4">
						{#each notifItems as item}
							<div class="text-center">
								<p class="text-2xl font-bold {item.color}">{item.value}</p>
								<p class="text-xs text-slate-500 mt-1">{item.label}</p>
							</div>
						{/each}
					</div>
				{/if}
			</div>

			<!-- Feature Adoption -->
			<div class="bg-white rounded-xl border border-slate-200 p-6">
				<h2 class="text-sm font-semibold text-slate-700 mb-4">Feature Adoption</h2>
				<div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-4">
					{#each featureItems as item}
						<div class="text-center p-3 bg-slate-50 rounded-lg">
							<p class="text-2xl font-bold text-slate-900">{item.value}</p>
							<p class="text-xs text-slate-500 mt-1">{item.label}</p>
							{#if stats.events.total > 0}
								<p class="text-xs text-indigo-500 mt-0.5">{pct(item.value, stats.events.total)}% of events</p>
							{/if}
						</div>
					{/each}
				</div>
			</div>
		{/if}
	</div>
</AppShell>
