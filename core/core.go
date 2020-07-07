package core

import (
	"os"

	"github.com/gogo/protobuf/proto"
	crypto "github.com/libp2p/go-libp2p-crypto"
	host "github.com/libp2p/go-libp2p-host"
	inet "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	protocol "github.com/libp2p/go-libp2p-protocol"
)

// Node type - p2p Host base. Implements various protocols.
type Node interface {
	P2PHost() host.Host
	Repository() Repo
	Sender() MsgSender
	Receiver() MsgReceiver
}

// MsgSender interface to send proto messages
type MsgSender interface {
	SendReqProtoMsg(peer.ID, protocol.ID, proto.Message) error
	SendRespProtoMsg(string, peer.ID, protocol.ID, proto.Message) error
}

// MsgReceiver interface to handle Stream to convert to protobuf message
type MsgReceiver interface {
	ReceiveProtoMsg(inet.Stream, proto.Message) error
}

// Repo defines the interface used by other subsystems
type Repo interface {
	PrivKey() crypto.PrivKey
	PubKey() crypto.PubKey
	Conf() Config
	KVRepo() Store
	FsRepo() Store
}

type Config interface {
	Get(string, interface{}) bool
}

const (
	// SortNatural use natural order
	SortNatural Sort = iota
	// SortCreatedDesc created newest to oldest
	SortCreatedDesc
	// SortCreatedAsc created oldest to newset
	SortCreatedAsc
	// SortUpdatedDesc updated newest to oldest
	SortUpdatedDesc
	// SortUpdatedAsc updated oldest to newset
	SortUpdatedAsc
)

type (
	// Sort type
	Sort int

	// Items defines a list of Item class
	Items []Item

	// Item is the base type for all data structures used
	// in the Store
	Item interface {
		GetTable() string
		GetId() string
	}

	// FileItemSetter is used by file type store to set File
	FileItemSetter interface {
		SetFp(*os.File)
	}

	// TimeTracker interface provides utitlity to set and update
	// timestamps
	TimeTracker interface {
		SetCreated(t int64)
		GetCreated() int64
		SetUpdated(t int64)
		GetUpdated() int64
	}

	// IdSetter gives utility to assign new ID as required
	IdSetter interface {
		SetId(string)
	}

	// ListOpt is the options available while getting list output
	ListOpt struct {
		Page  int64
		Limit int64
		Sort  Sort
	}

	// Store provides a generic DB interface
	Store interface {
		Create(Item) error
		Update(Item) error
		Delete(Item) error
		Read(Item) error
		List(Items, ListOpt) (int, error)
		ListAll(allocFn func(int) Items) (Items, error)
	}
)
