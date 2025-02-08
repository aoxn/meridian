package unstructed

import (
	"encoding/json"
	"fmt"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
	"regexp"
	"strings"
)

func ToUnstructured(tpl string) (*Unstructured, error) {
	m := make(map[string]interface{})
	err := json.Unmarshal([]byte(tpl), &m)
	if err != nil {
		return nil, fmt.Errorf("not json: %s", err.Error())
	}
	return NewUnstructured(m), nil
}

const match = `([\w]{0,})\[([\w-=+/\.]{0,})\s{0,}[:]{0,}\s{0,}([\w-=+/\.]{0,})\]`

// Unstructured wrapper of unstructured.Unstructured
// path: spec.containers[key:value].name
// value: apiserver
//
// path: spec.containers[key:value].command[key]
// value: key
type Unstructured struct {
	unstructured.Unstructured
}

func NewUnstructured(m map[string]interface{}) *Unstructured {
	return &Unstructured{Unstructured: unstructured.Unstructured{Object: m}}
}

func (u *Unstructured) ToJson() (string, error) {
	data, err := json.Marshal(u.Object)
	if err != nil {
		return "", fmt.Errorf("marshal json: %s", err.Error())
	}
	return string(data), err
}

func (u *Unstructured) GetValue(ppath string) (interface{}, bool) {
	val, err := u.WalkPath(ppath, nil, false)
	if err != nil {
		klog.Errorf("GetValue: error %s", err.Error())
		return nil, false
	}
	return val, true
}

func (u *Unstructured) SetValue(ppath string, value interface{}) error {
	_, err := u.WalkPath(ppath, value, true)
	return err
}

func (u *Unstructured) WalkPath(ppath string, value interface{}, set bool) (interface{}, error) {
	path, err := loadPath(ppath)
	if err != nil {
		fmt.Printf("Error: WalkPath, loadPath(), %v\n", err)
		return nil, err
	}
	var val interface{} = u.Object
	lfield := len(path.field)
	for i, field := range path.field {
		switch ValueType(&field) {
		case VALUE_TYPE_MAP:
			// map value
			m, ok := val.(map[string]interface{})
			if ok {
				if i == lfield-1 && set {
					// last subpath
					m[field.name] = nvalue(value)
				}
				val, ok = m[field.name]
				if !ok {
					m[field.name] = make(map[string]interface{})
					val = m[field.name]
					klog.Warningf("filed not found: default add %s", field.name)
				}
				continue
			}
			return nil, fmt.Errorf("field not a map. %v", val)
		case VALUE_TYPE_SIMPLE_SLICE:
			// SimpleArrayValue
			if i != lfield-1 {
				return nil, fmt.Errorf("midle path should not be a simple slice")
			}
			// map value
			m, ok := val.(map[string]interface{})
			if !ok {
				return nil, NOT_FOUND
			}
			val, ok = m[field.name]
			if !ok {
				return nil, NOT_FOUND
			}
			// SimpleArrayValue
			s, ok := val.([]interface{})
			if ok {
				newv := make([]interface{}, 0)
				var found interface{}
				for _, v := range s {
					if x, ok := v.(string); ok {
						// match prefix
						if strings.HasPrefix(x, field.key) {
							found = v
							if !set {
								// read operation. return directly.
								return v, nil
							}
							newv = append(newv, nvalue(value))
						} else {
							newv = append(newv, v)
						}
						continue
					}
					return nil, fmt.Errorf("not string slice %v", v)
				}
				if set && i == lfield-1 {
					if found == nil {
						newv = append(newv, nvalue(value))
					}
					m[field.name] = newv
					return value, nil
				}
				if found != nil {
					// return directly or continue
					val = found
					continue
				}
				return nil, NOT_FOUND
			}
			return nil, fmt.Errorf("not slice. %s", val)
		case VALUE_TYPE_COMPLEX_SLICE:
			// map value
			m, ok := val.(map[string]interface{})
			if !ok {
				return nil, NOT_FOUND
			}
			val, ok = m[field.name]
			if !ok {
				return nil, NOT_FOUND
			}
			s, ok := val.([]interface{})
			if ok {
				var found interface{}
				newv := make([]interface{}, 0)
				for _, v := range s {
					if x, ok := v.(map[string]interface{}); ok {
						if x[field.key] == field.value {
							found = v
							if (!set) && i != lfield-1 {
								// continue with read.
								break
							}
							newv = append(newv, nvalue(value))
						} else {
							newv = append(newv, v)
						}
						continue
					}
					return nil, fmt.Errorf("not string slice %v", v)
				}
				if set && i == lfield-1 {
					if found == nil {
						newv = append(newv, nvalue(value))
					}
					m[field.name] = newv
					return val, nil
				}
				if found != nil {
					val = found
					continue
				}
				return nil, NOT_FOUND
			}
			return nil, fmt.Errorf("field not an complex map interface. %s", val)
		}
	}
	return val, nil
}

func nvalue(v interface{}) interface{} {
	cont, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("marshal: %s", err.Error()))
	}
	var mvalue interface{}
	err = json.Unmarshal(cont, &mvalue)
	if err != nil {
		panic(fmt.Sprintf("unmarshal: %s", err.Error()))
	}
	return mvalue
}

type path struct {
	namedKey string
	field    []field
}

type field struct {
	// field eg. containers[name:kubernetes-kdm]
	field string
	// name eg. containers
	name string
	// key eg. name
	key string

	// value eg. kubernetes-kdm
	value string
}

func loadPath(ppath string) (path, error) {
	reg, err := regexp.Compile(match)
	if err != nil {
		return path{}, err
	}
	idxs := []field{}
	for _, subp := range strings.Split(ppath, ".") {
		bt := reg.FindStringSubmatch(subp)
		if len(bt) <= 0 {
			idxs = append(idxs, field{name: subp, field: subp})
			continue
		}

		idx := field{
			field: subp,
			name:  string(bt[1]),
			key:   string(bt[2]),
			value: string(bt[3]),
		}
		idxs = append(idxs, idx)
	}
	return path{
		namedKey: ppath,
		field:    idxs}, nil
}

const (
	VALUE_TYPE_MAP           = "MAP_VALUE"
	VALUE_TYPE_SIMPLE_SLICE  = "SIMPLE_SLICE"
	VALUE_TYPE_COMPLEX_SLICE = "COMPLEX_SLICE"
)

func ValueType(field *field) string {
	if field.key == "" && field.value == "" {
		return VALUE_TYPE_MAP
	}
	if field.key != "" && field.value == "" {
		return VALUE_TYPE_SIMPLE_SLICE
	}
	if field.key != "" && field.value != "" {
		return VALUE_TYPE_COMPLEX_SLICE
	}
	return VALUE_TYPE_MAP
}

type NotFound struct {
	Key string
}

func (f NotFound) Error() string {
	return fmt.Sprintf("NotFound %s", f.Key)
}

var NOT_FOUND = NotFound{}
