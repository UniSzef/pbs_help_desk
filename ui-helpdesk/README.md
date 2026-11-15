# UI Helpdesk - frontend onboarding

This folder contains the pbs_help_desk frontend stack:
- React 19
- Vite 8
- TypeScript 6
- Tailwind CSS 3
- shadcn/ui (code-first workflow)

## 1. Quick start

Requirements:
- Node.js 20+
- npm 10+

Install dependencies and run locally:

```bash
cd ui-helpdesk
npm install
npm run dev
```

The app starts on the Vite dev URL (usually http://localhost:5173).

## 2. Main commands

```bash
npm run dev      # development server
npm run build    # production build (tsc + vite build)
npm run preview  # preview production build locally
npm run lint     # ESLint
```

## 3. Working with shadcn/ui in this project

shadcn is not used as a prebuilt component package.
Components are added as source code to this repository when needed.

Examples:

```bash
cd ui-helpdesk
npx shadcn@latest add button
npx shadcn@latest add input dialog dropdown-menu
```

Component catalog: https://ui.shadcn.com/docs/components

Important:
- Always run shadcn commands from the `ui-helpdesk` folder.
- Use `npx shadcn@latest ...`, not a global `shadcn` command.
- Generated UI components are stored in `src/components/ui`.

## 4. Path alias and imports

The alias `@ -> src` is configured in:
- `tsconfig.app.json` (`paths`)
- `vite.config.ts` (`resolve.alias`)

Example imports:
- `import { Card } from '@/components/ui/card'`
- `import { cn } from '@/lib/utils'`

## 5. Constants from src/index.css?

Yes. Treat CSS variables in `src/index.css` (for example `--background`, `--foreground`, `--primary`, `--border`) as design tokens for the app theme.

Recommended usage:
- Prefer semantic utility classes like `bg-background`, `text-foreground`, `border-border`, `bg-primary`.
- Prefer shadcn component variants (`default`, `secondary`, etc.) before custom one-off colors.
- If a new semantic color is needed, add a new CSS variable token in `src/index.css` and map it in `tailwind.config.cjs`.

Avoid:
- Hardcoding raw colors in many places (for example repeated `bg-[#... ]`) unless it is a deliberate, isolated case.

## 6. Known technical notes

1. During `npm run build`, Lightning CSS warnings may appear for unknown at-rules (`@theme`, `@utility`, `@custom-variant`) from imported shadcn/tw-animate styles. The build still succeeds.
2. Vite shows a warning about `__dirname` in `vite.config.ts` for a future `configLoader: 'native'`. This does not block current development.
