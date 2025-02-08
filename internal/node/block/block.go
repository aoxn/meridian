/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package block

import (
	"context"
	"fmt"
	"github.com/aoxn/meridian/internal/tool"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"reflect"
	"strings"
	"sync"
	"time"
)

type Block interface {
	// Name block的名称
	Name() string

	// Ensure 确保模块被正确安装，调谐幂等
	Ensure(ctx context.Context) error

	// Purge 确保模块被清理卸载，幂等
	Purge(ctx context.Context) error

	// CleanUp 清理
	CleanUp(ctx context.Context) error
}

const RETRY_WORD = "please retry"

var _ Block = &ConcurrentBlock{}

func DefaultRetry(action Block) Block {
	return WithRetry(
		action,
		wait.Backoff{
			Duration: 1 * time.Second,
			Factor:   2,
			Steps:    4,
		},
		// default retry decider
		[]NeedRetry{
			func(err error) bool {
				if err == nil {
					return false
				}
				return strings.Contains(err.Error(), RETRY_WORD)
			},
		},
	)
}

// WithRetry Excute with retry with given backoff
// policy and retryDeside
func WithRetry(
	action Block,
	backOff wait.Backoff,
	retryOn []NeedRetry,
) Block {
	return &Retryable{
		Block:   action,
		BackOff: backOff,
		RetryOn: retryOn,
	}
}

type Retryable struct {
	Block
	BackOff wait.Backoff
	RetryOn []NeedRetry
}

func (r *Retryable) Ensure(ctx context.Context) error {
	return wait.ExponentialBackoff(
		r.BackOff,
		func() (done bool, err error) {
			err = r.Ensure(ctx)
			for _, need := range r.RetryOn {
				if need(err) {
					klog.Errorf("retry on error: %s", err.Error())
					return false, nil
				}
			}
			return true, err
		},
	)
}

type NeedRetry func(error) bool

// NewConcurrentBlock execute block concurrently
func NewConcurrentBlock(rand []Block) Block {
	return &ConcurrentBlock{concurrent: rand}
}

type ConcurrentBlock struct{ concurrent []Block }

func (u *ConcurrentBlock) Ensure(ctx context.Context) error {
	var errs tool.Errors
	grp := sync.WaitGroup{}
	for _, action := range u.concurrent {
		grp.Add(1)
		go func(act Block) {
			klog.Infof("start to execute block: %s", reflect.ValueOf(act).Type())
			defer grp.Done()
			err := act.Ensure(ctx)
			if err != nil {
				errs = append(errs, err)
				klog.Errorf("run action concurrent error: %s", err.Error())
			}
		}(action)
	}
	klog.Infof("wait for concurrent block to finish")
	grp.Wait()
	klog.Infof("concurrent block finished")
	if len(errs) <= 0 {
		return nil
	}
	return errs
}

func (u *ConcurrentBlock) Name() string {
	return fmt.Sprintf("concurrent bock")
}

func (u *ConcurrentBlock) Purge(ctx context.Context) error {
	var errs tool.Errors
	grp := sync.WaitGroup{}
	for _, action := range u.concurrent {
		grp.Add(1)
		go func(act Block) {
			klog.Infof("start to execute block: %s", reflect.ValueOf(act).Type())
			defer grp.Done()
			err := act.Purge(ctx)
			if err != nil {
				errs = append(errs, err)
				klog.Errorf("run action concurrent error: %s", err.Error())
			}
		}(action)
	}
	klog.Infof("wait for concurrent block to finish")
	grp.Wait()
	klog.Infof("concurrent block finished")
	if len(errs) <= 0 {
		return nil
	}
	return errs
}

func (u *ConcurrentBlock) CleanUp(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func RunBlocks(
	actions []Block,
) error {
	ctx := context.TODO()
	for _, action := range actions {
		klog.Infof("run action: %s", reflect.TypeOf(action))
		err := action.Ensure(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}
