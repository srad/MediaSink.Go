package patterns

type Event[T any] struct {
	Data T      `json:"data"`
	Name string `json:"name"`
}

type Dispatcher[T any] struct {
	listeners []func(Event[T])
}

func (dispatcher *Dispatcher[T]) Subscribe(f func(Event[T])) {
	dispatcher.listeners = append(dispatcher.listeners, f)
}

func (dispatcher *Dispatcher[T]) Notify(name string, data T) {
	msg := Event[T]{Name: name, Data: data}
	for _, f := range dispatcher.listeners {
		f(msg)
	}
}

func NewMessage[T any](name string, data T) Event[T] {
	return Event[T]{Name: name, Data: data}
}
