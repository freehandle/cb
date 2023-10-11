package social

import (
	"github.com/freehandle/breeze/consensus/chain"
	"github.com/freehandle/breeze/crypto"
	"github.com/freehandle/breeze/util"
)

func NewBlockSocial(epoch uint64) []byte {
	bytes := []byte{chain.MsgNewBlock}
	util.PutUint64(epoch, &bytes)
	return bytes
}

func ActionSocial(action []byte) []byte {
	bytes := []byte{chain.MsgAction}
	bytes = append(bytes, action...)
	return bytes
}

func SealBlockSocial(epoch uint64, hash crypto.Hash) []byte {
	bytes := []byte{chain.MsgBlockSealed}
	util.PutUint64(epoch, &bytes)
	util.PutHash(hash, &bytes)
	return bytes
}

func CommitBlockSocial(epoch uint64, invalidated []crypto.Hash) []byte {
	bytes := []byte{chain.MsgBlockCommitted}
	util.PutUint64(epoch, &bytes)
	util.PutHashArray(invalidated, &bytes)
	return nil
}
