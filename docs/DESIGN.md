---
version: alpha
name: Pixel Terminal
description: >-
  Retro 8-bit computing aesthetic (VIC-20 / Commodore 64 / early home micros)
  reimagined with modern proportions, generous whitespace, and a restrained
  palette. CRT phosphor glow meets flat design — pixel fonts for display,
  monospace for body, warm amber/blue/green phosphor accents on deep navy.
  Scanlines and box-drawing borders as decoration, not noise.
colors:
  # ── Surface (deep navy CRT bezel) ──
  bg-base: "#0a0e1a"
  bg-surface: "#121829"
  bg-elevated: "#1a2238"
  bg-inset: "#060912"
  # ── Text ──
  text-primary: "#e8eaf0"
  text-secondary: "#9aa3b8"
  text-muted: "#5c6577"
  # ── Phosphor accents ──
  amber: "#ffb627"
  amber-dim: "#cc8c1a"
  green: "#5fe0a0"
  green-dim: "#3da06e"
  blue: "#5b9eff"
  blue-dim: "#3d72b8"
  magenta: "#ff5c8a"
  cyan: "#4dd2ff"
  # ── Semantic ──
  border: "#2a3450"
  border-bright: "#3d4f73"
  selection: "#1e3a5f"
  link: "#5b9eff"
  link-hover: "#4dd2ff"
  # ── Aliases (used by components) ──
  primary: "#ffb627"
  secondary: "#5fe0a0"
  tertiary: "#5b9eff"
  neutral: "#121829"
typography:
  display-xl:
    fontFamily: "'Press Start 2P', 'VT323', monospace"
    fontSize: "3.5rem"
    fontWeight: 400
    lineHeight: "1.15"
    letterSpacing: "0.02em"
  display-lg:
    fontFamily: "'Press Start 2P', 'VT323', monospace"
    fontSize: "2.5rem"
    fontWeight: 400
    lineHeight: "1.2"
    letterSpacing: "0.02em"
  display-md:
    fontFamily: "'Press Start 2P', 'VT323', monospace"
    fontSize: "1.75rem"
    fontWeight: 400
    lineHeight: "1.3"
    letterSpacing: "0.02em"
  display-sm:
    fontFamily: "'Press Start 2P', 'VT323', monospace"
    fontSize: "1.25rem"
    fontWeight: 400
    lineHeight: "1.4"
    letterSpacing: "0.03em"
  heading-lg:
    fontFamily: "'Press Start 2P', monospace"
    fontSize: "1.125rem"
    fontWeight: 400
    lineHeight: "1.5"
    letterSpacing: "0.05em"
  heading-md:
    fontFamily: "'Press Start 2P', monospace"
    fontSize: "0.875rem"
    fontWeight: 400
    lineHeight: "1.5"
    letterSpacing: "0em"
  body-lg:
    fontFamily: "'JetBrains Mono', 'Fira Code', 'Cascadia Code', monospace"
    fontSize: "1.125rem"
    fontWeight: 400
    lineHeight: "1.7"
    letterSpacing: "0em"
  body-md:
    fontFamily: "'JetBrains Mono', 'Fira Code', 'Cascadia Code', monospace"
    fontSize: "1rem"
    fontWeight: 400
    lineHeight: "1.7"
    letterSpacing: "0em"
  body-sm:
    fontFamily: "'JetBrains Mono', 'Fira Code', 'Cascadia Code', monospace"
    fontSize: "0.875rem"
    fontWeight: 400
    lineHeight: "1.6"
    letterSpacing: "0em"
  code:
    fontFamily: "'JetBrains Mono', 'Fira Code', 'Cascadia Code', monospace"
    fontSize: "0.875rem"
    fontWeight: 400
    lineHeight: "1.5"
    letterSpacing: "0em"
  caption:
    fontFamily: "'JetBrains Mono', monospace"
    fontSize: "0.75rem"
    fontWeight: 400
    lineHeight: "1.4"
    letterSpacing: "0.05em"
  label:
    fontFamily: "'Press Start 2P', monospace"
    fontSize: "0.625rem"
    fontWeight: 400
    lineHeight: "1.4"
    letterSpacing: "0.1em"
rounded:
  none: 0px
  xs: 2px
  sm: 4px
spacing:
  xs: 4px
  sm: 8px
  md: 16px
  lg: 24px
  xl: 32px
  2xl: 48px
  3xl: 64px
  4xl: 96px
shadows:
  glow-amber: "0 0 12px rgba(255, 182, 39, 0.3), 0 0 24px rgba(255, 182, 39, 0.1)"
  glow-green: "0 0 12px rgba(95, 224, 160, 0.3), 0 0 24px rgba(95, 224, 160, 0.1)"
  glow-blue: "0 0 12px rgba(91, 158, 255, 0.3), 0 0 24px rgba(91, 158, 255, 0.1)"
  inset-screen: "inset 0 0 60px rgba(0, 0, 0, 0.6)"
components:
  page:
    backgroundColor: "{colors.bg-base}"
    textColor: "{colors.text-primary}"
    rounded: "{rounded.none}"
    padding: "0px"
  terminal-window:
    backgroundColor: "{colors.bg-surface}"
    textColor: "{colors.text-primary}"
    rounded: "{rounded.none}"
    padding: "0px"
  terminal-header:
    backgroundColor: "{colors.bg-elevated}"
    textColor: "{colors.text-secondary}"
    rounded: "{rounded.none}"
    padding: "8px 16px"
  code-block:
    backgroundColor: "{colors.bg-inset}"
    textColor: "{colors.text-primary}"
    rounded: "{rounded.xs}"
    padding: "16px"
  button-primary:
    backgroundColor: "{colors.amber}"
    textColor: "#0a0e1a"
    rounded: "{rounded.none}"
    padding: "12px 24px"
  button-primary-hover:
    backgroundColor: "{colors.amber-dim}"
    textColor: "#0a0e1a"
    rounded: "{rounded.none}"
    padding: "12px 24px"
  button-secondary:
    backgroundColor: "transparent"
    textColor: "{colors.green}"
    rounded: "{rounded.none}"
    padding: "12px 24px"
  button-secondary-hover:
    backgroundColor: "{colors.bg-elevated}"
    textColor: "{colors.green}"
    rounded: "{rounded.none}"
    padding: "12px 24px"
  link-inline:
    textColor: "{colors.link}"
    typography: "{typography.body-md}"
  link-inline-hover:
    textColor: "{colors.link-hover}"
    typography: "{typography.body-md}"
  text-selection:
    backgroundColor: "{colors.selection}"
    textColor: "{colors.text-primary}"
    rounded: "{rounded.none}"
    padding: "0px"
  badge:
    backgroundColor: "{colors.bg-elevated}"
    textColor: "{colors.amber}"
    rounded: "{rounded.none}"
    padding: "4px 8px"
  badge-green:
    backgroundColor: "{colors.bg-elevated}"
    textColor: "{colors.green}"
    rounded: "{rounded.none}"
    padding: "4px 8px"
  badge-blue:
    backgroundColor: "{colors.bg-elevated}"
    textColor: "{colors.blue}"
    rounded: "{rounded.none}"
    padding: "4px 8px"
  badge-magenta:
    backgroundColor: "{colors.bg-elevated}"
    textColor: "{colors.magenta}"
    rounded: "{rounded.none}"
    padding: "4px 8px"
  nav-item:
    textColor: "{colors.text-secondary}"
    typography: "{typography.heading-md}"
    padding: "8px 16px"
  nav-item-active:
    textColor: "{colors.amber}"
    typography: "{typography.heading-md}"
    padding: "8px 16px"
  card:
    backgroundColor: "{colors.neutral}"
    textColor: "{colors.text-primary}"
    rounded: "{rounded.none}"
    padding: "24px"
  card-header:
    backgroundColor: "{colors.bg-elevated}"
    textColor: "{colors.amber}"
    rounded: "{rounded.none}"
    padding: "12px 16px"
  prompt:
    backgroundColor: "transparent"
    textColor: "{colors.green}"
    typography: "{typography.body-md}"
    padding: "0px"
  divider:
    backgroundColor: "{colors.border}"
    textColor: "{colors.text-secondary}"
    rounded: "{rounded.none}"
    padding: "0px"
---

## Overview

**Pixel Terminal** is an 8-bit retro design system for the `ws` CLI project
website. It channels the phosphor glow of VIC-20, Commodore 64, and early
home microcomputers — but with modern proportions, generous whitespace, and
clean flat layout. No skeuomorphic CRT bezels or scanline overlays on body
text; the retro character lives in the typography (pixel display fonts),
box-drawing borders, phosphor accent colors, and terminal-style code blocks.

The vibe: a boot screen for a friendly, powerful tool. Warm amber is the
primary accent (the "READY." prompt), green for success/active states, blue
for links and navigation, magenta for warnings. Deep navy backgrounds evoke
the inside of a CRT without the glass.

## Colors

### Surfaces

- **bg-base (#0a0e1a):** Page background — the deepest navy, like the void
  behind a CRT screen.
- **bg-surface (#121829):** Cards, terminal windows, and content containers.
  Slightly raised from the base.
- **bg-elevated (#1a2238):** Headers, toolbars, badge backgrounds. The
  brightest surface before accent colors.
- **bg-inset (#060912):** Code blocks and terminal interiors — darker than
  the base to create depth.

### Text

- **text-primary (#e8eaf0):** Off-white with a slight cool tint. Never pure
  white (#fff) — that's too harsh against the navy.
- **text-secondary (#9aa3b8):** Muted blue-gray for descriptions, nav items,
  secondary content.
- **text-muted (#5c6577):** Captions, placeholders, the faintest text tier.

### Phosphor Accents

- **amber (#ffb627):** Primary accent — the "READY." prompt, CTA buttons,
  active states. Warm, inviting, unmistakably retro.
- **green (#5fe0a0):** Success, active workspace indicators, the shell
  prompt character (`$`). Relaxed, go-ahead.
- **blue (#5b9eff):** Links, navigation hover, interactive elements. Cool,
  clickable.
- **magenta (#ff5c8a):** Warnings, errors, destructive actions. Hot pink
  phosphor.
- **cyan (#4dd2ff):** Link hover, special highlights. Brighter than blue,
  used sparingly.

Each accent has a dim variant (`*-dim`) for hover/active button states.

### Borders

- **border (#2a3450):** Standard hairline borders on cards, dividers, table
  rows.
- **border-bright (#3d4f73):** Focused/active borders, highlighted box edges.

## Typography

Two font families carry the whole system:

- **Press Start 2P** — pixel font for all display headings, nav items, labels,
  and badges. Used sparingly for body text (only `display-*` sizes). Never
  below 0.625rem — pixel fonts become illegible at small sizes.
- **JetBrains Mono** (or Fira Code / Cascadia Code fallback) — monospace for
  all body text, code, descriptions. Modern, readable, carries the terminal
  aesthetic without being kitschy.

**Key rules:**

- Display headings (display-xl through display-sm) use Press Start 2P. These
  are for hero sections, page titles, and feature highlights only.
- All body text is monospace — this is a developer tool, and monospace
  reinforces the terminal DNA.
- `label` typography (0.625rem Press Start 2P) is for tiny all-caps labels
  like "READY", "ACTIVE", "LAYER".
- Letter spacing on pixel fonts is slightly positive (0.02em–0.1em) to
  improve legibility.
- Line height on body text is generous (1.7) — modern proportions, not
  cramped 8-bit text.

## Layout & Spacing

Spacing is on a 4px grid:

| Token | Value | Usage |
|-------|-------|-------|
| xs | 4px | Tight gaps within components |
| sm | 8px | Small element gaps, badge padding |
| md | 16px | Standard padding, paragraph spacing |
| lg | 24px | Card padding, section sub-gaps |
| xl | 32px | Section internal spacing |
| 2xl | 48px | Between major sections |
| 3xl | 64px | Hero section padding |
| 4xl | 96px | Page-level vertical rhythm |

**Layout principles:**

- Max content width: 720px (centered). Narrower than typical — monospace
  text reads better in constrained columns.
- Generous vertical whitespace between sections (2xl–3xl).
- Cards and terminal windows span full width within the content container.
- No sidebar — single-column flow for simplicity.

## Elevation & Depth

Flat design with one exception: **phosphor glow shadows**. These simulate
CRT phosphor emission on active/focused elements:

- **glow-amber:** Applied to primary CTA buttons and active amber elements.
- **glow-green:** Applied to active status indicators and the shell prompt.
- **glow-blue:** Applied to focused links and interactive elements.
- **inset-screen:** Applied to terminal/code interiors for CRT depth.

No drop shadows on cards — borders define edges. This keeps the aesthetic
clean and flat while the glow adds the retro warmth.

## Shapes

- **rounded: none (0px):** Default for ALL components. 8-bit pixels have no
  curves. Terminal windows, buttons, cards, badges — everything is
  rectangular.
- **rounded: xs (2px):** Exception for code blocks only — a barely-perceptible
  soften to distinguish them from the surrounding card surface.

No pill shapes, no large border-radius. Sharp corners are a core aesthetic
constraint.

## Components

### Terminal Window

The signature component — a code/terminal block styled as a retro computer
screen:

- Header bar with bg-elevated background, text-secondary color, shows a
  window title (e.g., `ws — workspace graph CLI`)
- Body with bg-inset background, text-primary color, monospace font
- Box-drawing border (1px solid border color) around the whole window
- Optional inset-screen shadow for CRT depth
- Used for: code examples, CLI output, the hero section prompt

### Buttons

- **Primary:** Amber background, dark text, no radius. Phosphor glow on
  hover. One per view — the main CTA.
- **Secondary:** Transparent background, green text, 1px green border. Fill
  on hover (bg-elevated). Multiple per view OK.

### Badges

Small rectangular labels with bg-elevated background and accent-colored text.
No border-radius. Used for: status indicators (ACTIVE/IDLE), feature tags,
layer hashes. Color variants: amber (default), green (success), blue (info),
magenta (warning).

### Navigation

Horizontal nav bar, no underline on links. Active item uses amber text;
inactive uses text-secondary. Press Start 2P font at heading-md size (0.875rem).
Spacing: 16px between items.

### Cards

Rectangular containers with bg-surface, 1px border, no radius. Optional
header bar (card-header component) with bg-elevated + amber text. Used for:
feature explanations, command references, agent roster rows.

### Prompt

Inline text component showing a shell prompt: green `$` followed by a
command in text-primary. No background or border — purely typographic.
Used inline in descriptions and code examples.

### Divider

Full-width hairline in border color. Optionally rendered as a box-drawing
line (`─────`) using border-bright for a retro terminal feel.

## Do's and Don'ts

### Do

- Use monospace for all body text — it's a developer tool
- Keep Press Start 2P to display sizes only (≥0.625rem)
- Apply phosphor glow shadows to draw attention to interactive elements
- Use box-drawing characters (│, ─, ┌, ┐, └, ┘) for decorative borders
- Maintain generous whitespace — modern proportions, not cramped 8-bit
- Use amber sparingly — it's the primary accent, not a background color

### Don't

- Don't use border-radius above 2px — 8-bit means sharp corners
- Don't overlay scanline textures on body text — keep it clean
- Don't use pure white (#ffffff) — it's too harsh against the navy
- Don't use Press Start 2P for long body text — it's unreadable at length
- Don't use drop shadows — phosphor glow is the only depth cue
- Don't use more than two accent colors on a single component
