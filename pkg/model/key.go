package model

import "fmt"

func WrapperKey(height uint64, i int) []byte {
	return []byte(fmt.Sprintf("wrapper-%d-%d", height, i))
}

func IBTPKey(id string) []byte {
	return []byte(fmt.Sprintf("ibtp-%s", id))
}
