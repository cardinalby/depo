package depo

import (
	"strings"
	"unsafe"
)

type componentInfo struct {
	id    uint64
	value any
	tag   any
}

func (ci *componentInfo) ID() uint64 {
	return ci.id
}

func (ci *componentInfo) Value() any {
	return ci.value
}

func (ci *componentInfo) Tag() any {
	return ci.tag
}

type lcNodeOwnInfo struct {
	depNode depNode
	lcHook  *lifecycleHook
}

func (lcn lcNodeOwnInfo) ID() uintptr {
	return uintptr(unsafe.Pointer(lcn.lcHook))
}

func (lcn lcNodeOwnInfo) String() string {
	nameWithOrigin, _ := lcn.stringForLcPhase("")
	return nameWithOrigin
}

func (lcn lcNodeOwnInfo) stringForLcPhase(phase failedLifecyclePhase) (nameWithOrigin string, method string) {
	var sb strings.Builder
	if phase == "" {
		sb.WriteString(lcn.lcHook.String())
	} else {
		var name string
		name, method = lcn.lcHook.stringForPhase(phase)
		sb.WriteString(name)
	}
	sb.WriteString("\nIn ")
	sb.WriteString(lcn.depNode.GetDepInfo().String())
	return sb.String(), method
}

func (lcn lcNodeOwnInfo) ComponentInfo() ComponentInfo {
	depInfo := lcn.depNode.GetDepInfo()
	return &componentInfo{
		id:    uint64(depInfo.Id),
		value: lcn.depNode.GetProvidedValue(),
		tag:   depInfo.GetTag(),
	}
}

func (lcn lcNodeOwnInfo) Component() any {
	// if lcNode is created it means node is already registered with no errors
	return lcn.depNode.GetProvidedValue()
}

func (lcn lcNodeOwnInfo) ComponentID() uint64 {
	return uint64(lcn.depNode.GetDepInfo().Id)
}

func (lcn lcNodeOwnInfo) Tag() any {
	return lcn.lcHook.tag
}
