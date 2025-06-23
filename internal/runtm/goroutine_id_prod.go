//go:build !depo.testing

package runtm

func GetGoroutineID() string {
	panic("is not reliable to be used in prod build")
}
