package gofeedx

// Unified extensions builder API

// ExtOption represents a unified option that can contribute extension nodes
// at feed/channel scope and/or item/entry scope.
type ExtOption interface {
	feedNodes() []ExtensionNode
	itemNodes() []ExtensionNode
}

// extNodesOption is an internal implementation of ExtOption.
type extNodesOption struct {
	feed []ExtensionNode
	item []ExtensionNode
}

func (o extNodesOption) feedNodes() []ExtensionNode { return o.feed }
func (o extNodesOption) itemNodes() []ExtensionNode { return o.item }

// newFeedNodes constructs an ExtOption that injects nodes at feed/channel scope.
func newFeedNodes(nodes ...ExtensionNode) ExtOption { return extNodesOption{feed: nodes} }

// newItemNodes constructs an ExtOption that injects nodes at item/entry scope.
func newItemNodes(nodes ...ExtensionNode) ExtOption { return extNodesOption{item: nodes} }

// ApplyExtensions applies unified extension options at feed/channel scope.
func (f *Feed) ApplyExtensions(opts ...ExtOption) {
	if f == nil || len(opts) == 0 {
		return
	}
	for _, o := range opts {
		if o == nil {
			continue
		}
		f.Extensions = append(f.Extensions, o.feedNodes()...)
	}
}

// ApplyExtensions applies unified extension options at item/entry scope.
func (it *Item) ApplyExtensions(opts ...ExtOption) {
	if it == nil || len(opts) == 0 {
		return
	}
	for _, o := range opts {
		if o == nil {
			continue
		}
		it.Extensions = append(it.Extensions, o.itemNodes()...)
	}
}


