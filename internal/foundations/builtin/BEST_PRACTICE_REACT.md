# React Best Practices

## Component Design

- Functional components with hooks only
- Keep components small, focused, and composable
- Separate presentational components from feature logic

## State Management

- Use the smallest state solution that fits
- TanStack Query for server state
- Use context sparingly — good for theme, auth, locale

## Data Fetching

- Keep API calls out of presentational components
- Validate external data at the boundary (Zod schemas)
- Handle loading, empty, error, and stale states

## TypeScript

- Strict mode enabled
- Derive types from Zod schemas with z.infer<>
- Use discriminated unions for state modeling

## Accessibility

- Semantic HTML and native controls
- Keyboard accessible with visible focus states
- ARIA only when native HTML is insufficient
