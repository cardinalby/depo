package depo

import "fmt"

// UsePtrLateInit is a helper for UseLateInit that allows you to provide a pointer to a value
// that will be initialized later. Other dependencies will receive a pointer to an empty object first, then a new
// *T object will be created by `providePtr` function and its value will be copied into the empty object.
// It allows you to keep the familiar NewSomething() *T constructors pattern if your objects are copyable.
func UsePtrLateInit[T any](providePtr func() *T) *T {
	if providePtr == nil {
		panic(fmt.Errorf("%w providePtr", errNilValue))
	}
	empty := new(T)
	UseLateInit(func() {
		initialized := providePtr()
		if initialized == nil {
			panic(fmt.Errorf("%w providePtr result", errNilValue))
		}
		*empty = *initialized
	})
	return empty
}

// UsePtrLateInitE is a helper for UseLateInitE that allows you to provide a pointer to a value
// that will be initialized later. Other dependencies will receive a pointer to an empty object first, then a new
// *T object will be created by `providePtr` function and its value will be copied into the empty object.
// It allows you to keep the familiar NewSomething() *T constructors pattern if your objects are copyable.
func UsePtrLateInitE[T any](providePtr func() (*T, error)) *T {
	if providePtr == nil {
		panic(fmt.Errorf("%w providePtr", errNilValue))
	}
	empty := new(T)
	UseLateInitE(func() error {
		initialized, err := providePtr()
		if err != nil {
			return err
		}
		if initialized == nil {
			return fmt.Errorf("%w providePtr result", errNilValue)
		}
		*empty = *initialized
		return nil
	})
	return empty
}
