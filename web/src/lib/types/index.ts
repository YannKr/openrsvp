export interface Organizer {
	id: string;
	email: string;
	name: string;
	timezone: string;
	isAdmin: boolean;
	createdAt: string;
	updatedAt: string;
}

export interface Event {
	id: string;
	organizerId: string;
	title: string;
	description: string;
	eventDate: string;
	endDate?: string;
	location: string;
	timezone: string;
	retentionDays: number;
	contactRequirement: 'email' | 'phone' | 'email_or_phone' | 'email_and_phone';
	showHeadcount: boolean;
	showGuestList: boolean;
	status: 'draft' | 'published' | 'cancelled' | 'archived';
	shareToken: string;
	rsvpDeadline?: string;
	maxCapacity?: number;
	waitlistEnabled: boolean;
	commentsEnabled: boolean;
	seriesId?: string;
	seriesIndex?: number;
	seriesOverride?: boolean;
	createdAt: string;
	updatedAt: string;
}

export interface EventSeries {
	id: string;
	organizerId: string;
	title: string;
	description: string;
	location: string;
	timezone: string;
	eventTime: string;
	durationMinutes?: number;
	recurrenceRule: 'weekly' | 'biweekly' | 'monthly';
	recurrenceEnd?: string;
	maxOccurrences?: number;
	seriesStatus: 'active' | 'stopped';
	retentionDays: number;
	contactRequirement: 'email' | 'phone' | 'email_or_phone' | 'email_and_phone';
	showHeadcount: boolean;
	showGuestList: boolean;
	rsvpDeadlineOffsetHours?: number;
	maxCapacity?: number;
	createdAt: string;
	updatedAt: string;
}

export interface InviteCard {
	id: string;
	eventId: string;
	templateId: string;
	heading: string;
	body: string;
	footer: string;
	primaryColor: string;
	secondaryColor: string;
	font: string;
	customData: Record<string, unknown>;
	createdAt: string;
	updatedAt: string;
}

export interface Attendee {
	id: string;
	eventId: string;
	name: string;
	email?: string;
	phone?: string;
	rsvpStatus: 'pending' | 'attending' | 'maybe' | 'declined' | 'waitlisted';
	rsvpToken: string;
	contactMethod: 'email' | 'sms';
	dietaryNotes: string;
	plusOnes: number;
	createdAt: string;
	updatedAt: string;
}

export interface Message {
	id: string;
	eventId: string;
	senderType: 'organizer' | 'attendee';
	senderId: string;
	recipientType: 'organizer' | 'attendee' | 'group';
	recipientId: string;
	subject: string;
	body: string;
	readAt?: string;
	createdAt: string;
}

export interface Reminder {
	id: string;
	eventId: string;
	remindAt: string;
	targetGroup: 'all' | 'attending' | 'maybe' | 'declined' | 'pending';
	message: string;
	status: 'scheduled' | 'sent' | 'cancelled' | 'failed';
	createdAt: string;
	updatedAt: string;
}

export interface RSVPStats {
	attending: number;
	attendingHeadcount: number;
	maybe: number;
	maybeHeadcount: number;
	declined: number;
	pending: number;
	waitlisted: number;
	total: number;
	totalHeadcount: number;
}

export interface PublicEvent {
	title: string;
	description: string;
	eventDate: string;
	endDate?: string;
	location: string;
	timezone: string;
	contactRequirement: 'email' | 'phone' | 'email_or_phone' | 'email_and_phone';
	rsvpDeadline?: string;
	rsvpsClosed: boolean;
	maxCapacity?: number;
	spotsLeft?: number;
	atCapacity: boolean;
	waitlistEnabled: boolean;
	commentsEnabled: boolean;
}

export interface PublicAttendance {
	headcount: number;
	names?: string[];
}

export interface CoHost {
	id: string;
	eventId: string;
	organizerId: string;
	role: 'cohost';
	addedBy: string;
	organizerEmail: string;
	organizerName: string;
	createdAt: string;
}

export interface ApiError {
	error: string;
	message: string;
	status: number;
}

export interface ApiResponse<T> {
	data: T;
}

export interface PaginatedResponse<T> {
	data: T[];
	total: number;
	page: number;
	perPage: number;
}

export interface EventQuestion {
	id: string;
	eventId: string;
	label: string;
	type: 'text' | 'select' | 'checkbox';
	options: string[];
	required: boolean;
	sortOrder: number;
	createdAt: string;
	updatedAt: string;
}

export interface QuestionAnswer {
	id: string;
	attendeeId: string;
	questionId: string;
	answer: string;
	createdAt: string;
	updatedAt: string;
}

export type InviteTemplate = {
	id: string;
	name: string;
	description: string;
	previewImage: string;
};

export interface PublicComment {
	id: string;
	authorName: string;
	body: string;
	createdAt: string;
}

export interface EventComment {
	id: string;
	eventId: string;
	attendeeId: string;
	authorName: string;
	body: string;
	createdAt: string;
}

export interface PaginatedComments {
	comments: PublicComment[];
	hasMore: boolean;
	nextCursor?: string;
}

export interface Webhook {
	id: string;
	eventId: string;
	url: string;
	secret?: string;
	eventTypes: string[];
	description: string;
	enabled: boolean;
	createdAt: string;
	updatedAt: string;
}

export interface WebhookWithSecret extends Webhook {
	secret: string;
}

export interface WebhookDelivery {
	id: string;
	webhookId: string;
	eventType: string;
	payload: string;
	responseStatus?: number;
	responseBody?: string;
	error?: string;
	attempt: number;
	deliveredAt?: string;
	createdAt: string;
}

export interface CSVImportRow {
	name: string;
	email: string;
	phone: string;
	dietaryNotes: string;
	plusOnes: number;
	error?: string;
	duplicate?: boolean;
}

export interface CSVPreviewResponse {
	rows: CSVImportRow[];
	totalRows: number;
	validRows: number;
	errorRows: number;
	duplicates: number;
}

export interface CSVImportResult {
	imported: number;
	skipped: number;
	failed: number;
	duplicates: number;
	invited: number;
}

export interface EmailStats {
	totalSent: number;
	delivered: number;
	opened: number;
	clicked: number;
	bounced: number;
	complained: number;
	failed: number;
	pending: number;
}

export interface InstanceStats {
	events: {
		total: number;
		draft: number;
		published: number;
		cancelled: number;
		archived: number;
	};
	attendees: {
		total: number;
		totalHeadcount: number;
		attending: number;
		maybe: number;
		declined: number;
		pending: number;
		waitlisted: number;
		avgPerEvent: number;
	};
	organizers: {
		total: number;
	};
	features: {
		waitlistEvents: number;
		commentsEnabledEvents: number;
		cohostedEvents: number;
		eventsWithQuestions: number;
		eventsWithCapacity: number;
		seriesEvents: number;
	};
	notifications: {
		total: number;
		sent: number;
		failed: number;
		delivered: number;
		opened: number;
		bounced: number;
		complained: number;
	};
}

export interface NotificationLogEntry {
	id: string;
	eventId: string;
	attendeeId: string;
	channel: string;
	provider: string;
	status: string;
	deliveryStatus: string;
	error: string;
	recipient: string;
	subject: string;
	messageId: string;
	sentAt?: string;
	deliveredAt?: string;
	openedAt?: string;
	clickedAt?: string;
	bouncedAt?: string;
	bounceType: string;
	complaintAt?: string;
	createdAt: string;
}
