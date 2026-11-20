package tui

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/LBYPatrick/ashley/internal/community"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func openCommunityFixture(t *testing.T) *Model {
	t.Helper()
	m := newModel(t)
	m.open("skills.sh")
	m.cursor = slices.IndexFunc(skillsActions, func(action struct{ label, description, command string }) bool {
		return action.command == "bundle"
	})
	key(m, "enter")
	if m.screen != communityScreen {
		t.Fatalf("bundle action opened %q", m.screen)
	}
	return m
}

func TestCommunityChecklistAppliesOnlyAfterConfirm(t *testing.T) {
	m := openCommunityFixture(t)
	text := ansi.Strip(m.View())
	for _, bundle := range community.Bundles() {
		if !strings.Contains(text, bundle.Label) || !strings.Contains(text, bundle.Source) {
			t.Errorf("bundle %q not shown with its source", bundle.ID)
		}
	}
	m.communityFinished(communityResult{installed: map[string]bool{"emil": true}})
	var requests [][]string
	m.options.Background = func(_ context.Context, args []string) (string, error) {
		requests = append(requests, args)
		return "Installed Modern Web Guidance", nil
	}
	if cmd := m.communityKey(tea.KeyMsg{Type: tea.KeyEnter}); cmd != nil || m.community.selected["emil"] {
		t.Fatal("Enter on a bundle should only change selection")
	}
	key(m, "down")
	if cmd := m.communityKey(tea.KeyMsg{Type: tea.KeySpace}); cmd != nil || !m.community.selected["modern-web"] {
		t.Fatal("Space should check a bundle without starting a command")
	}
	text = ansi.Strip(m.View())
	if !strings.Contains(text, "Will remove") || !strings.Contains(text, "Will install") {
		t.Fatal("pending actions not visible")
	}
	m.communityKey(tea.KeyMsg{Type: tea.KeyEnd})
	cmd := m.communityKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil || !m.community.busy {
		t.Fatal("Confirm did not start installation")
	}
	if again := m.communityKey(tea.KeyMsg{Type: tea.KeyEnter}); again != nil {
		t.Fatal("duplicated an in-progress request")
	}
	msg := cmd().(communityResult)
	if !reflect.DeepEqual(requests, [][]string{{"__community-bundles", "modern-web"}}) {
		t.Fatalf("sent incorrect checked selection: %v", requests)
	}
	msg.installed = map[string]bool{"modern-web": true}
	m.Update(msg)
	if m.community.busy || !m.community.installed["modern-web"] || m.community.selected["emil"] {
		t.Fatal("failed to refresh installed state after applying")
	}
	key(m, "l")
	if !strings.Contains(ansi.Strip(m.View()), "Installed Modern Web Guidance") {
		t.Fatal("installation output unavailable")
	}
	key(m, "esc")
	key(m, "esc")
	if m.screen != "skills.sh" || skillsActions[m.cursor].command != "bundle" {
		t.Fatal("lost position in Library")
	}
}

func TestCommunityChecklistKeepsPendingChangesAfterFailure(t *testing.T) {
	m := openCommunityFixture(t)
	m.community.selected["emil"], m.community.selected["find-skills"] = true, true
	m.communityFinished(communityResult{installed: map[string]bool{"emil": true}, applyErr: errors.New("download failed")})
	if m.communityBundleState("emil") != "Installed" || m.communityBundleState("find-skills") != "Will install" {
		t.Fatal("failed to retain requested changes for retry")
	}
	m.communityFinished(communityResult{stateErr: errors.New("invalid marker")})
	if m.communityApply() != nil {
		t.Fatal("applied changes without knowing which bundles are installed")
	}
}

func TestCommunityChecklistFitsSmallTerminal(t *testing.T) {
	m := openCommunityFixture(t)
	m.Update(tea.WindowSizeMsg{Width: 24, Height: 8})
	m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	view := m.View()
	assertFrame(t, view, 24, 8)
	if !strings.Contains(ansi.Strip(view), "Confirm") {
		t.Fatal("could not reach Confirm on compact screen")
	}
}
