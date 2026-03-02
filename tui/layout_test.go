package tui

import (
	"math/rand"
	"testing"

	"lazypx/state"
)

func TestComputeLayout_Standard(t *testing.T) {
	l := ComputeLayout(120, 40, state.PanelNodes)
	assertLayoutValid(t, l)

	if l.SidebarInnerW < minSidebarInnerW {
		t.Errorf("sidebar inner width %d < min %d", l.SidebarInnerW, minSidebarInnerW)
	}
	if l.DetailInnerW < minDetailInner {
		t.Errorf("detail inner width %d < min %d", l.DetailInnerW, minDetailInner)
	}
}

func TestComputeLayout_Small(t *testing.T) {
	l := ComputeLayout(80, 24, state.PanelNodes)
	assertLayoutValid(t, l)
}

func TestComputeLayout_Wide(t *testing.T) {
	l := ComputeLayout(200, 50, state.PanelVMs)
	assertLayoutValid(t, l)

	// Sidebar shouldn't take more than maxSidebarPct
	maxSidebar := 200 * maxSidebarPct / 100
	if l.SidebarOuterW > maxSidebar+2 { // +2 for rounding
		t.Errorf("sidebar outer %d exceeds max %d%% of 200", l.SidebarOuterW, maxSidebarPct)
	}
}

func TestComputeLayout_Tiny(t *testing.T) {
	l := ComputeLayout(40, 12, state.PanelCTs)
	assertLayoutValid(t, l)
}

func TestComputeLayout_NoNegatives_Fuzz(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 10000; i++ {
		w := rng.Intn(500) + 1
		h := rng.Intn(200) + 1

		// pick random focus
		focus := state.PanelType(rng.Intn(6))

		l := ComputeLayout(w, h, focus)
		assertNoNegatives(t, l, w, h)
	}
}

func TestComputeLayout_VerticalInvariant(t *testing.T) {
	sizes := [][2]int{{80, 24}, {120, 40}, {200, 50}, {40, 12}, {300, 80}}
	for _, sz := range sizes {
		l := ComputeLayout(sz[0], sz[1], state.PanelNodes)

		// The 4 lists must sum up exactly to SidebarOuterH
		sumLists := l.NodesOuterH + l.VMsOuterH + l.CTsOuterH + l.StorageOuterH
		if sumLists != l.SidebarOuterH {
			t.Errorf("accordion heights %d + %d + %d + %d = %d, want SidebarOuterH %d",
				l.NodesOuterH, l.VMsOuterH, l.CTsOuterH, l.StorageOuterH, sumLists, l.SidebarOuterH)
		}

		total := l.HeaderH + l.SidebarOuterH + l.TasksOuterH + l.KeyBarH
		if total != sz[1] {
			t.Errorf("vertical invariant broken at %dx%d: header(%d)+main(%d)+tasks(%d)+keybar(%d)=%d, want %d",
				sz[0], sz[1], l.HeaderH, l.SidebarOuterH, l.TasksOuterH, l.KeyBarH, total, sz[1])
		}
	}
}

func TestComputeLayout_HorizontalInvariant(t *testing.T) {
	sizes := [][2]int{{80, 24}, {120, 40}, {200, 50}}
	for _, sz := range sizes {
		l := ComputeLayout(sz[0], sz[1], state.PanelNodes)
		total := l.SidebarOuterW + l.DetailOuterW
		if total != sz[0] {
			t.Errorf("horizontal invariant broken at %dx%d: sidebar(%d)+detail(%d)=%d, want %d",
				sz[0], sz[1], l.SidebarOuterW, l.DetailOuterW, total, sz[0])
		}
	}
}

func TestGaugeWidth_Scales(t *testing.T) {
	gw40 := GaugeWidth(40)
	if gw40 < 14 {
		t.Errorf("gauge width at innerW=40 is %d, want >= 14", gw40)
	}

	gw100 := GaugeWidth(100)
	if gw100 > 32 {
		t.Errorf("gauge width at innerW=100 is %d, want <= 32", gw100)
	}

	prev := GaugeWidth(20)
	for w := 21; w <= 200; w++ {
		cur := GaugeWidth(w)
		if cur < prev {
			t.Errorf("gauge width decreased from %d to %d at innerW=%d", prev, cur, w)
		}
		prev = cur
	}
}

func TestOverlayWidth(t *testing.T) {
	w := OverlayWidth(50, 120)
	if w != 50 {
		t.Errorf("overlay width = %d, want 50", w)
	}

	w = OverlayWidth(50, 40)
	if w > 34 {
		t.Errorf("overlay width = %d on 40-wide terminal, want <= 34", w)
	}

	w = OverlayWidth(50, 10)
	if w < 20 {
		t.Errorf("overlay width = %d, want >= 20", w)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func assertLayoutValid(t *testing.T, l Layout) {
	t.Helper()
	assertNoNegatives(t, l, l.TermW, l.TermH)

	vTotal := l.HeaderH + l.SidebarOuterH + l.TasksOuterH + l.KeyBarH
	if vTotal != l.TermH {
		t.Errorf("vertical: %d + %d + %d + %d = %d, want %d",
			l.HeaderH, l.SidebarOuterH, l.TasksOuterH, l.KeyBarH, vTotal, l.TermH)
	}

	hTotal := l.SidebarOuterW + l.DetailOuterW
	if hTotal != l.TermW {
		t.Errorf("horizontal: %d + %d = %d, want %d",
			l.SidebarOuterW, l.DetailOuterW, hTotal, l.TermW)
	}
}

func assertNoNegatives(t *testing.T, l Layout, w, h int) {
	t.Helper()
	fields := map[string]int{
		"SidebarOuterW": l.SidebarOuterW,
		"SidebarOuterH": l.SidebarOuterH,
		"SidebarInnerW": l.SidebarInnerW,
		"SidebarInnerH": l.SidebarInnerH,
		"DetailOuterW":  l.DetailOuterW,
		"DetailOuterH":  l.DetailOuterH,
		"DetailInnerW":  l.DetailInnerW,
		"DetailInnerH":  l.DetailInnerH,
		"TasksOuterW":   l.TasksOuterW,
		"TasksOuterH":   l.TasksOuterH,
		"TasksInnerW":   l.TasksInnerW,
		"TasksInnerH":   l.TasksInnerH,
		"NodesOuterH":   l.NodesOuterH,
		"VMsOuterH":     l.VMsOuterH,
		"CTsOuterH":     l.CTsOuterH,
		"StorageOuterH": l.StorageOuterH,
	}
	for name, v := range fields {
		if v < 0 {
			t.Errorf("%s = %d (negative) at %dx%d", name, v, w, h)
		}
	}
}
