package core

import peer "github.com/libp2p/go-libp2p-peer"

// HeartbeatProtocol definition
type HeartbeatProtocol interface {
	Heartbeat(peer.ID) bool
}
