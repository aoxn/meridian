package log

import (
	"fmt"
	tm "github.com/buger/goterm"
	"k8s.io/klog/v2"
	"math/rand"
	"strings"
	"sync"
	"time"
)

// custom CLI loading spin for kind
var bframes = []string{
	"⠈⠁",
	"⠈⠑",
	"⠈⠱",
	"⠈⡱",
	"⢀⡱",
	"⢄⡱",
	"⢄⡱",
	"⢆⡱",
	"⢎⡱",
	"⢎⡰",
	"⢎⡠",
	"⢎⡀",
	"⢎⠁",
	"⠎⠁",
	"⠊⠁",
}

type Pgmbar struct {
	started time.Time
	lastIdx int
	mu      sync.Mutex
	meesage Resource
	events  []Resource
	done    chan bool
	ticker  *time.Ticker
}
type Resource struct {
	StartedTime    string
	UpdatedTime    string
	EventId        string
	ResourceType   string
	ResourceId     string
	ResourceName   string
	StatusReason   string
	ResourceStatus string
}

func NewPgmbar(created string, resource []Resource) *Pgmbar {
	when := time.Now()
	if created != "" {
		m, err := time.ParseInLocation("2006-01-02T15:04:05", created, time.Local)
		if err != nil {
			klog.Errorf("created time format: %s", err.Error())
		}
		when = m
	}
	bar := &Pgmbar{
		started: when,
		meesage: Resource{
			ResourceId:     "extra_mesage_id",
			ResourceType:   "WDRIP::MESSAGE::OUTPUT",
			ResourceName:   "extra message",
			ResourceStatus: "Initialized",
			UpdatedTime:    "1s",
		},
		events: resource,
		done:   make(chan bool),
		ticker: time.NewTicker(1 * time.Second),
	}
	go bar.run()
	return bar
}

func (b *Pgmbar) SetMessageWithTime(m string, now string) {
	when := time.Now()
	if now != "" {
		m, err := time.ParseInLocation("2006-01-02T15:04:05", now, time.Local)
		if err != nil {
			klog.Errorf("created time format: %s", err.Error())
		}
		when = m
	}
	//logrus.Errorf("\n\n\n\n\n\n\n\n\n\nSETTTT: started=%s, now=%s, when=%s, secondes=%d\n\n\n\n",b.started, now,when, when.Sub(b.started)/time.Second)
	b.meesage.UpdatedTime = tm.Color(
		fmt.Sprintf(
			"TimeElapse: %ds",
			when.Sub(b.started)/time.Second,
		),
		tm.MAGENTA,
	)
	b.meesage.ResourceStatus = m
}

func (b *Pgmbar) AddEvents(res []Resource) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, r := range res {
		b.AddEvent(r)
	}
}

func (b *Pgmbar) AddEvent(newr Resource) {
	found := false
	for i := range b.events {
		rid := b.events[i].ResourceId
		if rid == newr.ResourceId {
			found = true
			b.events[i] = newr
			break
		}
	}
	if !found {
		b.events = append(b.events, newr)
	}
}

var (
	SUCCESS = "SUCCESS"
)

func (b *Pgmbar) Finish(succ string) {
	b.SetMessageWithTime(fmt.Sprintf("Stack %s", succ), "")
	b.done <- succ == SUCCESS
}

func (b *Pgmbar) run() {
	// do flush buffer
	for {
		select {
		case <-b.done:
			b.PrintEvents()
			return
		case <-b.ticker.C:
			b.PrintEvents()
		}
	}
}

func (b *Pgmbar) PrintEvents() {
	b.mu.Lock()
	defer b.mu.Unlock()
	tm.MoveCursorUp(b.lastIdx)
	_, err := tm.Print(
		message(append(b.events, b.meesage), b.lastIdx))
	if err != nil {
		panic(fmt.Sprintf("%s", err.Error()))
	}
	_, _ = tm.Print("\r\n")
	b.lastIdx = len(b.events) + 1
	tm.Flush()
}

func message(reso []Resource, last int) string {
	var result []string
	count := len(bframes) - 1
	reasoncolor := tm.GREEN
	for i := range reso {
		frame := randframes(count)
		status := strings.ToLower(reso[i].ResourceStatus)
		if strings.Contains(status, "fail") {
			frame = tm.Color("✗ ", tm.RED)
			reasoncolor = tm.RED
		} else if strings.Contains(status, "complete") {
			frame = tm.Color("✓ ", tm.CYAN)
			reasoncolor = tm.CYAN
		} else {
			reasoncolor = tm.GREEN
		}
		result = append(result,
			tm.ResetLine(
				fmt.Sprintf(
					"%2s【%-44s】(%-35s) [%25s,%d, %d] %s %s",
					frame,
					tm.Bold(reso[i].ResourceType),
					tm.Color(reso[i].ResourceId, tm.BLUE),
					tm.Color(tm.Bold(reso[i].ResourceStatus), reasoncolor),
					len(reso), last,
					reso[i].StartedTime,
					reso[i].UpdatedTime,
					//reso[i].StatusReason,
					//reso[i].ResourceId,
				),
			),
		)
	}
	return strings.Join(result, "\n")
}

var seed = rand.New(rand.NewSource(time.Now().UnixNano()))

func randframes(le int) string { return bframes[seed.Intn(le)] }

var bartest = `
	bar := log.NewPgmbar([]log.Resource{
		{
			ResourceId: "master-id-001",
			StatusReason: "Creating",
			ResourceStatus: "Creating",
			ResourceName: "k8s-master",
			ResourceType: "ALIYUN::ECS::INSTANCE",
		},
		{
			ResourceId: "worker-id-002",
			StatusReason: "Creating",
			ResourceStatus: "Creating",
			ResourceName: "k8s-worker",
			ResourceType: "ALIYUN::ECS::INSTANCE",
		},{
			ResourceId: "slb-id-001",
			StatusReason: "Creating",
			ResourceStatus: "Creating",
			ResourceName: "k8s-slb",
			ResourceType: "ALIYUN::ECS::LOADBALANCER",
		},
	})
	time.Sleep(4*time.Second)
	bar.AddEvents([]log.Resource{
		{
			ResourceId: "master-id-001",
			StatusReason: "Creating",
			ResourceStatus: "Complete",
			ResourceName: "k8s-master",
			ResourceType: "ALIYUN::ECS::INSTANCE",
		},
		{
			ResourceId: "worker-id-002",
			StatusReason: "Fail fast",
			ResourceStatus: "Fail",
			ResourceName: "k8s-worker",
			ResourceType: "ALIYUN::ECS::INSTANCE",
		},
	})
	time.Sleep(3 * time.Second)
	return nil
`
