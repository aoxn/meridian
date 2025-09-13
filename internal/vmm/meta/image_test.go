package meta

import (
	"context"
	"flag"
	"github.com/aoxn/meridian/internal/tool/downloader"
	"k8s.io/klog/v2"
	"testing"
)

func initLog() {
	klog.InitFlags(nil)
	err := flag.Set("v", "6")
	if err != nil {
		panic(err)
	}
}

func TestPull(t *testing.T) {
	initLog()
	flag.Parse()
	var l = "https://updates.cdn-apple.com/2025SpringFCS/" +
		"fullrestores/082-44534/CE6C1054-99A3-4F67-A823-3EE9E6510CDE/" +
		"UniversalMac_15.5_24F74_Restore.ipsw"
	err := pull(l)
	if err != nil {
		t.Fatalf("pull image: %s", err.Error())
	}
}

func pull(location string) error {
	dBar, err := downloader.New(0)
	if err != nil {
		return err
	}
	pBar, err := downloader.New(0)
	if err != nil {
		return err
	}
	o := &PullOpt{
		Location:      location,
		Digest:        "",
		DecompressBar: dBar,
		DownloadBar:   pBar,
	}
	return Local.Image().Pull(context.TODO(), "abc", o)
}
