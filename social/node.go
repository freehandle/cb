package social

import (
	"fmt"
	"log"
	"net"

	"github.com/freehandle/breeze/consensus/chain"
	"github.com/freehandle/breeze/crypto"
	"github.com/freehandle/breeze/socket"
	"github.com/freehandle/breeze/util"
)

type ProtocolValidatorNodeConfig struct {
	BlockProviderAddr  string
	BlockProviderToken crypto.Token
	Port               int
	NodeCredentials    crypto.PrivateKey
	ValidateOutgoing   socket.ValidateConnection
	KeepNBlocks        int
}

func LaunchNode[M Merger[M], B Blocker[M]](config ProtocolValidatorNodeConfig, blockchain *SocialBlockChain[M, B]) chan error {
	finalize := make(chan error, 2)

	outgoing, err := net.Listen("tcp", fmt.Sprintf(":%v", config.Port))
	if err != nil {
		finalize <- fmt.Errorf("could not listen on port %v: %v", config.Port, err)
		return finalize
	}

	blockSyncRequest := make(chan BlockSyncRequest)
	forward := make(chan []byte)
	newBlock := make(chan struct{})
	pool := make(socket.ConnectionPool)

	go func() {
		messages := BreezeBlockListener(config, blockchain.epoch)
		for {
			signal := <-messages
			switch signal.Signal {
			case ErrSignal:
				finalize <- signal.Err
				return
			case NewBlockSignal:
				newBlock <- struct{}{}
				blockchain.Lock()
				fmt.Println("------------------------> new", signal.Epoch)
				if err := blockchain.NextBlock(signal.Epoch); err == nil {
					forward <- NewBlockSocial(blockchain.epoch)
				} else {
					log.Printf("LaunchNode> %v", err)
				}
				blockchain.Unlock()
			case ActionSignal:
				if blockchain.Validate(signal.Action) {
					forward <- ActionSocial(signal.Action)
				}
			case ActionArraySignal:
				for n := 0; n < signal.Actions.Len(); n++ {
					if blockchain.Validate(signal.Action) {
						forward <- ActionSocial(signal.Action)
					}
				}
			case SealSignal:
				blockchain.Lock()
				fmt.Println("------------------------> seal", signal.Epoch)
				if hash, err := blockchain.SealBlock(signal.Epoch, signal.Hash); err == nil {
					forward <- SealBlockSocial(signal.Epoch, hash)
				} else {
					log.Printf("LaunchNode> %v", err)
				}
				blockchain.Unlock()
			case CommitSignal:
				blockchain.Lock()
				fmt.Println("------------------------> commit", signal.Epoch)
				if invalidated, err := blockchain.Commit(signal.Epoch, signal.HashArray); err == nil {
					forward <- CommitBlockSocial(signal.Epoch, invalidated)
				} else {
					log.Printf("LaunchNode> %v", err)
				}
				blockchain.Unlock()
			}
		}
	}()

	// listen outgoing (cached with recent blocks)
	go func() {
		for {
			if conn, err := outgoing.Accept(); err == nil {
				trustedConn, err := socket.PromoteConnection(conn, config.NodeCredentials, config.ValidateOutgoing)
				if err != nil {
					conn.Close()
				}
				go WaitForOutgoingSyncRequest(trustedConn, blockSyncRequest)
			}

		}
	}()

	go func() {
		for {
			select {
			case <-newBlock:
				pool.DropDead() // clear dead connections
			case msg := <-forward:
				pool.Broadcast(msg)
			case req := <-blockSyncRequest:
				cached := socket.NewCachedConnection(req.conn)
				pool.Add(cached)
				blockchain.Lock()
				blockchain.Sync(cached, req.epoch)
				blockchain.Unlock()
			}
		}

	}()

	return finalize

}

type BlockSyncRequest struct {
	conn  *socket.SignedConnection
	epoch uint64
}

func WaitForOutgoingSyncRequest(conn *socket.SignedConnection, syncRequest chan BlockSyncRequest) {
	data, err := conn.Read()
	if err != nil || len(data) != 9 || data[0] != chain.MsgSyncRequest {
		conn.Shutdown()
		return
	}
	epoch, _ := util.ParseUint64(data, 1)
	syncRequest <- BlockSyncRequest{conn: conn, epoch: epoch}
}
