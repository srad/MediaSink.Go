package utils

type Observer interface {
	Update(string, string)
	GetChannelName() string
}

type Subject interface {
	Register(Observer)
	Deregister(Observer)
	NotifyAll()
}
