package main

import (
	"bufio"
	"log"
	"net"
)

type ChatRoom struct {
	users                 map[string]*ChatUser
	joins                 chan *ChatUser
	incoming, disconnects chan string
}

func NewChatRoom() *ChatRoom {
	return &ChatRoom{
		users:       make(map[string]*ChatUser),
		joins:       make(chan *ChatUser),
		incoming:    make(chan string),
		disconnects: make(chan string),
	}
}

func (cr *ChatRoom) ListenForMessages() {
	go func() {
		for {
			select {
			case username := <-cr.disconnects:
				if cr.users[username] != nil {
					cr.users[username].Close()
					delete(cr.users, username)
					cr.Broadcast("*** " + username + " has disconnected")
				}

			case msg := <-cr.incoming:
				cr.Broadcast(msg)

			case user := <-cr.joins:
				cr.users[user.username] = user
				cr.Broadcast("*** " + user.username + " has joined")
			}
		}
	}()
}

func (cr *ChatRoom) Logout(username string) {
	cr.disconnects <- username
}

func (cr *ChatRoom) Join(conn net.Conn) {
	user := NewChatUser(conn)
	err := user.Login(cr)
	if err != nil {
		log.Fatal("Error Logging in", err)
	}
	cr.joins <- user
}

func (cr *ChatRoom) Broadcast(msg string) {
	for _, user := range cr.users {
		user.Send(msg)
	}
}

type ChatUser struct {
	conn       net.Conn
	disconnect bool
	username   string
	outgoing   chan string
	reader     *bufio.Reader
	writer     *bufio.Writer
}

func NewChatUser(conn net.Conn) *ChatUser {
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	return &ChatUser{
		conn:       conn,
		disconnect: false,
		outgoing:   make(chan string),
		reader:     reader,
		writer:     writer,
	}
}

func (cu *ChatUser) ReadIncomingMessages(chatroom *ChatRoom) {
	go func() {
		for {
			msg, err := cu.ReadLine()
			if cu.disconnect {
				break
			}
			if err != nil {
				chatroom.Logout(cu.username)
				break
			}
			if msg != "" {
				chatroom.incoming <- "[" + cu.username + "]: " + msg + "\n"
			}
		}
	}()
}

func (cu *ChatUser) WriteOutgoingMessages(chatroom *ChatRoom) {
	for {
		select {
		case msg := <-cu.outgoing:
			cu.WriteString(msg)
		}
	}
}

func (cu *ChatUser) Login(chatroom *ChatRoom) error {
	cu.WriteString("Welcome to my Chat Server!\n")
	cu.WriteString("Please enter your username:\n")
	username, err := cu.ReadLine()

	if err != nil {
		return err
	}

	cu.username = username
	cu.WriteString("Welcome, " + cu.username + "\n")
	go cu.WriteOutgoingMessages(chatroom)
	go cu.ReadIncomingMessages(chatroom)
	return nil
}

func (cu *ChatUser) ReadLine() (string, error) {
	bytes, _, err := cu.reader.ReadLine()
	str := string(bytes)
	return str, err
}

func (cu *ChatUser) WriteString(msg string) error {
	cu.writer.WriteString(msg)
	cu.writer.Flush()
	return nil
}

func (cu *ChatUser) Send(msg string) {
	cu.outgoing <- msg
}

func (cu *ChatUser) Close() {
	cu.disconnect = true
	cu.conn.Close()
}

func main() {
	log.Println("Chat server starting!")
	listener, err := net.Listen("tcp", ":6677")
	if err != nil {
		log.Fatal("Connection Error", err)
		return
	}

	chatroom := NewChatRoom()
	chatroom.ListenForMessages()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal("Error Accepting Connection", err)
		}
		log.Println(conn.RemoteAddr())
		go chatroom.Join(conn)
	}

}
