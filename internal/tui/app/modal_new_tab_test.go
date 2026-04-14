package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestRenderNewTabModal_ShowsTitle(t *testing.T) {
	t.Parallel()
	m := newColdModel(t)
	m.activeModal = ModalNewTab
	m.tabPicker = newTabPicker(t.TempDir(), nil)

	out := ansi.Strip(m.renderNewTabModal(30))
	if !strings.Contains(out, "NEW TAB") {
		t.Errorf("new tab modal should render the title, got %q", out)
	}
}

func TestRenderNewTabModal_ClampsHeight(t *testing.T) {
	t.Parallel()
	m := newColdModel(t)
	m.activeModal = ModalNewTab
	m.tabPicker = newTabPicker(t.TempDir(), nil)

	// contentHeight smaller than the modal's default minimum (15): the
	// code clamps up to 15, then clamps back down so it fits.
	out := ansi.Strip(m.renderNewTabModal(4))
	if out == "" {
		t.Error("should still render when contentHeight is tiny")
	}
}

func TestRenderNewTabModal_NarrowWidthFloor(t *testing.T) {
	t.Parallel()
	m := newColdModel(t)
	m.width = 40 // would compute overlayW=24, below the 50 floor
	m.activeModal = ModalNewTab
	m.tabPicker = newTabPicker(t.TempDir(), nil)

	out := ansi.Strip(m.renderNewTabModal(30))
	if !strings.Contains(out, "NEW TAB") {
		t.Errorf("modal should render even at narrow width, got %q", out)
	}
}

func TestUpdateNewTabModal_ForwardsToPicker(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mkPlainDir(t, root, "a")
	mkPlainDir(t, root, "b")

	m := newColdModel(t)
	m.activeModal = ModalNewTab
	m.tabPicker = newTabPicker(root, nil)

	// Pressing Down should advance the picker cursor; the update runs
	// through updateNewTabModal, so that function's branch is covered.
	result, _ := m.updateNewTabModal(keyMsg("down").(tea.KeyPressMsg))
	if result.tabPicker.cursor != 1 {
		t.Errorf("cursor should advance through updateNewTabModal, got %d",
			result.tabPicker.cursor)
	}
}

func TestUpdateActiveModal_NewTabBranch(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mkPlainDir(t, root, "x")

	m := newColdModel(t)
	m.activeModal = ModalNewTab
	m.tabPicker = newTabPicker(root, nil)

	// Drives the ModalNewTab branch of updateActiveModal.
	_, _ = m.updateActiveModal(keyMsg("down").(tea.KeyPressMsg))
}
