import type { ApiError } from '$lib/types';

const BASE_URL = '/api/v1';
const TOKEN_KEY = 'openrsvp_session';

class ApiClient {
	private token: string = '';

	constructor() {
		if (typeof window !== 'undefined') {
			this.token = localStorage.getItem(TOKEN_KEY) || '';
		}
	}

	setToken(token: string) {
		this.token = token;
		if (typeof window !== 'undefined') {
			if (token) {
				localStorage.setItem(TOKEN_KEY, token);
			} else {
				localStorage.removeItem(TOKEN_KEY);
			}
		}
	}

	getToken(): string {
		return this.token;
	}

	async request<T>(path: string, options: RequestInit = {}): Promise<T> {
		const url = `${BASE_URL}${path}`;
		const headers: Record<string, string> = {
			'Content-Type': 'application/json',
			...((options.headers as Record<string, string>) || {})
		};

		if (this.token) {
			headers['Authorization'] = `Bearer ${this.token}`;
		}

		const response = await fetch(url, {
			...options,
			headers
		});

		if (!response.ok) {
			const error: ApiError = await response.json().catch(() => ({
				error: 'unknown',
				message: response.statusText,
				status: response.status
			}));
			throw error;
		}

		if (response.status === 204) {
			return undefined as T;
		}

		return response.json();
	}

	get<T>(path: string) {
		return this.request<T>(path, { method: 'GET' });
	}

	post<T>(path: string, body?: unknown) {
		return this.request<T>(path, {
			method: 'POST',
			body: body ? JSON.stringify(body) : undefined
		});
	}

	put<T>(path: string, body?: unknown) {
		return this.request<T>(path, {
			method: 'PUT',
			body: body ? JSON.stringify(body) : undefined
		});
	}

	patch<T>(path: string, body?: unknown) {
		return this.request<T>(path, {
			method: 'PATCH',
			body: body ? JSON.stringify(body) : undefined
		});
	}

	delete<T>(path: string) {
		return this.request<T>(path, { method: 'DELETE' });
	}
}

export const api = new ApiClient();
