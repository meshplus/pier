package model

import "fmt"

func IBTPKey(id string) []byte {
	return []byte(fmt.Sprintf("ibtp-%s", id))
}
