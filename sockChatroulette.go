package main

import "fmt"
import "io"
import "net"
import "log"

const listenAddr = "localhost:4000"

func main() {
	l, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatal(err)
	}
	for {
		c, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go match(c)
	}
}

var partner = make(chan io.ReadWriteCloser)

func match(c io.ReadWriteCloser) {
	fmt.Fprint(c, "Waiting for a chat partner...")
	select {
	case partner <- c:
		// chat handled by other goroutine
	case p := <-partner:
		chat(p, c)
	}
}

func chat(a, b io.ReadWriteCloser) {
	fmt.Fprintln(a, "You are now connected to a chat partner")
	fmt.Fprintln(b, "You are now connected to a chat partner")
	errc := make(chan error, 1)
	go cp(a, b, errc)
	go cp(b, a, errc)
	if err := <-errc; err != nil {
		log.Println(err)
	}
	a.Close()
	b.Close()
}

func cp(w io.Writer, r io.Reader, errc chan<- error) {
	_, err := io.Copy(w, r)
	errc <- err
}
