package gofeedx

// Extensions are added via the builders using:
//  - FeedBuilder.WithExtensions(nodes ...ExtensionNode)
//  - ItemBuilder.WithExtensions(nodes ...ExtensionNode)
//
// ExtensionNode is defined in xmlnode.go and safely marshals XML for RSS/Atom/PSP,
// and is flattened for JSON Feed in json.go. No additional extension helper API is provided
// to keep a single, consistent way of adding target-specific elements.