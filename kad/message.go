package kad

type Message interface{}

type Request[K Key[K], A Address[A]] interface {
	Message

	// Target returns the target key and true, or false if no target key has been specfied.
	Target() K
	EmptyResponse() Response[K, A]
}

type Response[K Key[K], A Address[A]] interface {
	Message

	CloserNodes() []NodeInfo[K, A]
}
