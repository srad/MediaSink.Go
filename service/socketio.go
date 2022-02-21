package service

import (
	"fmt"

	socketio "github.com/googollee/go-socket.io"
)

var SocketioServer *socketio.Server

func SetupSocket() *socketio.Server {
	SocketioServer = socketio.NewServer(nil)

	SocketioServer.OnConnect("/", func(s socketio.Conn) error {
		s.SetContext("")
		fmt.Println("connected:", s.ID())
		return nil
	})

	SocketioServer.OnEvent("/", "notice", func(s socketio.Conn, msg string) {
		fmt.Println("notice:", msg)
		s.Emit("reply", "have "+msg)
	})

	SocketioServer.OnEvent("/chat", "msg", func(s socketio.Conn, msg string) string {
		s.SetContext(msg)
		return "recv " + msg
	})

	SocketioServer.OnEvent("/", "bye", func(s socketio.Conn) string {
		last := s.Context().(string)
		s.Emit("bye", last)
		s.Close()
		return last
	})

	SocketioServer.OnError("/", func(s socketio.Conn, e error) {
		fmt.Println("meet error:", e)
	})

	SocketioServer.OnDisconnect("/", func(s socketio.Conn, reason string) {
		fmt.Println("closed", reason)
	})

	go SocketioServer.Serve()
	//defer server.Close()

	return SocketioServer
}
