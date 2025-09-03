package meta

import (
	"context"
	"fmt"
	"os"
	"path"
)

type Backend interface {
	K8S() AbstractK8S

	Task() AbstractTask

	Config() AbstractConfig

	Machine() AbstractMachine

	Image() AbstractImage

	Docker() AbstractDocker
}

type Dir interface {
	Dir() string
}

type AbstractConfig interface {
	Dir
	Get(key string) (*Config, error)
	Set(cfg *Config) error
}

type AbstractMachine interface {
	Dir
	Get(key string) (*Machine, error)
	List() ([]*Machine, error)
	Create(machine *Machine) error
	Update(machine *Machine) error
	Destroy(machine *Machine) error
}

type AbstractImage interface {
	Dir
	Get(key string) (*Image, error)
	List() ([]*Image, error)
	Create(image *Image) error
	Update(image *Image) error
	Pull(ctx context.Context, name string, opt *PullOpt) error
	Remove(name string) error
}

type AbstractTask interface {
	Dir
	Get(key string) (*Task, error)
	List() ([]*Task, error)
	Create(t *Task) error
	Stop(t *Task) error
	Remove(t *Task) error
}

type AbstractDocker interface {
	Dir
	Get(key string) (*Docker, error)
	List() ([]*Docker, error)
	Create(d *Docker) error
	Update(d *Docker) error
	Remove(d *Docker) error
}

type AbstractK8S interface {
	Dir
	Get(key string) (*Kubernetes, error)
	List() ([]*Kubernetes, error)
	Create(k *Kubernetes) error
	Update(k *Kubernetes) error
	Remove(k *Kubernetes) error
}

type Docker struct {
	Name    string
	Version string
	VmName  string
	State   string
}

type Task struct {
	Id string `json:"id"`
}

type Config struct {
	AbsDir string `json:"absDir"`
}

func (cfg *Config) Dir() string {
	return cfg.AbsDir
}

type Image struct {
	Name     string            `json:"name"`
	OS       string            `json:"os"`
	Arch     string            `json:"arch"`
	Version  string            `json:"version"`
	Location string            `json:"location"`
	Labels   map[string]string `json:"labels"`
}

var Local = newLocalOrPanic()

var (
	_ AbstractConfig  = &config{}
	_ AbstractImage   = &image{}
	_ AbstractMachine = &machine{}
)

func DftRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(fmt.Sprintf("failed to get home directory: %v", err))
	}
	return path.Join(home, defaultRoot), nil
}

func NewLocalOrDie() Backend {
	var genRoot string
	home, err := os.UserHomeDir()
	if err != nil {
		panic(fmt.Sprintf("failed to get home directory: %v", err))
	}
	genRoot = path.Join(home, defaultRoot)
	return &local{genRoot}
}

func newLocalOrPanic() Backend {
	var genRoot string
	home, err := os.UserHomeDir()
	if err != nil {
		panic(fmt.Sprintf("failed to get home directory: %v", err))
	}
	genRoot = path.Join(home, defaultRoot)
	return &local{genRoot}
}

func NewLocal(root ...string) (Backend, error) {
	var genRoot string
	if len(root) == 0 {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		genRoot = path.Join(home, defaultRoot)
	} else {
		genRoot = root[0]
	}
	return &local{genRoot}, nil
}

type local struct {
	root string
}

func (l *local) Config() AbstractConfig {
	cfg := &config{root: l.root}
	_, err := os.Stat(cfg.Dir())
	if err != nil {
		if os.IsNotExist(err) {
			_ = os.MkdirAll(cfg.Dir(), 0755)
		}
	}
	return cfg
}

func (l *local) Docker() AbstractDocker {
	cfg := &docker{root: l.root}
	_, err := os.Stat(cfg.Dir())
	if err != nil {
		if os.IsNotExist(err) {
			_ = os.MkdirAll(cfg.Dir(), 0755)
		}
	}
	return cfg
}

func (l *local) Machine() AbstractMachine {
	mch := &machine{root: l.root}
	_, err := os.Stat(mch.Dir())
	if err != nil {
		if os.IsNotExist(err) {
			_ = os.MkdirAll(mch.Dir(), 0755)
		}
	}
	return mch
}

func (l *local) Image() AbstractImage {
	img := &image{root: l.root}
	_, err := os.Stat(img.Dir())
	if err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(img.Dir(), 0755)
		}
	}
	return img
}

func (l *local) K8S() AbstractK8S {
	img := &kubernetes{root: l.root}
	_, err := os.Stat(img.Dir())
	if err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(img.Dir(), 0755)
		}
	}
	return img
}

func (l *local) Task() AbstractTask {
	return &task{root: l.root}
}

const (
	defaultRoot = ".meridian"
	machineJson = "machine.json"
	imageJson   = "image.json"
	dockerJson  = "docker.json"
)
