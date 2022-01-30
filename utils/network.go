package utils

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
)

const (
	ConnHost = "localhost"
	ConnPort = "3333"
	ConnType = "tcp"
)

func TCPServer() {
	url := ConnHost + ":" + ConnPort

	// Listen for incoming connections.
	l, err := net.Listen(ConnType, url)
	if err != nil {
		log.Fatalln(err.Error())
	}
	// Close the listener when the application closes.
	defer l.Close()
	fmt.Println("Listening on " + url)

	for {
		// Listen for an incoming connection.
		conn, err := l.Accept()
		if err != nil {
			log.Fatalln("Error accepting: ", err.Error())
		}
		go handleTCPConnection(conn)
	}
}

// It is expected that all jobs are processed linearly
// TODO: Replace with named pipe when using ffmpeg -progress ..pipe..
func handleTCPConnection(conn net.Conn) {
	fmt.Printf("Serving %s\n", conn.RemoteAddr().String())
	for {
		netData, err := bufio.NewReader(conn).ReadString('\n')
		if err == io.EOF {
			log.Println("TCP EOF")
			break
		}
		if err != nil {
			log.Println(err)
			return
		}

		msg := strings.TrimSpace(string(netData))
		log.Println(msg)
		//if workers.ActiveJobId != 0 {
		//	models.UpdateJobInfo(workers.ActiveJobId, msg)
		//}
	}
	log.Println("End TCP connection")
	conn.Close()
}
