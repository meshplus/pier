package model

import "fmt"

func WrapperKey(height uint64) []byte {
	return []byte(fmt.Sprintf("wrapper-%d", height))
}

func IBTPKey(id string) []byte {
	return []byte(fmt.Sprintf("ibtp-%s", id))
}
