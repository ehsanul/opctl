package model

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
)

type ReadSeekCloser interface {
	io.ReadCloser
	io.Seeker
}

// DataHandle is a provider agnostic interface for interacting w/ data
// @TODO: merge Value and DataHandle
//counterfeiter:generate -o fakes/dataHandle.go . DataHandle
type DataHandle interface {
	// ListDescendants lists descendant of the data node pointed to by the current handle
	ListDescendants(
		ctx context.Context,
		eventChannel chan Event,
		callID string,
	) (
		[]*DirEntry,
		error,
	)

	// GetContent gets data from the current handle
	GetContent(
		ctx context.Context,
		eventChannel chan Event,
		callID string,
		contentPath string,
	) (
		ReadSeekCloser,
		error,
	)

	// Path returns the local path of the data; may or may not be same as Ref
	// returns nil if data doesn't exist locally
	Path() *string

	// Ref returns a ref to the data; may or may not be same as Path
	Ref() string
}

// DirEntry represents an entry in a directory (a sub directory or file)
type DirEntry struct {
	Path string
	Size int64
	Mode os.FileMode
}

// Value represents a typed value
type Value struct {
	Array   *[]interface{}          `json:"array,omitempty"`
	Boolean *bool                   `json:"boolean,omitempty"`
	Dir     *string                 `json:"dir,omitempty"`
	File    *string                 `json:"file,omitempty"`
	Number  *float64                `json:"number,omitempty"`
	Object  *map[string]interface{} `json:"object,omitempty"`
	Socket  *string                 `json:"socket,omitempty"`
	String  *string                 `json:"string,omitempty"`
}

// // ValueNode interfaces with a value of any type
// type ValueNode interface {
// 	// ID returns a globally unique id for this value
// 	ID() string
// }

// // FileValue interfaces with a file typed value
// type FileValue interface {
// 	GetReadSeekCloser() (ReadSeekCloser, error)

// 	ValueNode
// }

// // DirValue interfaces with a directory typed value
// type DirValue interface {
// 	GetDescendant(
// 		ref string,
// 	) (
// 		Value,
// 		error,
// 	)

// 	ValueNode
// }

// Unbox unboxes a Value into a native go type
func (value Value) Unbox() (interface{}, error) {
	if value.Array != nil {
		nativeArray := []interface{}{}
		for itemKey, itemValue := range *value.Array {
			switch typedItemValue := itemValue.(type) {
			case Value:
				nativeItem, err := typedItemValue.Unbox()
				if err != nil {
					return nil, errors.Wrap(err, fmt.Sprintf("unable to unbox item '%v' from array", itemKey))
				}

				nativeArray = append(nativeArray, nativeItem)
			default:
				nativeArray = append(nativeArray, itemValue)
			}
		}
		return nativeArray, nil
	} else if value.Boolean != nil {
		return *value.Boolean, nil
	} else if value.Dir != nil {
		return *value.Dir, nil
	} else if value.File != nil {
		return *value.File, nil
	} else if value.Number != nil {
		return *value.Number, nil
	} else if value.Object != nil {
		nativeObject := map[string]interface{}{}
		for propKey, propValue := range *value.Object {
			switch typedPropValue := propValue.(type) {
			case Value:
				var err error
				if nativeObject[propKey], err = typedPropValue.Unbox(); err != nil {
					return nil, errors.Wrap(err, fmt.Sprintf("unable to unbox property '%v' from object", propKey))
				}
			default:
				nativeObject[propKey] = propValue
			}
		}
		return nativeObject, nil
	} else if value.Socket != nil {
		return *value.Socket, nil
	} else if value.String != nil {
		return *value.String, nil
	}
	return nil, fmt.Errorf("unable to unbox value '%+v'", value)
}

func (value Value) format() (interface{}, error) {
	if nil != value.Array {
		formattedArray := []interface{}{}
		for itemKey, itemValue := range *value.Array {
			switch typedItemValue := itemValue.(type) {
			case Value:
				formattedValue, err := typedItemValue.format()
				if nil != err {
					return "", errors.Wrap(err, fmt.Sprintf("unable to stringify item '%v' from array", itemKey))
				}

				formattedArray = append(formattedArray, formattedValue)
			default:
				formattedArray = append(formattedArray, itemValue)
			}
		}
		return formattedArray, nil
	} else if nil != value.Boolean {
		return strconv.FormatBool(*value.Boolean), nil
	} else if nil != value.Dir {
		return *value.Dir, nil
	} else if nil != value.File {
		return *value.File, nil
	} else if nil != value.Number {
		return fmt.Sprintf("%f", *value.Number), nil
	} else if nil != value.Object {
		formattedMap := map[string]interface{}{}
		for propKey, propValue := range *value.Object {
			switch typedPropValue := propValue.(type) {
			case Value:
				var err error
				if formattedMap[propKey], err = typedPropValue.format(); nil != err {
					return "", errors.Wrap(err, fmt.Sprintf("unable to stringify property '%v' from object", propKey))
				}
			default:
				formattedMap[propKey] = propValue
			}
		}
		return formattedMap, nil
	} else if nil != value.Socket {
		return *value.Socket, nil
	} else if nil != value.String {
		return *value.String, nil
	}
	return "", fmt.Errorf("unable to stringify value '%+v'", value)
}

func FormatValueMap(valueMap map[string]*Value) (string, error) {
	formattedValues := map[string]interface{}{}
	for name, value := range valueMap {
		description, err := value.format()
		if err != nil {
			return "", err
		}
		formattedValues[name] = description
	}
	bytes, err := yaml.Marshal(formattedValues)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
