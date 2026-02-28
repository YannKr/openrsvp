<script lang="ts">
	import { page } from '$app/stores';
</script>

<svelte:head>
	<title>{$page.status} — OpenRSVP</title>
</svelte:head>

<div class="min-h-screen bg-white flex flex-col">
	<!-- Decorative background -->
	<div class="absolute inset-0 overflow-hidden pointer-events-none" aria-hidden="true">
		<div class="absolute -top-40 -right-40 w-[500px] h-[500px] rounded-full bg-gradient-to-br from-indigo-100 via-purple-50 to-pink-100 opacity-50 blur-3xl"></div>
		<div class="absolute -bottom-40 -left-40 w-[500px] h-[500px] rounded-full bg-gradient-to-tr from-fuchsia-100 via-indigo-50 to-cyan-100 opacity-40 blur-3xl"></div>
	</div>

	<!-- Navbar -->
	<header class="relative z-10 bg-white/80 backdrop-blur-lg border-b border-slate-100">
		<nav class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between">
			<a href="/" class="flex items-center gap-2">
				<div class="w-8 h-8 rounded-lg bg-indigo-600 flex items-center justify-center">
					<svg class="w-5 h-5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
						<path stroke-linecap="round" stroke-linejoin="round" d="M3 19v-8.93a2 2 0 01.89-1.664l7-4.666a2 2 0 012.22 0l7 4.666A2 2 0 0121 10.07V19M3 19a2 2 0 002 2h14a2 2 0 002-2M3 19l6.75-4.5M21 19l-6.75-4.5M3 10l6.75 4.5M21 10l-6.75 4.5m0 0l-1.14.76a2 2 0 01-2.22 0l-1.14-.76" />
					</svg>
				</div>
				<span class="text-xl font-bold text-slate-900">OpenRSVP</span>
			</a>
		</nav>
	</header>

	<!-- Error content -->
	<main class="relative z-10 flex-1 flex items-center justify-center px-4 py-16">
		<div class="text-center max-w-lg">
			{#if $page.status === 404}
				<!-- Envelope illustration -->
				<div class="relative mx-auto mb-8 w-40 h-40">
					<!-- Shadow -->
					<div class="absolute bottom-2 left-1/2 -translate-x-1/2 w-32 h-4 bg-slate-200/60 rounded-full blur-md"></div>
					<!-- Envelope body -->
					<svg viewBox="0 0 160 140" fill="none" class="w-full h-full drop-shadow-lg">
						<!-- Envelope back -->
						<rect x="12" y="40" width="136" height="90" rx="8" fill="#e0e7ff" stroke="#a5b4fc" stroke-width="2"/>
						<!-- Envelope flap -->
						<path d="M12 48 L80 95 L148 48" fill="#c7d2fe" stroke="#a5b4fc" stroke-width="2" stroke-linejoin="round"/>
						<!-- Letter sticking out -->
						<rect x="30" y="20" width="100" height="60" rx="4" fill="white" stroke="#cbd5e1" stroke-width="1.5"/>
						<!-- Letter lines -->
						<line x1="46" y1="36" x2="114" y2="36" stroke="#e2e8f0" stroke-width="3" stroke-linecap="round"/>
						<line x1="46" y1="46" x2="100" y2="46" stroke="#e2e8f0" stroke-width="3" stroke-linecap="round"/>
						<line x1="46" y1="56" x2="80" y2="56" stroke="#e2e8f0" stroke-width="3" stroke-linecap="round"/>
						<!-- Question mark -->
						<text x="80" y="70" text-anchor="middle" font-size="28" font-weight="bold" fill="#a5b4fc" font-family="system-ui, sans-serif">?</text>
						<!-- Envelope front -->
						<path d="M12 130 L12 48 L80 95 L148 48 L148 130 Z" fill="#eef2ff" stroke="#a5b4fc" stroke-width="2" stroke-linejoin="round"/>
					</svg>
				</div>

				<h1 class="text-6xl font-extrabold tracking-tight text-slate-900 mb-2">404</h1>
				<h2 class="text-xl font-semibold text-slate-700 mb-3">This invite got lost in the mail</h2>
				<p class="text-slate-500 mb-10 leading-relaxed max-w-sm mx-auto">
					The page you're looking for doesn't exist or may have been moved. Let's get you back on track.
				</p>
			{:else}
				<!-- Generic error -->
				<div class="mx-auto mb-8 w-20 h-20 rounded-2xl bg-gradient-to-br from-red-100 to-orange-100 flex items-center justify-center shadow-sm">
					<svg class="w-10 h-10 text-red-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
						<path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z" />
					</svg>
				</div>

				<h1 class="text-6xl font-extrabold tracking-tight text-slate-900 mb-2">{$page.status}</h1>
				<h2 class="text-xl font-semibold text-slate-700 mb-3">Something went wrong</h2>
				<p class="text-slate-500 mb-10 leading-relaxed max-w-sm mx-auto">
					{$page.error?.message || "An unexpected error occurred. Please try again."}
				</p>
			{/if}

			<div class="flex flex-col sm:flex-row items-center justify-center gap-3">
				<a
					href="/"
					class="group inline-flex items-center gap-2 rounded-xl bg-indigo-600 px-6 py-3 text-sm font-semibold text-white hover:bg-indigo-700 transition-all shadow-lg shadow-indigo-600/25 hover:shadow-indigo-600/40"
				>
					<svg class="w-4 h-4 transition-transform group-hover:-translate-x-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
						<path stroke-linecap="round" stroke-linejoin="round" d="M10 19l-7-7m0 0l7-7m-7 7h18" />
					</svg>
					Back to Home
				</a>
				<a
					href="/events"
					class="inline-flex items-center gap-2 rounded-xl bg-white px-6 py-3 text-sm font-semibold text-slate-700 border border-slate-200 hover:border-slate-300 hover:bg-slate-50 transition-all"
				>
					My Events
				</a>
			</div>
		</div>
	</main>

	<!-- Footer -->
	<footer class="relative z-10 py-6 text-center text-sm text-slate-400">
		<a href="/" class="hover:text-slate-600 transition-colors">OpenRSVP</a>
	</footer>
</div>
