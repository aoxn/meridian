package meta

import "path"

type config struct {
	root string
}

func (c *config) Dir() string {
	return c.rootLocation()
}

func (c *config) rootLocation() string {
	return path.Join(c.root, "config")
}
func (c *config) Get(key string) (*Config, error) {
	return &Config{AbsDir: c.rootLocation()}, nil
}

func (c *config) Set(cfg *Config) error {
	//TODO implement me
	panic("implement me")
}
