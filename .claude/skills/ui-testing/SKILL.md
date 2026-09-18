---
name: ui-testing
description: Use whenever writing, modifying, or reviewing UI tests in React projects. Defines naming, structure, query priority, render conventions, and mocking patterns for component and hook tests. Framework-agnostic on the component library; assumes React + Testing Library + a modern test runner (Vitest or Jest). Inherits the high-level principles from [[testing]].
allowed-tools: Read, Write, Edit, Glob, Grep, Bash
paths: *.ts, *.tsx
---

## Scope

React component tests (`*.test.tsx` / `*.test.jsx`) + React hook tests (`*.test.ts`). Pure-function/domain tests → follow [[testing]]. High-level principles here (naming, structure, data minimality, one-behavior-per-test, behavior-over-implementation, delete-vacuous) pulled from [[testing]], stay in force.

## Global test setup

Most React projects centralize global mocks in setup file (e.g. `vitest.config.ts`'s `setupFiles`, `jest.config.js`'s `setupFilesAfterEach`, `defaultMocks.ts`). **Always read first** when starting on suite — global mocks need no local mock unless asserting on calls (global `vi.fn()` returns fresh fn per render, unassertable; override locally to spy).

## File naming

- Component tests: `<Component>.test.tsx`, co-located.
- Hook tests: `<useHook>.test.ts`, co-located.
- One test file per source file. No sibling files for helpers — test through consumer, or extract helper to own module if worth direct testing.

## Test name pattern

Natural-language sentence, **present simple, third-person**, describing observable behavior. No leading "Should". No snake_case, no camelCase.

```ts
// Good — present simple, plain language, behavior-focused
it('hides the dismiss button when status is healthy', ...);
it('fires the viewed event once on mount', ...);
it('navigates to the diagnostics route when the CTA is clicked', ...);

// Bad — leading "Should"
it('Should hide the dismiss button when status is healthy', ...);

// Bad — snake_case or camelCase
it('hides_dismiss_when_healthy', ...);
it('hidesDismissWhenHealthy', ...);
```

Plain English a product person says. Prefer `does not render …`, `calls …`, `shows …`, `hides …` over jargon. Failure paths: prefer `fails when <condition>` over `throws when <condition>`. No implementation details in name.

## Test structure (Given / When / Then)

Every test → **Given / When / Then**. Three phases → **two** blank lines (Given→When, When→Then). Statements in same phase grouped — no blank lines inside Given even with multiple lines (mock setup + render together). **No `// Given` / `// When` / `// Then` comments.**

```tsx
// Good — render is part of the setup (Given); one blank line between phases
it("calls onDismiss when the Dismiss button is clicked", async () => {
  const onDismiss = vi.fn();
  const user = userEvent.setup();
  renderBanner({ status: "critical" }, { onDismiss });

  await user.click(screen.getByRole("button", { name: /Dismiss/i }));

  expect(onDismiss).toHaveBeenCalledTimes(1);
});
```

**Render belongs to Given.** `render(...)` establishes state test acts on — part of setup, not own phase. Given ends when action under test begins.

**Traceability**: every value When/Then references must be explicit in Given. No relying on factory defaults to seed asserted values. Value matters to assertion → pass explicitly via render/setup helper.

## Test data minimality

- Seed only what assertion needs. Drop props, response fields, fixture rows that don't affect outcome.
- Behavior provable with one trigger / one row → no extras.
- Prefer **semantic shared constants** for recurring values (e.g. `CRITICAL_RESPONSE`, `DEFAULT_USER`) over ad-hoc literals.

## Test data visibility

All test data referenced in assertions must be visible in test body. Module-scope or `describe`-scope fixtures building implicit data violate this — reader should not scroll outer scopes to understand assertion. Pass data explicit through render helper, or use named constants whose meaning obvious from name alone.

## One behavior per test

Each `it(...)` verifies **single observable behavior**. Name needs "and" → split.

- Watch **rendered tree**, not just assertions. Test named "renders the dismiss button" with fixture exercising other variants does work for several rules. Reduce seed to minimum proving named rule.
- Two `expect` proving **different rules** = smell. Two `expect` proving **different facets of same outcome** (e.g. payload shape + call count) = fine.
- **No duplicate test cases.** Two `it(...)` exercising same behavior with same setup (even worded different) = noise. Keep one, delete rest.

## Rendering: centralize provider setup

Use single render helper (`renderWithProviders`, `customRender`, etc.) wiring routing, state, theming, i18n. Components under test must NOT see raw `render()` from `@testing-library/react` — bypasses real providers, drifts from prod.

```tsx
// Good
renderWithProviders(<MyComponent />);

// Bad — duplicated provider setup in every test
render(
  <Provider store={...}>
    <Router>
      <ThemeProvider><MyComponent /></ThemeProvider>
    </Router>
  </Provider>
);
```

Hooks → `renderHook` from `@testing-library/react`.

### Custom state for state-managed apps

Test needs state container in specific shape (e.g. asserting conditional render on user props) → build custom test store, pass to render helper. Wrap dispatches in `act()` so React commits state before assertions.

```ts
import { act } from '@testing-library/react';

const store = createTestStore({ user: userReducer });

it('shows the banner for new users', () => {
  act(() => {
    store.dispatch(userPatch({ signupDate: new Date('2026-01-19') }));
  });
  renderWithProviders(<MyComponent />, store);

  expect(screen.getByText('Banner text')).toBeTruthy();
});
```

(`createTestStore` / `userReducer` = placeholders — sub project's own store helpers.)

## Repeated construction → extract a helper

Same render shape in 3+ tests → extract `renderXxx(...)` helper absorbing incidental boilerplate. Tests then specify only what matters for scenario. Shape gains new field → update helper, never patch every call site.

## `beforeEach` discipline

- **Stateless deps** (mock factories, navigation mocks, analytics spies) used identical across suite: declare at `describe` scope, assign in `beforeEach`.
- **Mock state resets** (`vi.clearAllMocks()` / `jest.clearAllMocks()`, `localStorage.clear()`, `vi.useRealTimers()`): live in `beforeEach`.
- **Never seed test data in `beforeEach`.** Data setup belongs in `it(...)` body so tests stay readable + self-contained.

## Query priority

Per [Testing Library official guidance](https://testing-library.com/docs/queries/about/#priority):

1. `getByRole(role, { name })` — accessible.
2. `getByLabelText` — form fields with labels.
3. `getByPlaceholderText` — fallback.
4. `getByText` — non-interactive content.
5. `getByDisplayValue` — form fields with current value.
6. `getByAltText` / `getByTitle` — last resort for media / hovers.
7. `getByTestId` — escape hatch only. Smell — usually means markup should expose role.

Use `query*` for negative assertions (returns `null` instead of throwing).

## Matchers for text content (the nesting rule)

`getByText` resolution depends on how UI library nests text nodes:

- **Flat content**: `<span>Some message</span>` — one element, one text node. `screen.getByText('Some message')` works.
- **Nested content**: `<span>Hello <strong>world</strong></span>` — parent's `textContent` is `'Hello world'`, but has element children. Testing Library matches parent by default; multiple descendants with same `textContent` → ambiguity.

Nesting introduces ambiguity unsolvable with `getByRole` or `within(container)` → use scoped predicate:

```ts
screen.getByText((_, el) => el?.tagName === "SPAN" && el?.textContent === "Hello world");
```

**Default to plain string form.** Reach for predicate only on real ambiguity. Predicate brittle, harder to read; preventive use = over-defensive.

## Interactions: userEvent over fireEvent

- `userEvent.setup()` once per test, then `await user.click(...)`, `await user.type(...)`.
- `fireEvent` only for synthetic events `userEvent` doesn't model (resize, scroll, custom events).

## Mocking modules

Vitest: `vi.mock` hoisted to top of file. Mock factories referencing local symbols must use `vi.hoisted`:

```ts
const mockFn = vi.hoisted(() => vi.fn());

vi.mock("./module", () => ({ exportedFn: mockFn }));
```

Partial mock (preserve other exports) → `vi.importActual`:

```ts
vi.mock("./module", async () => {
  const actual = await vi.importActual<typeof import("./module")>("./module");
  return { ...actual, exportedFn: mockFn };
});
```

Jest equivalent: top-level `jest.mock(...)` + `jest.requireActual(...)`.

## Mocking `useNavigate` (react-router-dom)

```ts
const mockNavigate = vi.hoisted(() => vi.fn());

vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual<typeof import("react-router-dom")>("react-router-dom");
  return { ...actual, useNavigate: () => mockNavigate };
});
```

## What to assert

**Behavioral, user-observable outcomes only.**

- ✅ Button appears / disappears.
- ✅ Handler called with specific args.
- ✅ Navigation to specific URL happened.
- ✅ Analytics event fired with expected payload.
- ❌ Internal state value.
- ❌ Function reference stability across re-renders (no user-observable consequence — **vacuous test**).
- ❌ Implementation details of storage (`localStorage.getItem(...)`) when behavioral equivalent exists ("banner stays hidden after remount").

### `expect.arrayContaining` is subset matching, not equality

Passes when array contains _at least_ listed items — extras slip through. For "exactly these in any order", pair with `toHaveLength(N)` or sort both sides + `toEqual([...])`.

## Test behavior, not library boundaries

- UI library implements your product behavior → behavior still yours to test. Library = implementation detail, not excuse to skip testing.
- Behavior hard to test → **restructure code for testability first**. Most "untestable" behaviors = design signal — extract presentational component, pull state up, separate effects.
- **Delete vacuous tests.** Test that passes regardless of correctness = worse than no test. Remove implementation → test still passes → test proves nothing.

## Forbidden patterns

- Control flow in test bodies (`if`, `for`, `while`, `switch`, `try`). Tests stay declarative + linear.
- Vacuous tests with no failure mode.
- Asserting implementation details when behavioral equivalent exists.
- Multiple unrelated behaviors per `it(...)`.
- Re-implementing system under test in test setup (compute expected with same algorithm).
- Manual provider setup when centralized helper exists.
- Module-scope mutable state bleeding between tests.

## Cleanup

- `vi.clearAllMocks()` / `jest.clearAllMocks()` in `beforeEach` for any test asserting on mocks.
- `localStorage.clear()` / `sessionStorage.clear()` for storage tests.
- `vi.useRealTimers()` / `jest.useRealTimers()` in `afterEach` for tests faking time.

## Strategy and efficiency

Prefer fast, deterministic tests. Layer coverage:

- **Presentational components**: cover rendering rules (one test per visible branch).
- **Container components**: cover integration with hooks, navigation, analytics.
- **Hooks**: assert on public surface (return values, observable callbacks). No reaching into closure internals.
- Prefer highest level exercising behavior. Add lower-level tests only for branches combinatorially expensive at higher level.