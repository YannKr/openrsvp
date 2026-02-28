export function formatDate(date: string): string {
	return new Date(date).toLocaleDateString('en-US', {
		weekday: 'long',
		year: 'numeric',
		month: 'long',
		day: 'numeric'
	});
}

export function formatTime(date: string): string {
	return new Date(date).toLocaleTimeString('en-US', {
		hour: 'numeric',
		minute: '2-digit',
		hour12: true
	});
}

export function formatDateTime(date: string): string {
	return `${formatDate(date)} at ${formatTime(date)}`;
}

export function isInFuture(date: string): boolean {
	return new Date(date) > new Date();
}

export function isInPast(date: string): boolean {
	return new Date(date) < new Date();
}

export function daysUntil(date: string): number {
	const diff = new Date(date).getTime() - Date.now();
	return Math.ceil(diff / (1000 * 60 * 60 * 24));
}

export function toISOLocal(date: Date): string {
	const offset = date.getTimezoneOffset();
	const local = new Date(date.getTime() - offset * 60 * 1000);
	return local.toISOString().slice(0, 16);
}
