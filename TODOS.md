# TODOS

## Deferred Items

### Accessibility Audit (Post-Design System)
- **What:** Run a full WCAG 2.1 AA audit across all pages.
- **Why:** Design system color changes affect contrast ratios. DESIGN.md specifies AA-compliant pairs, but edge cases (muted text on subtle backgrounds, dark mode combinations) need automated verification.
- **Context:** Design preview showed good contrast (16.8:1 primary text, 4.5:1 primary on white). Run after design system ships and Playwright is set up. Consider integrating axe-core into Playwright tests.
- **Depends on:** Design system rollout complete + Playwright visual regression setup.
- **Added:** 2026-03-18 via /plan-eng-review
