package meta

import (
	"encoding/json"
	"fmt"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/pkg/errors"
	"os"
	"path"
)

type Kubernetes struct {
	Name    string         `yaml:"name" json:"name"`
	Version string         `yaml:"version" json:"version"`
	VmName  string         `yaml:"vmName" json:"vmName"`
	Spec    v1.RequestSpec `yaml:"spec" json:"spec"`
	State   string         `yaml:"state" json:"state"`
	Message string         `yaml:"message" json:"message"`
}

type kubernetes struct {
	root string
}

func (m *kubernetes) Dir() string {
	return m.rootLocation()
}

func (m *kubernetes) rootLocation(name ...string) string {
	return path.Join(m.root, "k8s", path.Join(name...))
}

func (m *kubernetes) Get(key string) (*Kubernetes, error) {
	pathName := m.rootLocation(key)
	info, err := os.Stat(pathName)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		return nil, errors.Wrapf(err, "NotFound: %s", key)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", pathName)
	}
	return m.load(path.Join(pathName, machineJson))
}

func (m *kubernetes) List() ([]*Kubernetes, error) {
	var machines []*Kubernetes
	pathName := m.Dir()
	info, err := os.Stat(pathName)
	if err != nil {
		return machines, err
	}
	if !info.IsDir() {
		return machines, fmt.Errorf("%s is not a directory", pathName)
	}
	// walk directory
	en, err := os.ReadDir(pathName)
	if err != nil {
		return machines, err
	}
	for _, dir := range en {
		dirName := dir.Name()
		mch, err := m.Get(dirName)
		if err != nil {
			continue
		}
		machines = append(machines, mch)
	}
	return machines, nil
}

func (m *kubernetes) Create(machine *Kubernetes) error {
	pathName := m.rootLocation(machine.Name)
	_, err := os.Stat(pathName)
	if err == nil {
		return fmt.Errorf("%s already exists", pathName)
	}
	err = os.MkdirAll(pathName, 0755)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(machine, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path.Join(pathName, machineJson), data, 0644)
}

func (m *kubernetes) Update(machine *Kubernetes) error {
	pathName := m.rootLocation(machine.Name)
	_, err := os.Stat(pathName)
	if err != nil {
		return fmt.Errorf("%s not exists", pathName)
	}
	data, err := json.MarshalIndent(machine, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path.Join(pathName, machineJson), data, 0644)
}

func (m *kubernetes) Remove(machine *Kubernetes) error {
	if machine.Name == "" {
		return fmt.Errorf("machine name is empty")
	}
	return os.RemoveAll(m.rootLocation(machine.Name))
}

func (m *kubernetes) load(machineUri string) (*Kubernetes, error) {
	data, err := os.ReadFile(machineUri)
	if err != nil {
		return nil, err
	}
	var mch Kubernetes
	err = json.Unmarshal(data, &mch)
	return &mch, err
}
