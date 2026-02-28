<script lang="ts">
	interface Props {
		templateId: string;
		heading: string;
		body: string;
		footer: string;
		primaryColor: string;
		secondaryColor: string;
		font: string;
		eventTitle: string;
		eventDate: string;
		eventLocation: string;
	}

	let {
		templateId,
		heading,
		body,
		footer,
		primaryColor,
		secondaryColor,
		font,
		eventTitle,
		eventDate,
		eventLocation
	}: Props = $props();

	const templateConfig = $derived.by(() => {
		switch (templateId) {
			case 'balloon-party':
				return {
					wrapperClass: 'balloon-party',
					decorBefore: '\u{1F388}',
					decorAfter: '\u{1F389}',
					bgGradient: `linear-gradient(135deg, ${primaryColor || '#f97316'}22, ${secondaryColor || '#eab308'}22)`,
					borderColor: primaryColor || '#f97316',
					accentColor: secondaryColor || '#eab308'
				};
			case 'confetti':
				return {
					wrapperClass: 'confetti',
					decorBefore: '\u{1F38A}',
					decorAfter: '\u{1F38A}',
					bgGradient: `linear-gradient(135deg, ${primaryColor || '#ec4899'}11, ${secondaryColor || '#8b5cf6'}11, ${primaryColor || '#ec4899'}11)`,
					borderColor: primaryColor || '#ec4899',
					accentColor: secondaryColor || '#8b5cf6'
				};
			case 'unicorn-magic':
				return {
					wrapperClass: 'unicorn-magic',
					decorBefore: '\u{2728}',
					decorAfter: '\u{1F984}',
					bgGradient: `linear-gradient(135deg, ${primaryColor || '#a855f7'}22, ${secondaryColor || '#ec4899'}22)`,
					borderColor: primaryColor || '#a855f7',
					accentColor: secondaryColor || '#ec4899'
				};
			case 'superhero':
				return {
					wrapperClass: 'superhero',
					decorBefore: '\u{26A1}',
					decorAfter: '\u{1F4A5}',
					bgGradient: `linear-gradient(135deg, ${primaryColor || '#ef4444'}22, ${secondaryColor || '#3b82f6'}22)`,
					borderColor: primaryColor || '#ef4444',
					accentColor: secondaryColor || '#3b82f6'
				};
			case 'garden-picnic':
				return {
					wrapperClass: 'garden-picnic',
					decorBefore: '\u{1F33F}',
					decorAfter: '\u{1F338}',
					bgGradient: `linear-gradient(135deg, ${primaryColor || '#22c55e'}22, ${secondaryColor || '#a3e635'}22)`,
					borderColor: primaryColor || '#22c55e',
					accentColor: secondaryColor || '#a3e635'
				};
			default:
				return {
					wrapperClass: 'default-template',
					decorBefore: '',
					decorAfter: '',
					bgGradient: `linear-gradient(135deg, ${primaryColor || '#6366f1'}22, ${secondaryColor || '#ec4899'}22)`,
					borderColor: primaryColor || '#6366f1',
					accentColor: secondaryColor || '#ec4899'
				};
		}
	});

	function formatDate(dateStr: string): string {
		if (!dateStr) return '';
		try {
			const date = new Date(dateStr);
			return date.toLocaleDateString('en-US', {
				weekday: 'long',
				year: 'numeric',
				month: 'long',
				day: 'numeric',
				hour: 'numeric',
				minute: '2-digit'
			});
		} catch {
			return dateStr;
		}
	}
</script>

<div
	class="invite-card {templateConfig.wrapperClass}"
	style="
		--primary: {primaryColor || '#6366f1'};
		--secondary: {secondaryColor || '#ec4899'};
		--card-bg: {templateConfig.bgGradient};
		--border-color: {templateConfig.borderColor};
		--accent-color: {templateConfig.accentColor};
		--card-font: {font || 'inherit'};
	"
>
	<!-- Template decorations -->
	{#if templateId === 'confetti'}
		<div class="confetti-dots" aria-hidden="true">
			{#each Array(20) as _, i}
				<span
					class="confetti-dot"
					style="
						left: {Math.random() * 100}%;
						top: {Math.random() * 100}%;
						background: {['#ec4899', '#8b5cf6', '#f59e0b', '#10b981', '#3b82f6', '#ef4444'][i % 6]};
						animation-delay: {Math.random() * 2}s;
						width: {4 + Math.random() * 6}px;
						height: {4 + Math.random() * 6}px;
					"
				></span>
			{/each}
		</div>
	{/if}

	{#if templateId === 'unicorn-magic'}
		<div class="sparkle-container" aria-hidden="true">
			{#each Array(8) as _, i}
				<span
					class="sparkle"
					style="
						left: {10 + Math.random() * 80}%;
						top: {10 + Math.random() * 80}%;
						animation-delay: {Math.random() * 3}s;
					"
				></span>
			{/each}
		</div>
	{/if}

	{#if templateId === 'garden-picnic'}
		<div class="garden-decor-top" aria-hidden="true">
			<span class="leaf leaf-1">{'\u{1F33F}'}</span>
			<span class="leaf leaf-2">{'\u{1F340}'}</span>
			<span class="leaf leaf-3">{'\u{1F33F}'}</span>
		</div>
		<div class="garden-decor-bottom" aria-hidden="true">
			<span class="flower flower-1">{'\u{1F338}'}</span>
			<span class="flower flower-2">{'\u{1F33B}'}</span>
			<span class="flower flower-3">{'\u{1F338}'}</span>
		</div>
	{/if}

	<!-- Card header -->
	<div class="card-header">
		{#if templateConfig.decorBefore}
			<span class="decor-emoji decor-left" aria-hidden="true">{templateConfig.decorBefore}</span>
		{/if}
		<h1 class="card-heading" style="font-family: {font || 'inherit'}">
			{heading || eventTitle}
		</h1>
		{#if templateConfig.decorAfter}
			<span class="decor-emoji decor-right" aria-hidden="true">{templateConfig.decorAfter}</span>
		{/if}
	</div>

	<!-- Event details -->
	<div class="card-details">
		{#if eventTitle && heading && heading !== eventTitle}
			<p class="event-title">{eventTitle}</p>
		{/if}
		{#if eventDate}
			<div class="detail-row">
				<svg class="detail-icon" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path stroke-linecap="round" stroke-linejoin="round" d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
				</svg>
				<span>{formatDate(eventDate)}</span>
			</div>
		{/if}
		{#if eventLocation}
			<div class="detail-row">
				<svg class="detail-icon" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path stroke-linecap="round" stroke-linejoin="round" d="M17.657 16.657L13.414 20.9a1.998 1.998 0 01-2.827 0l-4.244-4.243a8 8 0 1111.314 0z" />
					<path stroke-linecap="round" stroke-linejoin="round" d="M15 11a3 3 0 11-6 0 3 3 0 016 0z" />
				</svg>
				<span>{eventLocation}</span>
			</div>
		{/if}
	</div>

	<!-- Card body -->
	{#if body}
		<div class="card-body">
			<p>{body}</p>
		</div>
	{/if}

	<!-- Card footer -->
	{#if footer}
		<div class="card-footer">
			<p>{footer}</p>
		</div>
	{/if}
</div>

<style>
	.invite-card {
		position: relative;
		overflow: hidden;
		background: var(--card-bg);
		border: 2px solid var(--border-color);
		border-radius: 1.5rem;
		padding: 2.5rem 2rem;
		text-align: center;
		font-family: var(--card-font);
		max-width: 32rem;
		width: 100%;
		margin: 0 auto;
	}

	/* Balloon Party */
	.balloon-party {
		border-style: dashed;
		border-width: 3px;
		border-radius: 2rem;
	}
	.balloon-party .card-heading {
		color: var(--primary);
		font-size: 2rem;
		font-weight: 800;
		letter-spacing: -0.025em;
	}
	.balloon-party .decor-emoji {
		font-size: 2rem;
	}

	/* Confetti */
	.confetti {
		border-width: 3px;
		border-style: solid;
		border-image: linear-gradient(135deg, #ec4899, #8b5cf6, #f59e0b, #10b981) 1;
		border-radius: 0;
		overflow: hidden;
	}
	.confetti-dots {
		position: absolute;
		inset: 0;
		pointer-events: none;
	}
	.confetti-dot {
		position: absolute;
		border-radius: 50%;
		opacity: 0.4;
		animation: confetti-float 3s ease-in-out infinite;
	}
	@keyframes confetti-float {
		0%, 100% { transform: translateY(0) rotate(0deg); opacity: 0.4; }
		50% { transform: translateY(-8px) rotate(180deg); opacity: 0.7; }
	}
	.confetti .card-heading {
		color: var(--primary);
		font-size: 2rem;
		font-weight: 800;
		background: linear-gradient(135deg, var(--primary), var(--secondary));
		-webkit-background-clip: text;
		-webkit-text-fill-color: transparent;
		background-clip: text;
	}

	/* Unicorn Magic */
	.unicorn-magic {
		border-color: var(--primary);
		border-width: 2px;
		background: linear-gradient(135deg, #a855f722, #ec489922, #818cf822);
		box-shadow: 0 0 30px #a855f733, 0 0 60px #ec489911;
	}
	.unicorn-magic .card-heading {
		font-size: 2rem;
		font-weight: 800;
		background: linear-gradient(135deg, #a855f7, #ec4899, #818cf8);
		-webkit-background-clip: text;
		-webkit-text-fill-color: transparent;
		background-clip: text;
	}
	.sparkle-container {
		position: absolute;
		inset: 0;
		pointer-events: none;
	}
	.sparkle {
		position: absolute;
		width: 6px;
		height: 6px;
		background: #fbbf24;
		border-radius: 50%;
		animation: sparkle-twinkle 2s ease-in-out infinite;
	}
	@keyframes sparkle-twinkle {
		0%, 100% { opacity: 0.2; transform: scale(0.5); }
		50% { opacity: 1; transform: scale(1.2); }
	}

	/* Superhero */
	.superhero {
		border-width: 4px;
		border-color: var(--primary);
		border-radius: 0.5rem;
		box-shadow: 4px 4px 0 var(--secondary);
		background: linear-gradient(135deg, #ef444411, #3b82f611);
	}
	.superhero .card-heading {
		font-size: 2.25rem;
		font-weight: 900;
		color: var(--primary);
		text-transform: uppercase;
		letter-spacing: 0.05em;
		text-shadow: 2px 2px 0 var(--secondary);
	}
	.superhero .decor-emoji {
		font-size: 1.75rem;
	}

	/* Garden Picnic */
	.garden-picnic {
		border-color: var(--primary);
		border-width: 2px;
		border-radius: 2rem;
		background: linear-gradient(180deg, #22c55e0d, #a3e63511, #22c55e0d);
	}
	.garden-picnic .card-heading {
		font-size: 1.875rem;
		font-weight: 700;
		color: #166534;
	}
	.garden-decor-top, .garden-decor-bottom {
		display: flex;
		justify-content: center;
		gap: 1rem;
		font-size: 1.5rem;
		opacity: 0.6;
	}
	.garden-decor-top {
		margin-bottom: 0.5rem;
	}
	.garden-decor-bottom {
		margin-top: 0.5rem;
	}
	.leaf, .flower {
		display: inline-block;
		animation: sway 3s ease-in-out infinite;
	}
	.leaf-2, .flower-2 { animation-delay: 0.5s; }
	.leaf-3, .flower-3 { animation-delay: 1s; }
	@keyframes sway {
		0%, 100% { transform: rotate(-5deg); }
		50% { transform: rotate(5deg); }
	}

	/* Default template */
	.default-template .card-heading {
		font-size: 2rem;
		font-weight: 700;
		color: var(--primary);
	}

	/* Shared styles */
	.card-header {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 0.75rem;
		margin-bottom: 1.5rem;
		position: relative;
		z-index: 1;
	}

	.card-heading {
		line-height: 1.2;
		margin: 0;
	}

	.decor-emoji {
		font-size: 1.5rem;
		flex-shrink: 0;
	}

	.card-details {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
		margin-bottom: 1.5rem;
		position: relative;
		z-index: 1;
	}

	.event-title {
		font-size: 1.125rem;
		font-weight: 600;
		color: #334155;
		margin: 0;
	}

	.detail-row {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 0.5rem;
		color: #475569;
		font-size: 0.9375rem;
	}

	.detail-icon {
		width: 1.125rem;
		height: 1.125rem;
		flex-shrink: 0;
		color: var(--accent-color);
	}

	.card-body {
		position: relative;
		z-index: 1;
		margin-bottom: 1.5rem;
		padding: 1rem 0;
		border-top: 1px solid color-mix(in srgb, var(--border-color) 20%, transparent);
		border-bottom: 1px solid color-mix(in srgb, var(--border-color) 20%, transparent);
	}

	.card-body p {
		color: #334155;
		font-size: 1rem;
		line-height: 1.6;
		margin: 0;
		white-space: pre-line;
	}

	.card-footer {
		position: relative;
		z-index: 1;
	}

	.card-footer p {
		color: #64748b;
		font-size: 0.875rem;
		font-style: italic;
		margin: 0;
	}
</style>
