package objects

import (
	hbpb "github.com/aloknerurkar/gopcp/core/heartbeat/pb"
	"os"
)

type ServicesInfo struct {
	*hbpb.HeartbeatResp
}

// Item interface
func (s *ServicesInfo) GetId() string {
	return s.HeartbeatResp.MessageData.NodeId
}

func (s *ServicesInfo) GetTable() string {
	return "services_info"
}

// TimeTracker interface
func (s *ServicesInfo) SetCreated(t int64) {}
func (s *ServicesInfo) GetCreated() int64  { return 0 }

func (s *ServicesInfo) SetUpdated(t int64) {
	s.Updated = t
}

type FileP struct {
	*os.File
	Table string
}

func (f *FileP) SetFp(fp *os.File) {
	f.File = fp
}

func (f *FileP) GetId() string {
	if f.File != nil {
		return f.Name()
	}
	return ""
}

func (f *FileP) GetTable() string {
	return f.Table
}

type NodeInfo struct {
	*hbpb.NodeInfo
}

func (n *NodeInfo) GetTable() string {
	return "node_info"
}

func (n *NodeInfo) GetId() string {
	return string(n.IdBytes)
}
