package social

import (
	"github.com/freehandle/breeze/crypto"
	"github.com/freehandle/breeze/util"
)

type ProtocolBlock struct {
	Epoch     uint64
	Actions   [][]byte
	Seal      crypto.Hash
	Publisher crypto.Token
	Siganture crypto.Signature
}

type ProtocolBuilder struct {
	data []byte
}

func NewProtocolBuilder(epoch uint64) *ProtocolBuilder {
	data := make([]byte, 0)
	util.PutUint64(epoch, &data)
	return &ProtocolBuilder{
		data: data,
	}
}

func (p *ProtocolBuilder) AddAction(action []byte) {
	util.PutByteArray(action, &p.data)
}

func (p *ProtocolBuilder) Finalize(publisher crypto.PrivateKey) {
	zero := make([]byte, 0)
	util.PutByteArray(zero, &p.data)
	hash := crypto.Hasher(p.data)
	util.PutHash(hash, &p.data)
	util.PutToken(publisher.PublicKey(), &p.data)
	signature := publisher.Sign(hash[:])
	util.PutSignature(signature, &p.data)
}

func (p *ProtocolBuilder) Bytes() []byte {
	return p.data
}

func ParseProtocolBlock(data []byte) *ProtocolBlock {
	position := 0
	protocol := ProtocolBlock{Actions: make([][]byte, 0)}
	protocol.Epoch, position = util.ParseUint64(data, position)
	for {
		data, position = util.ParseByteArray(data, position)
		if len(data) == 0 {
			break
		}
		protocol.Actions = append(protocol.Actions, data)
	}
	hash := crypto.Hasher(data[:position])
	protocol.Seal, position = util.ParseHash(data, position)
	if !protocol.Seal.Equal(hash) {
		return nil
	}
	protocol.Publisher, position = util.ParseToken(data, position)
	protocol.Siganture, _ = util.ParseSignature(data, position)
	if protocol.Publisher.Verify(hash[:], protocol.Siganture) {
		return nil
	}
	return &protocol
}
