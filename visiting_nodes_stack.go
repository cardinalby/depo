package depo

import "github.com/cardinalby/depo/internal/strs"

// visitingNodesStack is used during lcNodes building where we don't have information about node's frames
type visitingNodesStack []depNode

func (s *visitingNodesStack) Push(node depNode) {
	*s = append(*s, node)
}

func (s *visitingNodesStack) Peek() depNode {
	return (*s)[len(*s)-1]
}

func (s *visitingNodesStack) Pop() {
	*s = (*s)[:len(*s)-1]
}

func (s *visitingNodesStack) String() string {
	var lines strs.Lines
	for _, node := range *s {
		lines.Push(lnFrameNamePrefixIn + node.GetDepInfo().String())
	}
	return lines.JoinReverse()
}

func (s *visitingNodesStack) CloneSkipLowest() visitingNodesStack {
	if len(*s) <= 1 {
		return nil
	}
	clone := make(visitingNodesStack, len(*s)-1)
	copy(clone, (*s)[1:])
	return clone
}
