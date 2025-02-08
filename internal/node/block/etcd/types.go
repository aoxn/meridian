package etcd

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
)

// Endpoint Status
type EndpointStatus struct {
	Endpoint string `json:"Endpoint,omitempty" protobuf:"bytes,1,opt,name=Endpoint"`
	Status   Status `json:"Status,omitempty" protobuf:"bytes,2,opt,name=Status"`
}

type Status struct {
	Header    Header   `json:"header,omitempty" protobuf:"bytes,1,opt,name=header"`
	Version   string   `json:"version,omitempty" protobuf:"bytes,2,opt,name=version"`
	DBsize    *big.Int `json:"dbsize,omitempty" protobuf:"bytes,3,opt,name=dbsize"`
	Leader    *big.Int `json:"leader,omitempty" protobuf:"bytes,4,opt,name=leader"`
	RaftIndex *big.Int `json:"raftIndex,omitempty" protobuf:"bytes,5,opt,name=raftIndex"`
	RaftTerm  *big.Int `json:"raftTerm,omitempty" protobuf:"bytes,6,opt,name=raftTerm"`
}

type Header struct {
	ClusterID *big.Int `json:"cluster_id,omitempty" protobuf:"bytes,1,opt,name=cluster_id"`
	MemberID  *big.Int `json:"member_id,omitempty" protobuf:"bytes,2,opt,name=member_id"`
	Revision  *big.Int `json:"revision,omitempty" protobuf:"bytes,3,opt,name=revision"`
	RaftTerm  *big.Int `json:"raft_term,omitempty" protobuf:"bytes,4,opt,name=raft_term"`
}

// Endpoint Health

type EndpointHealth struct {
	Endpoint string `json:"service,omitempty" protobuf:"bytes,1,opt,name=service"`
	Health   string `json:"health,omitempty" protobuf:"bytes,2,opt,name=health"`
	Took     string `json:"took,omitempty" protobuf:"bytes,3,opt,name=took"`
	ErrorStr string `json:"error,omitempty" protobuf:"bytes,4,opt,name=error"`
}

// Member List

type Members struct {
	Header  Header   `json:"header,omitempty" protobuf:"bytes,1,opt,name=header"`
	Members []Member `json:"members,omitempty" protobuf:"bytes,2,opt,name=members"`
}

func Load(r []string, o interface{}) error {
	result := strings.Join(r, "\n")
	if strings.TrimSpace(result) == "" {
		return fmt.Errorf("etcd command result, empty string")
	}
	return json.Unmarshal([]byte(result), o)
}
