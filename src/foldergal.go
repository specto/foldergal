package main


type Node struct {
	Path string
}

type Video struct {
	Node
}

type Image struct {
	Node
}

type Folder struct {
	Node
}

func (v *Video) GetChildren() []NodeItem {
	var r []NodeItem = nil
	return r
}

func (i *Image) GetChildren() []NodeItem {
	var r []NodeItem = nil
	return r
}

func (v *Folder) GetChildren() []NodeItem {
	var r []NodeItem
	return r
}

func (n *Node) GetPath() string {
	return n.Path
}


type NodeItem interface {
	GetChildren() []NodeItem
	GetPath() string
}