# Layout Contract

> How lazypx computes pane dimensions for any terminal size.

## Principle

All layout math lives in one pure function: `tui.ComputeLayout(termW, termH) Layout`. Frame sizes are derived from Lip Gloss styles via `GetFrameSize()`, never hardcoded. All values are clamped ≥ 0. Two invariants must always hold:

```
HeaderH + MainOuterH + TasksOuterH + KeyBarH == TermH
TreeOuterW + DetailOuterW == TermW
```

## Pane Map

```
┌─────────────────────── TermW ────────────────────────┐
│  Header (1 line, no border)                          │
├──────────────┬───────────────────────────────────────┤
│  Tree        │  Detail                               │
│  (bordered)  │  (bordered)                           │
│              │                                       │
│  Inner:      │  Inner:                               │
│  Outer-Frame │  Outer-Frame                          │
├──────────────┴───────────────────────────────────────┤
│  Tasks (bordered, full width)                        │
├──────────────────────────────────────────────────────┤
│  KeyBar (1 line, no border)                          │
└──────────────────────────────────────────────────────┘
```

## Sizing Rules

| Pane | Width | Height |
|------|-------|--------|
| Header | TermW | 1 (fixed) |
| Tree | `max(minInner+frame, W×28%)` capped at `W×40%` | `TermH - Header - Tasks - KeyBar` |
| Detail | `TermW - TreeOuter` | Same as Tree |
| Tasks | TermW | `max(minInner+frame, H×20%)` capped at 14, max half of available |
| KeyBar | TermW | 1 (fixed) |
| Overlays | `min(maxFixed, TermW-6)` floored at 20 | Content-driven |

## Minimums

- Tree inner width: 30 chars
- Detail inner width: 36 chars
- Tasks inner height: 3 rows
- Gauge width: 14 chars (min), 32 chars (max)

## Frame Derivation

```go
frameW, frameH := StylePaneBorder.GetFrameSize()
// For lipgloss.RoundedBorder(): frameW=2, frameH=2
innerW = outerW - frameW
innerH = outerH - frameH
```

Sub-models receive **inner** dimensions only. Each `View()` wraps its content in `StylePaneBorder.Width(innerW).Height(innerH).Render(content)`.

## Gauge Sizing

```go
func GaugeWidth(innerW int) int {
    remaining := innerW - 18  // 14-char label + 4 padding
    gw := remaining * 40/100  // 40% of remaining
    return clamp(gw, 14, 32)
}
```

Adapts to available width: narrow terminals get 14-char gauges, wide terminals get 32-char gauges.

## Resize Flow

```
tea.WindowSizeMsg → ComputeLayout(w, h) → store Layout in Model
  → tree.SetSize(innerW, innerH)
  → detail.SetSize(innerW, innerH)
  → tasks.SetSize(innerW, innerH)
  → tree.clampCursor()
```

## Tests

9 unit tests in `tui/layout_test.go`:
- Standard (120×40), Small (80×24), Wide (200×50), Tiny (40×12)
- 10K-iteration fuzz: random sizes 1-500 × 1-200, no negatives
- Vertical invariant: header+main+tasks+keybar == termH
- Horizontal invariant: tree+detail == termW
- Gauge monotonicity: width never decreases as terminal widens
- Overlay width: bounded and floor-clamped
