package node

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/aloknerurkar/gopcp/core/heartbeat"

	"github.com/aloknerurkar/gopcp/core"
	hbpb "github.com/aloknerurkar/gopcp/core/heartbeat/pb"
	"github.com/aloknerurkar/gopcp/core/objects"
	corepb "github.com/aloknerurkar/gopcp/core/pb"
	ggio "github.com/gogo/protobuf/io"
	"github.com/gogo/protobuf/proto"
	"github.com/google/uuid"
	logger "github.com/ipfs/go-log"
	libp2p "github.com/libp2p/go-libp2p"
	crypto "github.com/libp2p/go-libp2p-crypto"
	host "github.com/libp2p/go-libp2p-host"
	inet "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	protocol "github.com/libp2p/go-libp2p-protocol"
	"github.com/libp2p/go-libp2p/p2p/discovery"
	ma "github.com/multiformats/go-multiaddr"
)

var log = logger.Logger("Core::Node")

const clientVersion = "go-pcp/0.0.1"

type task interface {
	Execute()
}

// Node type - p2p Host base. Implements various protocols.
type nodeImpl struct {
	host.Host
	core.HeartbeatProtocol
	core.Repo
	taskQueue chan chan task
}

func initMDNS(ctx context.Context, n *nodeImpl, rendezvous string) error {
	ser, err := discovery.NewMdnsService(ctx, n.Host, time.Minute*5, rendezvous)
	if err != nil {
		log.Errorf("Failed registering MDNS service Err:%s", err.Error())
		return err
	}
	ser.RegisterNotifee((*discoveryNotifiee)(n))
	return nil
}

// NewNodeImpl : Create a new nodeImpl with its implemented protocols
func NewNodeImpl(parent context.Context, rep core.Repo) core.Node {

	var listenAddr string
	var p2pPort int
	if !rep.Conf().Get("listenAddr", &listenAddr) ||
		!rep.Conf().Get("p2pPort", &p2pPort) {
		log.Error("P2P config not provided.")
		return nil
	}

	listen, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%d", listenAddr, p2pPort))
	if err != nil {
		log.Errorf("Failed creating P2P listener. Make sure port %d is open.", p2pPort)
		return nil
	}

	cctx, cancel := context.WithCancel(parent)
	go func() {
		<-parent.Done()
		log.Infof("Parent done. Closing.")
		cancel()
	}()

	host, err := libp2p.New(cctx,
		libp2p.ListenAddrs(listen),
		libp2p.Identity(rep.PrivKey()))
	if err != nil {
		log.Errorf("Failed creating P2P node Err:%s", err.Error())
		return nil
	}

	nodeP := &nodeImpl{
		Repo:      rep,
		Host:      host,
		taskQueue: make(chan chan task, 20),
	}

	// Listen to peer connected/disconnected events
	host.Network().Notify((*netNotifiee)(nodeP))

	err = initMDNS(cctx, nodeP, "gopcp_node")
	if err != nil {
		log.Errorf("Failed initializing mDNS service Err:%s", err.Error())
		return nil
	}

	nodeP.startWorkers(cctx, 2)

	nodeP.HeartbeatProtocol = heartbeat.NewHeartbeatProtocol(cctx, nodeP)

	log.Infof("Started P2P node. Listening on port %d", p2pPort)
	return nodeP
}

func (n *nodeImpl) P2PHost() host.Host {
	return n.Host
}

func (n *nodeImpl) Sender() core.MsgSender {
	return n
}

func (n *nodeImpl) Receiver() core.MsgReceiver {
	return n
}

func (n *nodeImpl) Repository() core.Repo {
	return n.Repo
}

// AuthenticatedMsg : If message implements this interface, basic msg
// signing will be used. This is shamelessly copied from go-libp2p-examples/multipro
type AuthenticatedMsg interface {
	GetMessageData() *corepb.MessageData
	SetMessageData(*corepb.MessageData)
}

func (n *nodeImpl) authenticateMessage(message proto.Message, data *corepb.MessageData) bool {
	// store a temp ref to signature and remove it from message data
	// sign is a string to allow easy reset to zero-value (empty string)
	sign := data.Sign
	data.Sign = nil
	// marshall data without the signature to protobufs3 binary format
	bin, err := proto.Marshal(message)
	if err != nil {
		log.Errorf("Failed to marshal pb message Err:%s", err.Error())
		return false
	}
	// restore sig in message data (for possible future use)
	data.Sign = sign
	// restore peer id binary format from base58 encoded nodeImpl id data
	peerID, err := peer.IDB58Decode(data.NodeId)
	if err != nil {
		log.Errorf("Failed to decode nodeImpl id from base58 Err:%s", err.Error())
		return false
	}
	// verify the data was authored by the signing peer identified by the public key
	// and signature included in the message
	return n.verifyData(bin, []byte(sign), peerID, data.NodePubKey)
}

// Verify incoming p2p message data integrity
// data: data to verify
// signature: author signature provided in the message payload
// peerID: author peer id from the message payload
// pubKeyData: author public key from the message payload
func (n *nodeImpl) verifyData(data []byte, signature []byte, peerID peer.ID, pubKeyData []byte) bool {
	key, err := crypto.UnmarshalPublicKey(pubKeyData)
	if err != nil {
		log.Errorf("Failed to extract key from message key data Err:%s", err.Error())
		return false
	}

	// extract nodeImpl id from the provided public key
	idFromKey, err := peer.IDFromPublicKey(key)

	if err != nil {
		log.Errorf("Failed to extract peer id from public key Err:%s", err.Error())
		return false
	}

	// verify that message author nodeImpl id matches the provided nodeImpl public key
	if idFromKey != peerID {
		log.Errorf("Node id and provided public key mismatch Err:%s", err.Error())
		return false
	}

	res, err := key.Verify(data, signature)
	if err != nil {
		log.Errorf("Error authenticating data Err:%s", err.Error())
		return false
	}

	return res
}

func (n *nodeImpl) signProtoMessage(message proto.Message) ([]byte, error) {
	data, err := proto.Marshal(message)
	if err != nil {
		return nil, err
	}
	return n.signData(data)
}

// sign binary data using the local nodeImpl's private key
func (n *nodeImpl) signData(data []byte) ([]byte, error) {
	key := n.Peerstore().PrivKey(n.ID())
	res, err := key.Sign(data)
	return res, err
}

// newMessageData helper method - generate message data shared between all nodeImpl's p2p protocols
// messageID: unique for requests, copied from request for responses
func (n *nodeImpl) newMessageData(messageID string, gossip bool) *corepb.MessageData {
	// Add protobufs bin data for message author public key
	// this is useful for authenticating  messages forwarded by a nodeImpl authored by another nodeImpl
	nodePubKey, err := n.Peerstore().PubKey(n.ID()).Bytes()

	if err != nil {
		panic("Failed to get public key for sender from local peer store.")
	}

	return &corepb.MessageData{
		ClientVersion: clientVersion,
		NodeId:        peer.IDB58Encode(n.ID()),
		NodePubKey:    nodePubKey,
		Timestamp:     time.Now().Unix(),
		Id:            messageID,
		Gossip:        gossip,
	}
}

// helper method - writes a protobuf go data object to a network stream
// data: reference of protobuf go data object to send (not the object itself)
// s: network stream to write the data to
func (n *nodeImpl) sendProtoMessage(id peer.ID, p protocol.ID, data proto.Message) error {
	s, err := n.NewStream(context.Background(), id, p)
	if err != nil {
		log.Errorf("Failed opening new stream Err:%s", err.Error())
		return err
	}
	writer := ggio.NewFullWriter(s)
	err = writer.WriteMsg(data)
	if err != nil {
		log.Errorf("Failed writing message data Err:%s", err.Error())
		s.Reset()
		return err
	}
	// FullClose closes the stream and waits for the other side to close their half.
	err = inet.FullClose(s)
	if err != nil {
		log.Errorf("Failed to close stream Err:%s", err.Error())
		s.Reset()
		return err
	}
	return nil
}

// readProtoFromStream helper to read proto message from stream
func readProtoFromStream(s inet.Stream, msg proto.Message) error {
	buf, err := ioutil.ReadAll(s)
	if err != nil {
		s.Reset()
		log.Errorf("Failed reading proto from stream Err:%s", err.Error())
		return err
	}
	s.Close()

	proto.Unmarshal(buf, msg)
	if err != nil {
		log.Errorf("Failed unmarshaling message Err:%s", err.Error())
		return err
	}
	return nil
}

// SendReqProtoMsg : Implementation of MsgSender interface
func (n *nodeImpl) SendReqProtoMsg(p peer.ID, id protocol.ID, msg proto.Message) error {
	if authMsg, ok := msg.(AuthenticatedMsg); ok {
		msgData := n.newMessageData(uuid.New().String(), false)
		signature, err := n.signProtoMessage(msg)
		if err != nil {
			log.Errorf("Failed signing proto message Err:%s", err.Error())
			return err
		}
		msgData.Sign = signature
		authMsg.SetMessageData(msgData)
	}

	return n.sendProtoMessage(p, id, msg)
}

// SendRespProtoMsg : Implementation of MsgSender interface
func (n *nodeImpl) SendRespProtoMsg(msgID string, p peer.ID, id protocol.ID, msg proto.Message) error {
	if authMsg, ok := msg.(AuthenticatedMsg); ok {
		msgData := n.newMessageData(msgID, false)
		signature, err := n.signProtoMessage(msg)
		if err != nil {
			log.Errorf("Failed signing proto message Err:%s", err.Error())
			return err
		}
		msgData.Sign = signature
		authMsg.SetMessageData(msgData)
	}

	return n.sendProtoMessage(p, id, msg)
}

func (n *nodeImpl) ReceiveProtoMsg(s inet.Stream, msg proto.Message) error {
	err := readProtoFromStream(s, msg)
	if err != nil {
		return err
	}

	if authMsg, ok := msg.(AuthenticatedMsg); ok {
		msgData := authMsg.GetMessageData()
		if msgData == nil {
			log.Errorf("Authentication data not present")
			return errors.New("Auth data not present in message")
		}

		if !n.authenticateMessage(msg, msgData) {
			return errors.New("Failed authenticating message")
		}
	}

	return nil
}

type netNotifiee nodeImpl

func (nn *netNotifiee) impl() *nodeImpl {
	return (*nodeImpl)(nn)
}

func (nn *netNotifiee) Connected(n inet.Network, p inet.Conn) {
	peerConnectedWork := &peerConnectedTask{
		nd:   nn.impl(),
		conn: p,
	}

	workChan := make(chan task, 1)
	workChan <- peerConnectedWork

	nn.impl().taskQueue <- workChan
	log.Infof("Enqueued peerConnectedTask for %s", p.RemotePeer().Pretty())
	return
}

func (nn *netNotifiee) Disconnected(n inet.Network, v inet.Conn) {
	peerDisconnectedWork := &peerDisconnectedTask{
		nd:   nn.impl(),
		conn: v,
	}

	workChan := make(chan task, 1)
	workChan <- peerDisconnectedWork

	nn.impl().taskQueue <- workChan
	log.Infof("Enqueued peerDisconnectedTask for %s", v.RemotePeer().Pretty())
	return

}

func (nn *netNotifiee) OpenedStream(n inet.Network, v inet.Stream) {}
func (nn *netNotifiee) ClosedStream(n inet.Network, v inet.Stream) {}
func (nn *netNotifiee) Listen(n inet.Network, a ma.Multiaddr)      {}
func (nn *netNotifiee) ListenClose(n inet.Network, a ma.Multiaddr) {}

type discoveryNotifiee nodeImpl

func (dn *discoveryNotifiee) impl() *nodeImpl {
	return (*nodeImpl)(dn)
}

func (dn *discoveryNotifiee) HandlePeerFound(pi pstore.PeerInfo) {
	log.Infof("Peer discovery %s", pi.ID.Pretty())
	peerFoundWork := &peerFoundTask{
		pi: pi,
		nd: dn.impl(),
	}

	workChan := make(chan task, 1)
	workChan <- peerFoundWork

	dn.impl().taskQueue <- workChan
	log.Infof("Enqueued peerFoundTask for %s", pi.ID.Pretty())
	return
}

type peerConnectedTask struct {
	nd   *nodeImpl
	conn inet.Conn
}

func (p *peerConnectedTask) Execute() {
	store := p.nd.Repo.KVRepo()

	n := &objects.NodeInfo{
		NodeInfo: &hbpb.NodeInfo{
			IdBytes: []byte(p.conn.RemotePeer().Pretty()),
		},
	}

	err := store.Read(n)
	n.Status = "Connected"
	if err != nil {
		err = store.Create(n)
	} else {
		err = store.Update(n)
	}
	if err != nil {
		log.Errorf("Failed updating node info on new connection Err:%s", err.Error())
	}

	log.Infof("Connected to peer %s", p.conn.RemotePeer().Pretty())

	log.Info("Sending heartbeat...")
	p.nd.Heartbeat(p.conn.RemotePeer())
	return
}

type peerDisconnectedTask struct {
	nd   *nodeImpl
	conn inet.Conn
}

func (p *peerDisconnectedTask) Execute() {
	store := p.nd.Repo.KVRepo()

	n := &objects.NodeInfo{
		NodeInfo: &hbpb.NodeInfo{
			IdBytes: []byte(p.conn.RemotePeer().Pretty()),
		},
	}

	err := store.Read(n)
	if err != nil {
		log.Errorf("Expected peer to be in store Err:%s", err.Error())
	}

	log.Infof("Disconnected to peer %s", p.conn.RemotePeer().Pretty())

	n.Status = "Disconnected"
	err = store.Update(n)
	if err != nil {
		log.Errorf("Failed updating nodeinfo on closing connection Err:%s", err.Error())
	}
	return
}

type peerFoundTask struct {
	nd *nodeImpl
	pi pstore.PeerInfo
}

func (p *peerFoundTask) Execute() {
	store := p.nd.Repo.KVRepo()

	n := &objects.NodeInfo{
		NodeInfo: &hbpb.NodeInfo{
			IdBytes: []byte(p.pi.ID.Pretty()),
		},
	}

	log.Infof("Handle peer found %s", n.GetId())

	err := store.Read(n)
	if err != nil {
		log.Infof("New Peer found %s", n.GetId())
		n.Status = "Discovered"
		err = store.Create(n)
		if err != nil {
			log.Errorf("Failed creating nodeInfo Err:%s", err.Error())
			return
		}
	}
	// Add peer to address book
	p.nd.Peerstore().AddAddrs(p.pi.ID, p.pi.Addrs, pstore.PermanentAddrTTL)

	err = p.nd.Connect(context.Background(), p.pi)
	if err != nil {
		log.Errorf("Failed connecting to peer %v Err:%s", p.pi, err.Error())
		n.Status = "Failed to connect"
		err = store.Update(n)
		if err != nil {
			log.Errorf("Failed updating failure status Err:%s", err.Error())
		}
	}

	return
}
