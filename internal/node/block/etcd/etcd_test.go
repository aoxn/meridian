package etcd

import (
	"encoding/json"
	"fmt"
	"github.com/aoxn/meridian/internal/tool"
	"math/big"
	"testing"
)

func TestE2E(t *testing.T) {
	mems := `
{"header":{"cluster_id":18421691612643254815,"member_id":15864905497554707009,"raft_term":4},"members":[{"ID":308211321195050191,"name":"etcd-192.168.0.32.member","peerURLs":["https://192.168.0.32:2380"],"clientURLs":["https://192.168.0.32:2379"]},{"ID":14952074906564636628,"name":"etcd-192.168.0.30.member","peerURLs":["https://192.168.0.30:2380"],"clientURLs":["https://192.168.0.30:2379"]},{"ID":15864905497554707009,"name":"etcd-192.168.0.31.member","peerURLs":["https://192.168.0.31:2380"],"clientURLs":["https://192.168.0.31:2379"]}]}
`

	end := `
[{"Endpoint":"https://192.168.0.31:2379","Status":{"header":{"cluster_id":18421691612643254815,"member_id":15864905497554707009,"revision":296665,"raft_term":4},"version":"3.3.8","dbSize":4415488,"leader":14952074906564636628,"raftIndex":360582,"raftTerm":4}}]
`
	mem := Members{}
	err := json.Unmarshal([]byte(mems), &mem)
	if err != nil {
		t.Fatal(err.Error())
	}
	var endp []EndpointStatus
	err = json.Unmarshal([]byte(end), &endp)
	if err != nil {
		t.Fatal(err.Error())
	}
	fmt.Println(tool.PrettyYaml(mem))
	fmt.Println(tool.PrettyYaml(endp))
	me := big.NewInt(0)
	me.SetString("15864905497554707009", 10)
	fmt.Printf(fmt.Sprintf("%x\n", me))

	zero := big.NewInt(0)
	fmt.Printf("%t\n", zero.String() == "0")
	fmt.Printf(fmt.Sprintf("%x\n", zero))

}
