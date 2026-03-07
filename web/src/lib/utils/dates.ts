export function formatDate(date: string, timezone?: string): string {
	const opts: Intl.DateTimeFormatOptions = {
		weekday: 'long',
		year: 'numeric',
		month: 'long',
		day: 'numeric'
	};
	if (timezone) opts.timeZone = timezone;
	return new Date(date).toLocaleDateString('en-US', opts);
}

export function formatTime(date: string, timezone?: string): string {
	const opts: Intl.DateTimeFormatOptions = {
		hour: 'numeric',
		minute: '2-digit',
		hour12: true
	};
	if (timezone) opts.timeZone = timezone;
	return new Date(date).toLocaleTimeString('en-US', opts);
}

export function formatDateTime(date: string, timezone?: string): string {
	return `${formatDate(date, timezone)} at ${formatTime(date, timezone)}`;
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

/**
 * Interprets a datetime-local input value as being in the specified IANA
 * timezone and returns a UTC ISO string. Uses Intl.DateTimeFormat with
 * iterative offset convergence to handle DST correctly.
 */
export function datetimeLocalToUTC(datetimeLocal: string, timezone: string): string {
	// Parse the datetime-local value (e.g., "2026-03-15T11:11")
	const [datePart, timePart] = datetimeLocal.split('T');
	const [year, month, day] = datePart.split('-').map(Number);
	const [hour, minute] = timePart.split(':').map(Number);

	// Create a rough UTC guess: treat the local values as UTC initially
	let guess = new Date(Date.UTC(year, month - 1, day, hour, minute));

	// Iteratively converge on the correct offset (handles DST transitions)
	for (let i = 0; i < 2; i++) {
		const offset = getTimezoneOffsetMinutes(guess, timezone);
		guess = new Date(Date.UTC(year, month - 1, day, hour, minute + offset));
	}

	return guess.toISOString();
}

/**
 * Converts a UTC ISO string to a datetime-local string in the specified
 * timezone. Used to populate datetime-local inputs on the edit page.
 */
export function utcToDatetimeLocal(utcDate: string, timezone: string): string {
	const date = new Date(utcDate);
	const formatter = new Intl.DateTimeFormat('en-CA', {
		timeZone: timezone,
		year: 'numeric',
		month: '2-digit',
		day: '2-digit',
		hour: '2-digit',
		minute: '2-digit',
		hour12: false
	});

	const parts = formatter.formatToParts(date);
	const get = (type: Intl.DateTimeFormatPartTypes) =>
		parts.find(p => p.type === type)?.value ?? '00';

	const y = get('year');
	const m = get('month');
	const d = get('day');
	let h = get('hour');
	const min = get('minute');

	// Some locales return "24" for midnight — normalize to "00"
	if (h === '24') h = '00';

	return `${y}-${m}-${d}T${h}:${min}`;
}

/**
 * Returns the offset in minutes to subtract from a wall-clock time in the
 * given timezone to get UTC. Positive means the timezone is behind UTC.
 */
function getTimezoneOffsetMinutes(date: Date, timezone: string): number {
	const formatter = new Intl.DateTimeFormat('en-CA', {
		timeZone: timezone,
		year: 'numeric',
		month: '2-digit',
		day: '2-digit',
		hour: '2-digit',
		minute: '2-digit',
		second: '2-digit',
		hour12: false
	});

	const parts = formatter.formatToParts(date);
	const get = (type: Intl.DateTimeFormatPartTypes) =>
		parseInt(parts.find(p => p.type === type)?.value ?? '0');

	let h = get('hour');
	if (h === 24) h = 0;

	const localInTz = Date.UTC(
		get('year'),
		get('month') - 1,
		get('day'),
		h,
		get('minute'),
		get('second')
	);

	// offset = UTC - local-in-tz (in minutes)
	return (date.getTime() - localInTz) / 60000;
}
