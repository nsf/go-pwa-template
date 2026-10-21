package idlib

import (
	"github.com/matoous/go-nanoid/v2"
)

const idAlphabet = `0123456789abcdefghijklmnopqrstuvwxyz`

func Generate() string {
	return gonanoid.MustGenerate(idAlphabet, 8)
}
