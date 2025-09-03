package meta

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"os"
	"path"
)

type docker struct {
	root string
}

func (m *docker) Dir() string {
	return m.rootLocation()
}

func (m *docker) rootLocation(name ...string) string {
	return path.Join(m.root, "docker", path.Join(name...))
}

func (m *docker) Get(key string) (*Docker, error) {
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
	return m.load(path.Join(pathName, dockerJson))
}

func (m *docker) List() ([]*Docker, error) {
	var machines []*Docker
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

func (m *docker) Create(machine *Docker) error {
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
	return os.WriteFile(path.Join(pathName, dockerJson), data, 0644)
}

func (m *docker) Update(machine *Docker) error {
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

func (m *docker) Remove(machine *Docker) error {
	if machine.Name == "" {
		return fmt.Errorf("machine name is empty")
	}
	return os.RemoveAll(m.rootLocation(machine.Name))
}

func (m *docker) load(machineUri string) (*Docker, error) {
	data, err := os.ReadFile(machineUri)
	if err != nil {
		return nil, err
	}
	var mch Docker
	err = json.Unmarshal(data, &mch)
	if err != nil {
		return nil, err
	}
	return &mch, nil
}
