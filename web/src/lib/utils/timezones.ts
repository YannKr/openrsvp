export interface TimezoneOption {
	value: string;
	label: string;
}

/**
 * Comprehensive list of IANA timezones grouped by region,
 * covering major cities across all continents.
 */
export const timezoneOptions: TimezoneOption[] = [
	// Americas
	{ value: 'America/New_York', label: 'Eastern Time - New York (ET)' },
	{ value: 'America/Chicago', label: 'Central Time - Chicago (CT)' },
	{ value: 'America/Denver', label: 'Mountain Time - Denver (MT)' },
	{ value: 'America/Los_Angeles', label: 'Pacific Time - Los Angeles (PT)' },
	{ value: 'America/Anchorage', label: 'Alaska Time (AKT)' },
	{ value: 'Pacific/Honolulu', label: 'Hawaii Time (HT)' },
	{ value: 'America/Phoenix', label: 'Arizona (no DST)' },
	{ value: 'America/Toronto', label: 'Toronto (ET)' },
	{ value: 'America/Vancouver', label: 'Vancouver (PT)' },
	{ value: 'America/Edmonton', label: 'Edmonton (MT)' },
	{ value: 'America/Winnipeg', label: 'Winnipeg (CT)' },
	{ value: 'America/Halifax', label: 'Halifax (AT)' },
	{ value: 'America/St_Johns', label: "St. John's (NT)" },
	{ value: 'America/Mexico_City', label: 'Mexico City (CST)' },
	{ value: 'America/Cancun', label: 'Cancun (EST)' },
	{ value: 'America/Bogota', label: 'Bogota (COT)' },
	{ value: 'America/Lima', label: 'Lima (PET)' },
	{ value: 'America/Santiago', label: 'Santiago (CLT)' },
	{ value: 'America/Argentina/Buenos_Aires', label: 'Buenos Aires (ART)' },
	{ value: 'America/Sao_Paulo', label: 'Sao Paulo (BRT)' },
	{ value: 'America/Caracas', label: 'Caracas (VET)' },

	// Europe
	{ value: 'Europe/London', label: 'London (GMT/BST)' },
	{ value: 'Europe/Dublin', label: 'Dublin (GMT/IST)' },
	{ value: 'Europe/Paris', label: 'Paris (CET)' },
	{ value: 'Europe/Berlin', label: 'Berlin (CET)' },
	{ value: 'Europe/Amsterdam', label: 'Amsterdam (CET)' },
	{ value: 'Europe/Brussels', label: 'Brussels (CET)' },
	{ value: 'Europe/Madrid', label: 'Madrid (CET)' },
	{ value: 'Europe/Rome', label: 'Rome (CET)' },
	{ value: 'Europe/Zurich', label: 'Zurich (CET)' },
	{ value: 'Europe/Vienna', label: 'Vienna (CET)' },
	{ value: 'Europe/Stockholm', label: 'Stockholm (CET)' },
	{ value: 'Europe/Oslo', label: 'Oslo (CET)' },
	{ value: 'Europe/Copenhagen', label: 'Copenhagen (CET)' },
	{ value: 'Europe/Helsinki', label: 'Helsinki (EET)' },
	{ value: 'Europe/Warsaw', label: 'Warsaw (CET)' },
	{ value: 'Europe/Prague', label: 'Prague (CET)' },
	{ value: 'Europe/Budapest', label: 'Budapest (CET)' },
	{ value: 'Europe/Bucharest', label: 'Bucharest (EET)' },
	{ value: 'Europe/Athens', label: 'Athens (EET)' },
	{ value: 'Europe/Istanbul', label: 'Istanbul (TRT)' },
	{ value: 'Europe/Moscow', label: 'Moscow (MSK)' },
	{ value: 'Europe/Lisbon', label: 'Lisbon (WET)' },

	// Africa
	{ value: 'Africa/Cairo', label: 'Cairo (EET)' },
	{ value: 'Africa/Lagos', label: 'Lagos (WAT)' },
	{ value: 'Africa/Nairobi', label: 'Nairobi (EAT)' },
	{ value: 'Africa/Johannesburg', label: 'Johannesburg (SAST)' },
	{ value: 'Africa/Casablanca', label: 'Casablanca (WET)' },
	{ value: 'Africa/Accra', label: 'Accra (GMT)' },

	// Middle East
	{ value: 'Asia/Dubai', label: 'Dubai (GST)' },
	{ value: 'Asia/Riyadh', label: 'Riyadh (AST)' },
	{ value: 'Asia/Tehran', label: 'Tehran (IRST)' },
	{ value: 'Asia/Jerusalem', label: 'Jerusalem (IST)' },
	{ value: 'Asia/Beirut', label: 'Beirut (EET)' },
	{ value: 'Asia/Qatar', label: 'Doha (AST)' },
	{ value: 'Asia/Kuwait', label: 'Kuwait (AST)' },

	// South & Central Asia
	{ value: 'Asia/Kolkata', label: 'India (IST)' },
	{ value: 'Asia/Colombo', label: 'Sri Lanka (IST)' },
	{ value: 'Asia/Dhaka', label: 'Dhaka (BST)' },
	{ value: 'Asia/Kathmandu', label: 'Kathmandu (NPT)' },
	{ value: 'Asia/Karachi', label: 'Karachi (PKT)' },
	{ value: 'Asia/Tashkent', label: 'Tashkent (UZT)' },
	{ value: 'Asia/Almaty', label: 'Almaty (ALMT)' },

	// East & Southeast Asia
	{ value: 'Asia/Shanghai', label: 'China (CST)' },
	{ value: 'Asia/Hong_Kong', label: 'Hong Kong (HKT)' },
	{ value: 'Asia/Taipei', label: 'Taipei (CST)' },
	{ value: 'Asia/Tokyo', label: 'Tokyo (JST)' },
	{ value: 'Asia/Seoul', label: 'Seoul (KST)' },
	{ value: 'Asia/Singapore', label: 'Singapore (SGT)' },
	{ value: 'Asia/Kuala_Lumpur', label: 'Kuala Lumpur (MYT)' },
	{ value: 'Asia/Bangkok', label: 'Bangkok (ICT)' },
	{ value: 'Asia/Ho_Chi_Minh', label: 'Ho Chi Minh City (ICT)' },
	{ value: 'Asia/Jakarta', label: 'Jakarta (WIB)' },
	{ value: 'Asia/Manila', label: 'Manila (PHT)' },

	// Oceania
	{ value: 'Australia/Sydney', label: 'Sydney (AEST)' },
	{ value: 'Australia/Melbourne', label: 'Melbourne (AEST)' },
	{ value: 'Australia/Brisbane', label: 'Brisbane (AEST, no DST)' },
	{ value: 'Australia/Perth', label: 'Perth (AWST)' },
	{ value: 'Australia/Adelaide', label: 'Adelaide (ACST)' },
	{ value: 'Australia/Darwin', label: 'Darwin (ACST, no DST)' },
	{ value: 'Pacific/Auckland', label: 'Auckland (NZST)' },
	{ value: 'Pacific/Fiji', label: 'Fiji (FJT)' },
	{ value: 'Pacific/Guam', label: 'Guam (ChST)' },

	// UTC
	{ value: 'UTC', label: 'UTC' }
];

/** Map of timezone values to labels for quick lookup. */
const tzLabelMap = new Map(timezoneOptions.map((t) => [t.value, t.label]));

/**
 * Returns the display label for a timezone IANA identifier.
 * Falls back to the raw value if not in the predefined list.
 */
export function getTimezoneLabel(value: string): string {
	return tzLabelMap.get(value) || value;
}

/**
 * Returns the full options list, ensuring `extraValue` (e.g. the user's
 * auto-detected browser timezone) is included even if it isn't in the
 * predefined list. Prevents the Select dropdown from having a value
 * that doesn't match any option.
 */
export function getTimezoneOptions(extraValue?: string): TimezoneOption[] {
	if (!extraValue || tzLabelMap.has(extraValue)) {
		return timezoneOptions;
	}
	// Insert the extra timezone at the top so the user sees it immediately.
	return [{ value: extraValue, label: extraValue }, ...timezoneOptions];
}
