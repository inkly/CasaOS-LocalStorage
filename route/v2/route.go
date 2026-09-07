package v2

import (
	"github.com/inkly/CasaOS-LocalStorage/codegen"
)

type LocalStorage struct{}

func NewLocalStorage() codegen.ServerInterface {
	return &LocalStorage{}
}
