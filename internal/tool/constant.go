package tool

func DefaultImage(id string) string {
	if id != "" {
		return id
	}
	return ""
}

const (
	BootTypeLocal   = "local"
	BootTypeRecover = "recover"
	BootTypeCoord   = "coordinator"
	BootTypeJoin    = "manager"
)
