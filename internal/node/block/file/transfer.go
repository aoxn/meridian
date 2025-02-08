package file

import "path/filepath"

/*
	$base/$beta/public/wdrip/$pkgName-$version.tar
*/

type Tar interface {
	Tar() error
	Location() string
}

type Transfer struct {
	Bucket  string
	Regions []string

	From  *Path
	To    *Path
	Base  string
	Cache string

	Tar      Tar
	Upload   func(t *Transfer, from, to string) error
	Download func(t *Transfer, from, to string) error
}

func (f *Transfer) LocalTarURI() string {
	return filepath.Join(f.Cache, f.From.Name())
}

func (f *Transfer) LocalURL() string {
	return filepath.Join(f.Cache, f.From.URL())
}

func (f *Transfer) RemotePath() string {
	return filepath.Join(f.Base, f.From.URL())
}
