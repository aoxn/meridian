package xdpin

import (
	"github.com/robfig/cron/v3"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"time"
)

type Options struct{}

type Periodical interface {
	Name() string
	Schedule() string
	Run(Options) error
}

func AddPeriodical(mgr manager.Manager) error {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		location = time.Local
		klog.Infof("load location error: %v", err)
	}
	tm := cron.New(cron.WithLocation(location))
	for _, v := range []Periodical{
		NewSLBACL(), NewSSHSGRP(), NewPortMapping(mgr), NewXdpDomain(),
	} {
		_, err := tm.AddFunc(v.Schedule(), func() {
			err = v.Run(Options{})
			if err != nil {
				klog.Errorf("run periodical task[%s] error: %s", v.Name(), err.Error())
			}
		})
		if err != nil {
			klog.Errorf("bind periodical task error: %s", err.Error())
			continue
		}
		klog.Infof("add periodical task: %v at rate[%s]", v.Name(), v.Schedule())
	}
	tm.Start()
	return nil
}
