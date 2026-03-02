import { writable } from 'svelte/store';
import { api } from '$lib/api/client';

export const smsEnabled = writable(false);

let loaded = false;

export async function loadAppConfig() {
	if (loaded) return;
	try {
		const result = await api.get<{ data: { smsEnabled: boolean } }>('/config');
		smsEnabled.set(result.data.smsEnabled);
		loaded = true;
	} catch {
		smsEnabled.set(false);
	}
}
