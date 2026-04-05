# style

## Frontend
### JSX Elements
- Always use `<div>` and `<span>`, never semantic HTML (`<p>`, `<h1>`, etc.). This prevents style inheritance issues.

### Styling
- Tailwind classes only - no inline styles or style objects
- No negative margins allowed
- Use spacing/gap on parent instead of margins on child throughout.
- Spacing: multiples of 2 (gap-2, gap-4, p-4, m-8)
- Remove redundant responsive breakpoints
- avoid not-null assertions.

```tsx
// Good
<div className="flex gap-4">
  <Child1 />
  <Child2 />
</div>

// Bad
<div className="flex" style={{ gap: '12px' }}>
  <Child1 className="mr-3" />
</div>
```

### Component Patterns
- Extract repeated JSX into components
- Inline conditional placeholders to avoid duplicate wrappers
- Use ternary operators for conditionals, rather than `&&`

### Props & Constants
- Remove unused props
- Use constants for magic values that need explanation
- Add `TODO(cleanup):` for temporary code

## General
### Structure
- Do not ever create your own function if it already exists in the codebase!
- To get the owner of a meeting, use join of usermeetings and Meeting. Prefer this to using creatorId
- Avoid unnecessary comments. Only add them where absolutely necessary.
- All types that are not schemas or derived from schemas should be in `types/src/email.ts`

### TypeScript
- It is generally considered a bad pattern to cast types. Avoid unless necessary.
- Always use named params
- Avoid non null assertions, i.e: `const transcript = meeting.transcript!`. Instead, cast always like so: ` const transcript = meeting.transcript as NonNullable<typeof meeting.transcript>`
- If we need to export the type, they should be in the types package.
- Don't use`let` in the code base. Prefer const always.
- Explicit nullable types: `value?: number | null`
- Array syntax: `items: Item[]` not `Array<Item>`
- Array checks: `arr.length === 0` not `!arr`
- Avoid mutations
- Do not await the prisma db calls
- Use strong types where possible, i.e. Array\<MeetingWithForeignProperties\>
- null is when you asked the db and nothing was returned, whereas undefined is user level, such as when you get the

### Naming
- Use complete variable names, i.e. do `averageWordsPerMinute` instead of `averageWpm`
- Complete sentence errors/comments/logs.
- Booleans: `isOpen`, `hasData`, `shouldShow` (not `showModal`)
- Parameters: descriptive (`specificUserId` not `userId`)
- Types: match UI terminology
- When logging, everything should be a sentence (end with a period)

### Functional Programming
- Use `Array.from()`, `.map()`, `.reduce()` instead of for-loops
- Use utility functions: `getPluralSuffix()`, `.toLocaleString()`

### Lodash
- Use lodash where applicable. For example, the mean utility, and sum

### Common Circleback Utils

```
import {
  getFullName,
  getTimeRangeStartDate,
  isPresent,
} from '@circleback/utils'
```


---

# schema

## Meeting Types
- User meeting
- Meetings
- Workspace meeting
- Meeting is shared with workspace
- Team meeting
- Meeting is shared with team

## Workspaces vs. Teams
- 1 to many users are part of a team or workspace
- 0 to many teams are a part of a workspace
- 0 to 1 workspaces are associated with a user

## Profiles vs. Users
- A profile does not have to be a user (person with a circleback account)
- A user always has an associated profile

## Attendees vs. Users
- An attendee is someone who was either invited to the meeting or actually present at the meeting.
- A user is someone with a circleback account

## Shared Meetings
When editor access is given to a user, team, or workspace

## Cascading Deletes
- Use cascades for deletions instead of manual deletion logic
- More robust than manual handling
- Soft deletes only apply when end of expiry is reached, then hard delete

## Meeting and Calendar Relationship
- Meetings and calendar events are related but independent
- Deleting a meeting doesn't delete the calendar event
- Deleting a calendar event doesn't update the meeting
- Can record the same calendar event infinitely - creates multiple meeting entries

## Transcript Structure
- Every meeting has **zero to one** transcript
- Each transcript has multiple transcript chunks (for embeddings/search)
- Each transcript has multiple segments
- Each segment has multiple words

## Segments & Words
- Segment speaker is stored as a **string** (not profile ID reference)
- String matching is required at multiple places in code
- Each segment stores first word ID and last word ID to determine timestamps
- Word IDs differentiate duplicate words - same word text, different IDs

## Profiles
- Profiles persist even if meetings/attendees are deleted
- Needed for CRM-like functionality - preserving contact history
- Every user has a profile; every attendee has a profile
- Profile IDs are reused across multiple meetings with same person

## Attendees
- Attendee has `participant_name` field **only if they spoke** in the meeting
- If attendee didn't speak, `participant_name` stays
- Attendees are people invited OR who joined

## Meeting Users Vs Attendees
- `user_meeting` table = access level (owner/editor)
- `meeting_attendee` table = participants who were invited or joined
- Meeting owner is in `user_meeting` with access="owner"
- Editor access means meeting was shared with you

## Workspace/Team Sharing
- Workspace sharing doesn't create actual meeting user records
- Uses `workspace_user` table with workspace_id + meeting_id
- UI-level logic makes everyone appear as editor
- DB-level only stores workspace relationship, not individual user records

## ID Generation Rules
- Most IDs: big int, auto-generated by database
- `device_recording` ID: **string (UUID)**, generated client-side to avoid collisions
- Client-generated IDs must be strings with high collision-resistance (UUID v7)

## Data Conventions
## Null Vs Undefined
- **Null** = user explicitly set this value
- **Undefined** = system/automated value, nothing was set
- Database (Prisma/Supabase) always returns **null** for empty fields
- API responses may use **undefined** when data won't be present

## Optional Fields
- Segment language should be required but is optional due to old schema/API changes
- `creator_id` on meetings exists but **robust approach is to join** `user_meeting` table with `meeting` to find owners

## Storage Paths
- Recording: `recording-{user_id}-{device_recording_id}`
- After processing: `meeting_{meeting_id}` format


---

# general

## Build and Development Commands

```bash
# Development
npm run dev                    # Start Next.js web app (uses Turbopack with HTTPS)
npm run dev:mobile             # Start React Native Expo app

# Testing
npm test                       # Run all Jest tests
npm test -- path/to/test.ts    # Run specific test file
npm test -- --testNamePattern="pattern"  # Run tests matching pattern

# Code Quality
npm run lint                   # ESLint across all workspaces (max-warnings=0)
npm run lint:fix               # Auto-fix lint issues
npm run typecheck              # TypeScript type checking all workspaces

# Database
npm run create-migration       # Create Prisma migration
npm run format-schema          # Format Prisma schema

# Build
npm run build                  # Production build (Next.js with Turbopack)
```

## Architecture Overview

**Monorepo Structure (npm workspaces):**

- `apps/web` - Next.js 15 main application with Turbopack
- `apps/mobile` - React Native Expo app
- `packages/` - Shared libraries (@circleback/\*)

**Key Shared Packages:**

- `@circleback/types` - TypeScript types and Zod schemas
- `@circleback/utils` - Pure utility functions (dates, strings, meetings, etc.)
- `@circleback/db-client` - Prisma client with BigInt extension and read replicas
- `@circleback/hooks` - React hooks
- `@circleback/data-loaders` - SWR-based data fetching
- `@circleback/constants` - Shared constants

**Web App Architecture (`apps/web/`):**

- `pages/api/` - Next.js API routes organized by feature
- `helpers/api/` - Backend logic (db queries, LLM operations, external services)
- `helpers/client/` - Frontend utilities
- `components/` - 190+ React components organized by feature
- `prisma/` - Database schema and migrations

**Helper Pattern:** API routes delegate to helper functions in `helpers/api/`. Database operations are in `helpers/api/db/`, LLM operations in `helpers/api/llm/`.

## Technology Stack

- **Frontend:** Next.js 15, React 19, TypeScript 5.6 (strict), Tailwind CSS, Radix UI
- **Database:** PostgreSQL with Prisma 6.17 ORM
- **State:** Jotai for global state, SWR for data fetching
- **AI:** Vercel AI SDK with Anthropic, OpenAI, Google Vertex, Azure OpenAI
- **Testing:** Jest 30 with Node environment
- **Auth:** NextAuth.js

## Code Conventions

**Naming:**

- Functions: action verbs (`calculateTotal`, `validateInput`)
- Variables: nouns (`userProfile`, `totalPrice`)
- Booleans: is/has/can prefixes (`isActive`, `hasPermission`)
- Files: camelCase singular (`actionItem.ts`, `meetingUpload.ts`)

**Style:**

- Functional programming: pure functions, array methods over loops, no mutations
- Self-documenting code preferred over comments
- Comments only when names are insufficient, written as complete sentences with periods

**Database:**

- Follow existing naming patterns in `schema.prisma` (camelCase for fields)
- Migrations must be backwards-compatible (see `apps/web/README.md` for safety rules)
- Never drop or rename columns without careful migration strategy

## Testing

Tests are in `apps/web/tests/` mirroring the source structure:

- `tests/pages/api/` - API route tests
- `tests/helpers/api/` - Helper function tests
- Integration tests use `.integration.test.ts` suffix

Jest global setup loads environment variables from `apps/web/.env`. PostHog is mocked in test setup.

## Environment

- **Node version:** 22.x required
- **Root `.env`** symlinks to `apps/web/.env`
- Local development uses HTTPS with experimental Next.js flag
