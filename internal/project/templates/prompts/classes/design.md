# Design

When the project you're working in has established design systems, component libraries, or interaction patterns that differ from what's described here, follow the project.

## User Goals

**Identify the user's goal before choosing a component.** Every screen, dialog, and interaction exists to help someone accomplish something. Name the goal explicitly. "The user wants to filter results by date range" leads to different decisions than "we need a date picker." When the goal is unclear, the interface will be unclear regardless of how polished the components are.

**Design the interaction sequence, not just the screen.** A form is not a collection of fields; it is a conversation with an order. Walk through the sequence a user follows: what they see first, what they do next, what feedback they receive, where they go after completion. Map the transitions between states, not just the states themselves. An interface that looks right in a screenshot can still feel wrong in motion.

## Edge States

**Design for empty, error, loading, and overflow before the happy path.** The happy path is the least interesting state because it requires the fewest decisions. An empty list needs a message that tells the user why it is empty and what they can do about it. An error needs enough detail to be actionable without exposing internal system information. A loading state needs to communicate that progress is happening and, when possible, how much remains. Overflow (text truncation, excessive list items, viewport exhaustion) needs a strategy: truncate with disclosure, paginate, or scroll with visible indication of remaining content.

**Handle partial states.** Real data is incomplete. A user profile without an avatar, a product without a price, a notification without a timestamp. Design for graceful degradation when fields are missing rather than treating full data as the only valid state.

## Accessibility

**Build accessibility into the structure, not onto the surface.** Semantic HTML elements, logical heading hierarchy, sufficient color contrast (minimum 4.5:1 for normal text per WCAG 2.2 AA), visible focus indicators, and keyboard navigability are structural properties of the interface, not decorations applied at the end. Retrofitting accessibility after visual design is complete is expensive and produces inferior results.

**Support multiple interaction modes.** Not every user operates a mouse. Ensure every interactive element is reachable and operable via keyboard. Provide text alternatives for images and non-text content. Use ARIA attributes only when native HTML semantics are insufficient; incorrect ARIA is worse than no ARIA.

## Responsive Design

**Design for content fluidity, not fixed breakpoints.** Breakpoints are implementation artifacts; the real question is where content stops working at a given width. Let content determine where the layout needs to adapt rather than snapping to device categories. A three-column layout that collapses to one column at 600px because the columns become unreadable is responsive. A layout that collapses at 768px because "that's the tablet breakpoint" is guessing.

**Touch targets need physical size.** Interactive elements on touch devices need a minimum target size of 44x44 CSS pixels (per Apple HIG) or 48x48 density-independent pixels (per Material Design 3). Small hit targets cause mis-taps, and mis-taps cause frustration and errors. Spacing between adjacent targets matters as much as the targets themselves.

## Design System Consistency

**Use existing tokens and components before creating new ones.** Design systems exist to reduce decisions, not to be admired. When a component in the system solves the problem, use it, even if a custom solution would be slightly more elegant. Every custom component is a maintenance burden and a source of visual inconsistency. When the system genuinely lacks what you need, extend it through the system's conventions rather than building outside it.

**Respect the system's spacing, typography, and color scales.** Ad-hoc values (a margin of 13px in an 8-point grid, a gray that doesn't match any token) accumulate into visual noise. The eye detects inconsistency even when the mind cannot name it. Use the scale.
