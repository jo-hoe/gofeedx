package gofeedx

// AppendFeedExtensions appends extension nodes to the channel/feed scope.
func AppendFeedExtensions(f *Feed, nodes ...ExtensionNode) {
	if f == nil || len(nodes) == 0 {
		return
	}
	f.Extensions = append(f.Extensions, nodes...)
}

// AppendItemExtensions appends extension nodes to an item/entry scope.
func AppendItemExtensions(it *Item, nodes ...ExtensionNode) {
	if it == nil || len(nodes) == 0 {
		return
	}
	it.Extensions = append(it.Extensions, nodes...)
}