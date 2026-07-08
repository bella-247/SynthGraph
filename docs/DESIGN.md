# SynthGraph Design System

## Color System

### Background Hierarchy

| Token | Hex | Usage |
|-------|-----|-------|
| `--bg` | `#0a0e14` | Page background |
| `--bg2` | `#111820` | Card / elevated surface |
| `--bg3` | `#18222e` | Input / table header / hover |
| `--bg4` | `#1f2c3c` | Active tab / active state |

### Border Hierarchy

| Token | Hex | Usage |
|-------|-----|-------|
| `--border` | `#263040` | Default border |
| `--border-light` | `#38485a` | Hover border, emphasized |

### Text Hierarchy

| Token | Hex | Contrast on `--bg` | Usage |
|-------|-----|-------------------|-------|
| `--text` | `#e2e8f0` | 15.6:1 | Primary body text |
| `--text2` | `#94a3b8` | 7.8:1 | Secondary / metadata |
| `--text3` | `#7f8ea3` | 4.8:1 | Placeholder / disabled / muted |

### Semantic Colors

| Token | Hex | Usage |
|-------|-----|-------|
| `--accent` | `#3b82f6` | Primary actions, links, focus |
| `--accent-hover` | `#60a5fa` | Button hover state |
| `--green` | `#22c55e` | Success / valid |
| `--red` | `#ef4444` | Error / destructive |
| `--yellow` | `#eab308` | Warning / partial |
| `--orange` | `#d4761a` | Junction tables (graph) |
| `--purple` | `#a78bfa` | Unique constraints / semantic tags |

### Semantic Backgrounds

Each semantic color has a matching 10%-opacity background: `--green-bg`, `--red-bg`, `--yellow-bg`, `--orange-bg`, `--purple-bg`.

### Gradients (decorative only)

| Token | Value | Usage |
|-------|-------|-------|
| `--gradient-primary` | `135deg, #3b82f6, #8b5cf6` | Logo icon, progress bar |
| `--gradient-accent` | `135deg, #3b82f6, #06b6d4` | Active nav underline |

---

## Typography

### Font Stack

```css
--font-ui: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
--font-mono: "Cascadia Code", "Fira Code", "JetBrains Mono", monospace;
```

### Type Scale

| Level | Size | Weight | Usage |
|-------|------|--------|-------|
| h2 | 16px | 600 | Card headers |
| h3 | 15px | 600 | Modal titles, panel headers |
| h4 | 12px | 500 | Section headers (uppercase) |
| body | 14px | 400 | Default text |
| body-sm | 13px | 400 | Card descriptions, table cells |
| caption | 12px | 400 | Data preview, code samples |
| label | 12px | 500 | Form labels (uppercase) |
| tiny | 11px | 400 | Badges, version, timestamps |

### Line Heights

- Body: `1.6`
- Mono: `1.5`
- Tight: `1.4`

---

## Spacing Scale (4px Grid)

| Token | px | Usage |
|-------|----|-------|
| 4px | 4 | Badge margin, icon gap |
| 8px | 8 | Form gap, flex gap |
| 12px | 12 | Card padding inner gap |
| 14px | 14 | Table cell padding (horizontal) |
| 16px | 16 | Card padding (mobile), form group margin |
| 20px | 20 | Header padding |
| 24px | 24 | Card padding (desktop), main padding |
| 28px | 28 | Main padding top |
| 40px | 40 | Loading block padding |

---

## Component Specs

### Button

| Property | Default | Primary | Danger | Ghost | Small |
|----------|---------|---------|--------|-------|-------|
| Font size | 13px | 13px | 13px | 13px | 12px |
| Padding | 7px 16px | 7px 16px | 7px 16px | 7px 16px | 4px 10px |
| Border | 1px solid `--border` | transparent | transparent | transparent | 1px solid `--border` |
| Background | `--bg3` | `--accent` | `--red` | transparent | `--bg3` |
| Color | `--text` | `#fff` | `#fff` | `--text2` | `--text` |
| Border radius | `--radius-sm` | `--radius-sm` | `--radius-sm` | `--radius-sm` | `--radius-sm` |
| Hover bg | `--bg4` | `--accent-hover` | brightness 1.15 | `--bg3` | `--bg4` |
| Hover transform | translateY(-1px) | — | — | — | translateY(-1px) |
| Disabled | opacity 0.4 | `--bg3` / `--text2` | — | — | opacity 0.4 |

### Card

| Property | Value |
|----------|-------|
| Background | `--bg2` |
| Border | 1px solid `--border` |
| Border radius | `--radius` (10px) |
| Padding | 24px (16px mobile) |
| Box shadow | `--shadow` |
| Top highlight | 1px `--border-light` gradient |

### Table

| Property | Value |
|----------|-------|
| Font size | 13px (header: 11px) |
| Header bg | `--bg3` |
| Header transform | uppercase, 0.5px tracking |
| Cell padding | 10px 14px |
| Cell border | 1px solid `--border` |
| Row hover | rgba(255,255,255,0.02) |
| Wrapper | `overflow-x: auto` |
| Wrapper border | 1px solid `--border` |

### Modal

| Property | Value |
|----------|-------|
| Overlay | rgba(0,0,0,0.6) + blur(4px) |
| Background | `--bg2` |
| Border | 1px solid `--border` |
| Border radius | `--radius` (10px) |
| Padding | 24px |
| Width | 90%, max 440px |
| Box shadow | `--shadow-lg` |
| Enter anim | scale(0.95) + translateY(8px) → identity, 0.2s |

### Panel (slide-in)

| Property | Value |
|----------|-------|
| Width | 440px (100% mobile) |
| Position | fixed, right: -460px → 0 |
| Background | `--bg2` |
| Border-left | 1px solid `--border` |
| Box shadow | `--shadow-lg` |
| Transition | right 0.3s cubic-bezier |
| Padding | 24px 20px |

### Toast

| Property | Value |
|----------|-------|
| Position | fixed, bottom: 20px, right: 20px |
| Max width | 400px |
| Background | `--bg3` |
| Border | 1px solid `--border` (colored for error/success/info) |
| Border radius | `--radius-sm` (6px) |
| Box shadow | `--shadow-lg` |
| Enter anim | translateY(12px) + scale(0.96) → identity, 0.3s |
| Auto-dismiss | 4s |

---

## Animation Tokens

| Token | Duration | Timing | Usage |
|-------|----------|--------|-------|
| fast | 0.15s | ease | Button hover, nav hover, icon hover |
| normal | 0.2s | ease | Focus, modal enter |
| slow | 0.3s | ease / cubic-bezier | Page enter, panel slide, toast |
| progress | 0.4s | ease | Progress bar fill |

### Keyframes

| Name | Purpose |
|------|---------|
| `pageIn` | fade + translateY(8px) → identity |
| `modalIn` | scale(0.95) + translateY(8px) → identity |
| `toastIn` | translateY(12px) + scale(0.96) → identity |
| `genDoneIn` | translateY(-8px) → identity |
| `spin` | 360deg rotation (loading spinner) |
| `shimmer` | translateX(-100%) → 100% (progress bar) |

---

## Elevation

| Token | Value | Usage |
|-------|-------|-------|
| `--shadow` | `0 1px 3px rgba(0,0,0,0.4), 0 1px 2px rgba(0,0,0,0.3)` | Card default |
| `--shadow-lg` | `0 8px 40px rgba(0,0,0,0.5)` | Modal, panel, toast |
| `--shadow-glow` | `0 0 20px rgba(59,130,246,0.08)` | Accent glow (unused) |

---

## Iconography

- Logo: graph-node motif (3 circles connected by edges)
- Badge indicator: 10px uppercase, semibold
- Semantic icons: checkmark (success), cross (error), warning (yellow)
- Interactive: chevron (expand/collapse), plus (add), close (dismiss)
- Loading: circular spinner (16px, 2px border, top-accent)
