package meta

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"os"
	"path"
)

type task struct {
	root string
}

func (m *task) Dir() string {
	return m.rootLocation()
}

func (m *task) rootLocation(name ...string) string {
	return path.Join(m.root, "tasks", path.Join(name...))
}

func (m *task) Get(key string) (*Task, error) {
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
	return m.load(pathName)
}

func (m *task) List() ([]*Task, error) {
	var machines []*Task
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
		machines = append(machines, &Task{Id: dirName})
	}
	return machines, nil
}

func (m *task) Create(image *Task) error {
	pathName := m.rootLocation(image.Id)
	_, err := os.Stat(pathName)
	if err == nil {
		return fmt.Errorf("%s already exists", pathName)
	}
	data, err := json.MarshalIndent(image, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(pathName, data, 0644)
}

func (m *task) Update(image *Task) error {
	pathName := m.rootLocation(image.Id)
	_, err := os.Stat(pathName)
	if err != nil {
		return fmt.Errorf("%s not exists", pathName)
	}
	data, err := json.MarshalIndent(image, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(pathName, data, 0644)
}

func (m *task) Remove(image *Task) error {
	if image.Id == "" {
		return fmt.Errorf("image name is empty")
	}
	return os.RemoveAll(m.rootLocation(image.Id))
}

func (m *task) load(machineUri string) (*Task, error) {
	data, err := os.ReadFile(machineUri)
	if err != nil {
		return nil, err
	}
	var mch Task
	err = json.Unmarshal(data, &mch)
	return &mch, err
}

func (m *task) Stop(image *Task) error {
	return fmt.Errorf("unimplemented image stop")
}
