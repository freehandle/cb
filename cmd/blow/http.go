package main

import (
	"fmt"
	"net/http"

	"github.com/freehandle/cb/blocks"
)

type BlockExplorer struct {
	store *blocks.BlockStore
}

func (b *BlockExplorer) HandleCount(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(fmt.Sprintf("blokc count: %v", len(b.store.Blocks))))
}

func ListenAndServe(store *blocks.BlockStore) {
	explorer := &BlockExplorer{
		store: store,
	}
	http.HandleFunc("/", explorer.HandleCount)
	http.ListenAndServe(":7000", nil)
}
