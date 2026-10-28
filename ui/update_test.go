package ui

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestStaleDownloadResultsAreIgnored(t *testing.T) {
	m := NewMainModel(nil, "test", nil)
	m.state = DownloadProgressState
	oldCtx, oldJob := m.newDownloadJob()
	_, currentJob := m.newDownloadJob()
	if oldCtx.Err() == nil {
		t.Fatal("starting a download left the previous download running")
	}

	for _, msg := range []any{downloadCompleteMsg{job: oldJob}, downloadErrorMsg{job: oldJob}, delayedDownloadCompleteMsg{job: oldJob}} {
		if _, cmd := m.Update(msg); cmd != nil || m.cancelDownload == nil || m.state != DownloadProgressState {
			t.Fatalf("%T from an abandoned download changed the current download", msg)
		}
	}
	if _, cmd := m.Update(downloadCompleteMsg{job: currentJob}); cmd == nil {
		t.Fatal("current download completion was ignored")
	}
}

func TestCancelledDownloadErrorIsIgnored(t *testing.T) {
	m := NewMainModel(nil, "test", nil)
	m.state = DownloadProgressState
	ctx, job := m.newDownloadJob()

	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if ctx.Err() == nil || m.state != FollowedModelsState {
		t.Fatalf("Back left the download running or stayed on state %v", m.state)
	}
	m.Update(downloadErrorMsg{Error: context.Canceled, job: job})
	if m.state != FollowedModelsState || m.message != "" {
		t.Fatalf("cancelled download error moved to state %v with message %q", m.state, m.message)
	}
}
