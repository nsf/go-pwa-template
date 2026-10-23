// This package just shows where you would put your Go code. When it comes to mixed projects (Go and other things).
// It's best to keep Go code away from the root directory and its subdirectories. So everything goes into "internal".
package idlib

import (
	"github.com/matoous/go-nanoid/v2"
)

const idAlphabet = `0123456789abcdefghijklmnopqrstuvwxyz`

func Generate() string {
	return gonanoid.MustGenerate(idAlphabet, 8)
}
