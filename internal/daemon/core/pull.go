package core

import (
	"context"
	"fmt"
	"github.com/aoxn/meridian/internal/tool/downloader"
	"github.com/aoxn/meridian/internal/vmm/meta"
	"k8s.io/klog/v2"
	"sync"
	"time"
)

func NewLocalImageMgr(bk meta.Backend) *LocalImageMgr {
	return &LocalImageMgr{
		backend: bk,
		mu:      &sync.RWMutex{},
		pulling: map[string]*Pulling{},
	}
}

type LocalImageMgr struct {
	mu      *sync.RWMutex
	pulling map[string]*Pulling
	backend meta.Backend
}

func (img *LocalImageMgr) Pull(name string) (*Pulling, error) {
	img.mu.Lock()
	defer img.mu.Unlock()
	pull, ok := img.pulling[name]
	if !ok {
		dBar, err := downloader.New(0)
		if err != nil {
			return nil, err
		}
		pBar, err := downloader.New(0)
		if err != nil {
			return nil, err
		}
		pull = &Pulling{
			PullOption: &meta.PullOpt{
				DecompressBar: dBar,
				DownloadBar:   pBar,
			},
			complete: false,
		}
		img.pulling[name] = pull
		go func(pull *Pulling) {
			defer img.remove(name)
			err := img.backend.Image().Pull(context.TODO(), name, pull.PullOption)
			pull.err = err
			pull.complete = true
			klog.Errorf("pull image [%s] complete: %v", name, err)
		}(pull)
		return pull, nil
	}
	return pull, nil
}

func (img *LocalImageMgr) remove(name string) {
	img.mu.Lock()
	defer img.mu.Unlock()
	delete(img.pulling, name)
}

type Pulling struct {
	err        error
	complete   bool
	PullOption *meta.PullOpt
}

func (p *Pulling) Decode() meta.Status {
	var data []meta.StatusData
	if p.PullOption != nil && p.PullOption.DownloadBar != nil {

		data = append(data, meta.StatusData{
			Id:      "download",
			Current: p.PullOption.DownloadBar.Current(),
			Total:   p.PullOption.DownloadBar.Total(),
		})
	}
	if p.PullOption != nil && p.PullOption.DecompressBar != nil {

		data = append(data, meta.StatusData{
			Id:      "decompress",
			Current: p.PullOption.DecompressBar.Current(),
			Total:   p.PullOption.DecompressBar.Total(),
		})
	}
	if p.err == nil && p.complete {
		p.err = fmt.Errorf("PullComplete")
	}
	status := meta.Status{
		Data: data,
	}
	if p.err != nil {
		status.Err = fmt.Sprintf("%s", p.err.Error())
	}
	return status
}

func (p *Pulling) Wait(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context canceled")
		default:
		}
		klog.Infof("debug waiting pull:  %+v,  %+v", p.err, p.complete)
		if p.err != nil {
			return p.err
		}
		if p.complete {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
}
