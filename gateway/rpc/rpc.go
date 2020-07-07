package rpc

import (
	"context"
	"errors"

	"github.com/aloknerurkar/gopcp/core"
	hbpb "github.com/aloknerurkar/gopcp/core/heartbeat/pb"
	"github.com/aloknerurkar/gopcp/core/objects"
	corepb "github.com/aloknerurkar/gopcp/core/pb"
	pb "github.com/aloknerurkar/gopcp/gateway/pb"
)

type gateway struct {
	node core.Node
}

// NewGwService will return a new RPC server handler
func NewGwService(nd core.Node) pb.GoPcpServer {
	return &gateway{node: nd}
}

func (g *gateway) Nodes(ctx context.Context, req *pb.EmtpyMessage) (*pb.NodeList, error) {

	result, err := g.node.Repository().KVRepo().ListAll(func(i int) core.Items {
		l := make([]core.Item, i)
		for j := range l {
			l[j] = &objects.NodeInfo{NodeInfo: &hbpb.NodeInfo{}}
		}
		return l
	})

	if err != nil {
		return nil, errors.New("Failed listing nodes in the system Err:" + err.Error())
	}

	out := &pb.NodeList{
		Nodes: make([]*pb.NodeList_Item, len(result)),
	}

	for i, v := range result {
		svcInfo := &objects.ServicesInfo{
			HeartbeatResp: &hbpb.HeartbeatResp{
				MessageData: &corepb.MessageData{
					NodeId: v.GetId(),
				},
			},
		}

		out.Nodes[i] = &pb.NodeList_Item{
			Info: v.(*objects.NodeInfo).NodeInfo,
			Svcs: make([]*hbpb.ServiceInfo, 0),
		}

		err = g.node.Repository().KVRepo().Read(svcInfo)
		if err != nil {
			continue
		}

		out.Nodes[i].Svcs = append(out.Nodes[i].Svcs, svcInfo.Services...)
		out.Nodes[i].LastHeartbeat = svcInfo.Updated
	}

	return out, nil
}
