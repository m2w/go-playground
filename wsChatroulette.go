package main

// go to golang.org/doc/codewalk/markov

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"html/template"

	"code.google.com/p/go.net/websocket"
)

const listenAddr = "localhost:4000"

func main() {
	http.HandleFunc("/", rootHandler)
	http.Handle("/ws", websocket.Handler(wsHandler))
	err := http.ListenAndServe(listenAddr, nil)
	if err != nil {
		log.Fatal(err)
	}
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	rootTemplate.Execute(w, listenAddr)
}

var rootTemplate = template.Must(template.New("root").Parse(`
<html>
<head>
<meta charset="utf-8" />
<script>
function addMsg(msgNode) {
var line = document.createElement("p");
line.className = 'chatline';
line.appendChild(msgNode);
document.getElementById('chatLog').appendChild(line);
}


var ws = new WebSocket("ws://{{.}}/ws");
ws.onmessage = function(msgEvent) {
var em = document.createElement("em");
em.appendChild(document.createTextNode(msgEvent.data));
addMsg(em);
};
ws.onclose = function(closeEvent) {
var em = document.createElement("em");

em.appendChild(document.createTextNode("Your chat partner has disconnected"));
addMsg(em);

document.getElementById('msg').setAttribute('disabled');
document.getElementById('submit').setAttribute('disabled');
};
function chat() {
var msg = document.getElementById('msg').value,
    node = document.createTextNode(msg);
addMsg(node);
ws.send(msg);
return false;
}
</script>
<style>
#chatLog {
overflow-y:auto;
height: 15em;
}

p.chatline {
padding: 0;
margin: 0;
}
</style>
</head>
<body>
<h1>Welcome to Sock-Roulette</h1>
<div id="chatLog">

</div>
<form onsubmit="event.preventDefault(); chat()">
<input type="text" name="msg" id="msg">
<button type="submit" id="submit">send</button>
</form>
</body>
</html>
`))

type websock struct {
	io.ReadWriter
	done chan bool
}

func (s websock) Close() error {
	s.done <- true
	return nil
}

func wsHandler(ws *websocket.Conn) {
	s := websock{ws, make(chan bool)}
	go match(s)
	<-s.done
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
