package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/m/civitui/pkg/civit"
)

// TestNavOrderColumnMajor ensures tab/↑↓ walk left column top→bottom, then
// right column top→bottom (not left-to-right across each pair row).
func TestNavOrderColumnMajor(t *testing.T) {
	order := navOrder()
	if len(order) != numFormFields {
		t.Fatalf("navOrder len=%d, want numFormFields=%d", len(order), numFormFields)
	}
	seen := make(map[int]bool, len(order))
	for _, fi := range order {
		if fi < 0 || fi >= numFormFields {
			t.Fatalf("out-of-range field index %d", fi)
		}
		if seen[fi] {
			t.Fatalf("duplicate field %d in navOrder", fi)
		}
		seen[fi] = true
	}
	idx := map[int]int{}
	for i, fi := range order {
		idx[fi] = i
	}

	// Left column finishes before any right-half pair mate.
	// e.g. Width (left) before Height (right); Draft (left tail) before AspectRatio (right head).
	if idx[fiWidth] >= idx[fiHeight] {
		t.Fatalf("Width should be before Height (column-major): W=%d H=%d", idx[fiWidth], idx[fiHeight])
	}
	if idx[fiQuantity] >= idx[fiSeed] {
		t.Fatalf("Quantity should be before Seed: Q=%d Seed=%d", idx[fiQuantity], idx[fiSeed])
	}
	if idx[fiDraft] >= idx[fiAspectRatio] {
		t.Fatalf("Draft (end of left col) should be before AspectRatio (start of right col): Draft=%d AR=%d order=%v",
			idx[fiDraft], idx[fiAspectRatio], order)
	}
	// Right column: AspectRatio then Height then CFG then Seed…
	if !(idx[fiAspectRatio] < idx[fiHeight] && idx[fiHeight] < idx[fiCFGScale] && idx[fiCFGScale] < idx[fiSeed]) {
		t.Fatalf("right col order broken: AR=%d H=%d CFG=%d Seed=%d",
			idx[fiAspectRatio], idx[fiHeight], idx[fiCFGScale], idx[fiSeed])
	}

	m := NewModel(civit.NewClient("test"), false)
	// Down the left col: Quantity → Output Format (not Seed).
	m.activeInput = fiQuantity
	m.advanceFocus(+1)
	if m.activeInput != fiOutputFormat {
		t.Fatalf("after Quantity, next=%d want OutputFormat=%d", m.activeInput, fiOutputFormat)
	}
	// End of left col → first of right col.
	m.activeInput = fiDraft
	m.advanceFocus(+1)
	if m.activeInput != fiAspectRatio {
		t.Fatalf("after Draft, next=%d want AspectRatio=%d (start of right col)", m.activeInput, fiAspectRatio)
	}
	// End of right col wraps to top.
	m.activeInput = fiFluxUltraRaw
	m.advanceFocus(+1)
	if m.activeInput != fiPrompt {
		t.Fatalf("after last right-col field, next=%d want Prompt=%d", m.activeInput, fiPrompt)
	}
}

// TestFormColumnAlignment guards the fixed label/value columns in viewConfig.
// Solo field labels share one column; paired right-half labels share another.
func TestFormColumnAlignment(t *testing.T) {
	m := NewModel(civit.NewClient("test"), false)
	mod, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = mod.(Model)
	plain := stripANSI(m.View())

	var formLines []string
	for _, line := range strings.Split(plain, "\n") {
		if strings.Contains(line, "Model:") || strings.Contains(line, "Flux Mode:") ||
			strings.Contains(line, "Scheduler:") || strings.Contains(line, "Width:") ||
			strings.Contains(line, "Steps:") || strings.Contains(line, "Quantity:") ||
			strings.Contains(line, "Output Format:") || strings.Contains(line, "Upscale Width:") ||
			strings.Contains(line, "Experimental:") || strings.Contains(line, "Draft Mode:") {
			formLines = append(formLines, line)
		}
	}
	if len(formLines) < 6 {
		t.Fatalf("expected form lines, got %d\n%s", len(formLines), plain)
	}

	soloCols := map[string]int{}
	for _, line := range formLines {
		for _, lab := range []string{"Model:", "Flux Mode:", "Output Format:", "Draft Mode:"} {
			if c := strings.Index(line, lab); c >= 0 {
				soloCols[lab] = c
			}
		}
	}
	var soloRef int
	for lab, c := range soloCols {
		if soloRef == 0 {
			soloRef = c
			continue
		}
		if c != soloRef {
			t.Errorf("solo label %s at col %d, want %d", lab, c, soloRef)
		}
	}

	pairRight := map[string]int{}
	for _, line := range formLines {
		if strings.Contains(line, "Upscale Height:") {
			pairRight["Upscale Height:"] = strings.Index(line, "Upscale Height:")
			continue
		}
		for _, lab := range []string{"Aspect Ratio:", "Height:", "CFG Scale:", "Seed:", "Flux Ultra Raw:"} {
			if c := strings.Index(line, lab); c >= 0 {
				pairRight[lab] = c
			}
		}
	}
	var pairRef int
	for lab, c := range pairRight {
		if pairRef == 0 {
			pairRef = c
			continue
		}
		if c != pairRef {
			t.Errorf("pair-right label %s at col %d, want %d (all=%v)", lab, c, pairRef, pairRight)
		}
	}
	if len(pairRight) < 3 {
		t.Fatalf("expected several paired right labels, got %v\n%s", pairRight, plain)
	}
}
