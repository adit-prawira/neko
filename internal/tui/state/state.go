package state

import (
	"time"

	"github.com/adit-prawira/neko/internal/ffi"
)

type Panel int
type TickingMessage struct{}

const (
	CollectionPanel Panel = iota
	SearchPanel
	ResultsPanel
)

type Collection struct {
	Name  string
	Stats ffi.NekoStats
}

type Model struct {
	Collections   []Collection
	SelectedIndex int
	FocusedPanel  Panel
	SearchInput   string
	SearchResults []ffi.NekoSearchResult
	StatusMessage string
	LastLatency   time.Duration
	LastQuery     []float32
}

func (m Model) IsSearchPanelFocused() bool {
	return m.FocusedPanel == SearchPanel
}

func (m Model) IsCollectionPanelFocused() bool {
	return m.FocusedPanel == CollectionPanel
}

func (m Model) IsResultsPanelFocused() bool {
	return m.FocusedPanel == ResultsPanel
}
