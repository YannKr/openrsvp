<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api/client';
	import { toast } from '$lib/stores/toast';
	import { getTimezoneOptions } from '$lib/utils/timezones';
	import type { ApiResponse } from '$lib/types';
	import AppShell from '$lib/components/layout/AppShell.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import Spinner from '$lib/components/ui/Spinner.svelte';

	interface SetupConfig {
		instanceName: string;
		defaultTimezone: string;
		allowSignups: boolean;
		supportEmail: string;
		configured: boolean;
	}

	const browserTz = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
	const tzOptions = getTimezoneOptions(browserTz);

	let loading = $state(true);
	// True when the API rejected the config read with 401/403 — operator must log in as admin.
	let needsAdmin = $state(false);
	let submitting = $state(false);

	// Whether the instance has already been configured. Drives the wizard vs. settings framing.
	let configured = $state(false);

	// Form fields
	let instanceName = $state('');
	let defaultTimezone = $state(browserTz);
	let allowSignups = $state(true);
	let supportEmail = $state('');

	let errors: Record<string, string> = $state({});

	const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

	onMount(async () => {
		try {
			// Public endpoint: tells us whether this is a first-run instance.
			const status = await api.get<ApiResponse<{ configured: boolean }>>('/setup/status');
			configured = status.data.configured;
		} catch {
			// Non-fatal: fall back to first-run framing if status is unavailable.
		}

		try {
			// Admin-only endpoint. Load current values to prefill the form.
			const cfg = await api.get<ApiResponse<SetupConfig>>('/setup/config');
			instanceName = cfg.data.instanceName ?? '';
			defaultTimezone = cfg.data.defaultTimezone || browserTz;
			// An unsaved instance reports false; keep the env default (on) for first run.
			allowSignups = cfg.data.configured ? cfg.data.allowSignups : true;
			supportEmail = cfg.data.supportEmail ?? '';
		} catch (e: unknown) {
			const apiErr = e as { status?: number };
			if (apiErr.status === 401 || apiErr.status === 403) {
				needsAdmin = true;
			} else {
				toast.error('Failed to load instance settings');
			}
		} finally {
			loading = false;
		}
	});

	function validate(): boolean {
		errors = {};
		if (!instanceName.trim()) {
			errors.instanceName = 'Instance name is required';
		}
		if (supportEmail.trim() && !EMAIL_RE.test(supportEmail.trim())) {
			errors.supportEmail = 'Enter a valid email address';
		}
		return Object.keys(errors).length === 0;
	}

	async function handleSubmit() {
		if (!validate()) return;
		submitting = true;
		try {
			await api.post('/setup/config', {
				instanceName: instanceName.trim(),
				defaultTimezone: defaultTimezone,
				allowSignups: allowSignups,
				supportEmail: supportEmail.trim()
			});
			configured = true;
			toast.success('Instance configured');
		} catch (e: unknown) {
			const apiErr = e as { status?: number; message?: string };
			if (apiErr.status === 401 || apiErr.status === 403) {
				needsAdmin = true;
				toast.error('You must be signed in as an admin to save settings');
			} else {
				toast.error(apiErr.message || 'Failed to save settings');
			}
		} finally {
			submitting = false;
		}
	}
</script>

<svelte:head>
	<title>{configured ? 'Instance Settings' : 'Setup'} -- OpenRSVP</title>
</svelte:head>

<AppShell>
	<div class="max-w-2xl mx-auto">
		{#if loading}
			<div class="flex items-center justify-center py-20">
				<Spinner />
			</div>
		{:else if needsAdmin}
			<Card>
				<div class="text-center py-6 space-y-4">
					<h1 class="text-xl font-bold font-display text-neutral-900">Admin sign-in required</h1>
					<p class="text-sm text-neutral-500">
						Instance configuration can only be changed by an administrator. Please sign in
						with an admin account to continue.
					</p>
					<div class="pt-2">
						<Button href="/auth/login">Sign in as admin</Button>
					</div>
				</div>
			</Card>
		{:else}
			<div class="mb-8">
				<h1 class="text-2xl font-bold font-display text-neutral-900">
					{configured ? 'Instance Settings' : 'Welcome to OpenRSVP'}
				</h1>
				<p class="mt-2 text-sm text-neutral-500">
					{#if configured}
						Update your instance-wide configuration. These settings apply to every event and
						organizer on this server.
					{:else}
						Let's get your instance set up. These settings apply across the whole server and
						can be changed later from this page.
					{/if}
				</p>
			</div>

			<Card>
				<div class="space-y-6">
					<Input
						label="Instance name"
						name="instanceName"
						bind:value={instanceName}
						placeholder="Acme Events"
						helper="Shown to organizers and guests across the instance."
						error={errors.instanceName || ''}
						required
					/>

					<div>
						<Select
							label="Default timezone"
							name="defaultTimezone"
							bind:value={defaultTimezone}
							options={tzOptions}
						/>
						<p class="mt-1 text-sm text-neutral-500">Used as the default when creating new events.</p>
					</div>

					<fieldset class="pt-1">
						<legend class="text-sm font-medium text-neutral-700 mb-2">Organizer signups</legend>
						<label class="flex items-start gap-3 cursor-pointer">
							<input
								type="checkbox"
								bind:checked={allowSignups}
								class="mt-0.5 rounded border-neutral-300 text-primary focus:ring-primary/40"
							/>
							<div>
								<span class="text-sm text-neutral-700">Allow new organizer signups</span>
								<p class="text-xs text-neutral-400">
									When off, only existing organizers can sign in. New accounts must be
									invited.
								</p>
							</div>
						</label>
					</fieldset>

					<Input
						label="Support email (optional)"
						name="supportEmail"
						type="email"
						bind:value={supportEmail}
						placeholder="support@example.com"
						helper="Shown to users who need help. Leave blank to hide."
						error={errors.supportEmail || ''}
					/>
				</div>

				<div class="mt-8 flex items-center justify-end border-t border-neutral-200 pt-6">
					<Button onclick={handleSubmit} loading={submitting}>
						{configured ? 'Save settings' : 'Finish setup'}
					</Button>
				</div>
			</Card>
		{/if}
	</div>
</AppShell>
