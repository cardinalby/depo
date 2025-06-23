package dep

import (
	"reflect"
	"strconv"
	"strings"

	"github.com/cardinalby/depo/internal/runtm"
	"github.com/cardinalby/depo/internal/tests"
)

// Id is an unique identifier for a dependency/component definition.
type Id uint64

func (sn Id) Empty() bool {
	return sn == 0
}

func (sn Id) String() string {
	return strconv.FormatUint(uint64(sn), 10)
}

type Info struct {
	Ctx

	// valueType is type of value provided by a component
	valueType reflect.Type

	// "Provide" or "ProvideErr" - which constructor was used to define the component
	defCtorName string
}

func NewDepInfo(
	depCtx Ctx,
	valueType reflect.Type,
	defCtorName string,
) Info {
	return Info{
		Ctx:         depCtx,
		valueType:   valueType,
		defCtorName: defCtorName,
	}
}

func (d Info) Empty() bool {
	return d.Ctx.Empty()
}

func (d Info) String() string {
	if d.Empty() {
		return "<empty>"
	}
	var sb strings.Builder
	if d.defCtorName != "" {
		sb.WriteString(d.defCtorName)
		sb.WriteString("(")
		sb.WriteString(d.Id.String())
		sb.WriteString(") ")
	} else {
		sb.WriteString("[")
		sb.WriteString(d.Id.String())
		sb.WriteString("] ")
	}
	if d.valueType != nil {
		sb.WriteString(d.valueType.String())
	} else if tests.IsTestingBuild {
		panic("Info.valueType is nil, this should not happen")
	}
	sb.WriteString(" @ ")
	sb.WriteString(d.Ctx.RegAt.File())
	sb.WriteString(":")
	sb.WriteString(strconv.Itoa(d.Ctx.RegAt.Line()))
	return sb.String()
}

// Ctx contains the unique identifier of a dependency and the context in which it was registered
type Ctx struct {
	Id    Id              // makes Ctx objects created at the same function:line unique
	RegAt runtm.CallerCtx // points to the function in the client code that defined the dependency
}

func (d Ctx) Empty() bool {
	return d.Id == 0
}

func (d Ctx) String() string {
	if d.Empty() {
		return "<empty>"
	}
	var sb strings.Builder
	sb.WriteString("[")
	sb.WriteString(d.Id.String())
	sb.WriteString("] ")
	sb.WriteString(d.RegAt.String())
	return sb.String()
}
