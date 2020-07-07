package heartbeat

import (
	"context"

	"github.com/aloknerurkar/gopcp/core"
	"github.com/aloknerurkar/gopcp/core/objects"

	pb "github.com/aloknerurkar/gopcp/core/heartbeat/pb"
	corepb "github.com/aloknerurkar/gopcp/core/pb"
	logger "github.com/ipfs/go-log"
	inet "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
)

var log = logger.Logger("Core::Heartbeat")

const heartbeatRequest = "/heartbeat/req/0.0.1"
const heartbeatResponse = "/heartbeat/resp/0.0.1"

/*
 * Heartbeat protocol
 */
type heartbeat struct {
	ctx  context.Context
	node core.Node
}

// NewHeartbeatProtocol initializes the handlers for heartbeat protocol
func NewHeartbeatProtocol(ctx context.Context, node core.Node) core.HeartbeatProtocol {
	p := &heartbeat{
		ctx:  ctx,
		node: node,
	}
	node.P2PHost().SetStreamHandler(heartbeatRequest, p.onHeartbeatRequest)
	node.P2PHost().SetStreamHandler(heartbeatResponse, p.onHeartbeatResponse)
	return p
}

type respMsg struct {
	*pb.HeartbeatResp
}

func (r *respMsg) SetMessageData(data *corepb.MessageData) {
	r.MessageData = data
}

func (p *heartbeat) onHeartbeatRequest(s inet.Stream) {

	cctx := log.Start(p.ctx, "Heartbeat.onHeartbeatRequest")
	data := &corepb.EmptyMessage{}

	err := p.node.Receiver().ReceiveProtoMsg(s, data)
	if err != nil {
		// Error will be logged by receiver
		log.FinishWithErr(cctx, err)
		return
	}
	log.Infof("%s: Received heartbeat request from %s.", s.Conn().LocalPeer(),
		s.Conn().RemotePeer())

	result, err := p.node.Repository().FsRepo().ListAll(func(i int) core.Items {
		l := make([]core.Item, i)
		for j := range l {
			l[j] = &objects.FileP{Table: "bin"}
		}
		return l
	})
	if err != nil {
		log.Errorf("Failed listing files. Err:%s", err.Error())
		log.FinishWithErr(cctx, err)
		return
	}

	resp := &respMsg{HeartbeatResp: &pb.HeartbeatResp{
		Services: make([]*pb.ServiceInfo, len(result))}}
	for i, v := range result {
		resp.Services[i] = &pb.ServiceInfo{
			Name:    v.GetId(),
			Version: "0.0.1",
			Status:  "Installed",
		}
	}
	err = p.node.Sender().SendRespProtoMsg(data.MessageData.Id, s.Conn().RemotePeer(),
		heartbeatResponse, resp)
	if err != nil {
		// Error will be logged by sender
		log.FinishWithErr(cctx, err)
		return
	}
	log.Infof("%s: Sent heartbeat response to %s.", s.Conn().LocalPeer(),
		s.Conn().RemotePeer())
	log.Finish(cctx)
}

func (p *heartbeat) onHeartbeatResponse(s inet.Stream) {

	cctx := log.Start(p.ctx, "Heartbeat.onHeartbeatResponse")
	data := &pb.HeartbeatResp{}

	err := p.node.Receiver().ReceiveProtoMsg(s, data)
	if err != nil {
		log.FinishWithErr(cctx, err)
		return
	}
	log.Infof("%s: Received heartbeat response from %s.", s.Conn().LocalPeer(),
		s.Conn().RemotePeer())

	// Not storing signature in DB
	data.MessageData.Sign = nil
	data.MessageData.NodePubKey = nil

	obj := &objects.ServicesInfo{
		HeartbeatResp: data,
	}
	err = p.node.Repository().KVRepo().Update(obj)
	if err != nil {
		log.Errorf("Failed updating services info Err:%s", err.Error())
		log.FinishWithErr(cctx, err)
		return
	}

	log.Finish(cctx)
}

type reqMsg struct {
	*corepb.EmptyMessage
}

func (r *reqMsg) SetMessageData(msg *corepb.MessageData) {
	r.MessageData = msg
}

func (p *heartbeat) Heartbeat(pid peer.ID) bool {

	cctx := log.Start(p.ctx, "Heartbeat.DoHeartbeat")

	req := &reqMsg{EmptyMessage: &corepb.EmptyMessage{}}

	err := p.node.Sender().SendReqProtoMsg(pid, heartbeatRequest, req)
	if err != nil {
		log.FinishWithErr(cctx, err)
		return false
	}

	log.Infof("%s: Heartbeat to: %s was sent. Message Id: %s",
		p.node.P2PHost().ID(), pid, req.MessageData.Id)
	log.Finish(cctx)
	return true
}
