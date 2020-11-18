// Package light
package light

type Service interface {
	HeaderByHeight()
	HeaderByHash()
}
