package main

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"code.google.com/p/go.net/websocket"
)

const listenAddr = "localhost:4000"

func main() {
	rand.Seed(time.Now().UnixNano())
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
var line = document.createElement("p"),
    log = document.getElementById('chatLog');
line.className = 'chatline';
line.appendChild(msgNode);
log.appendChild(line);
log.scrollTop = log.scrollHeight;
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
var msgNode = document.getElementById('msg'),
    msg = msgNode.value,
    node = document.createTextNode(msg);
addMsg(node);
ws.send(msg);
msgNode.value = '';
msgNode.focus();
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
.container {
margin: auto;
max-width: 50em;
}
h1 {
text-align: center;
}
#msg {
width: 90%;
}
</style>
</head>
<body>
<div class="container">
<h1>Welcome to Sock-Roulette</h1>
<div id="chatLog">
</div>
<form onsubmit="event.preventDefault(); chat()">
<input type="text" name="msg" id="msg">
<button type="submit" id="submit">send</button>
</form>
</div>
</body>
</html>
`))

type websock struct {
	io.Reader
	io.Writer
	done chan bool
}

func (s websock) Close() error {
	s.done <- true
	return nil
}

var chain = NewChain(2)

func wsHandler(ws *websocket.Conn) {
	r, w := io.Pipe()
	go func() {
		_, err := io.Copy(io.MultiWriter(w, chain), ws)
		w.CloseWithError(err)
	}()
	s := websock{r, ws, make(chan bool)}
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
	case <-time.After(5* time.Second):
		chat(Bot(), c)
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

// Bot returns an io.ReadWriteCloser that responds to
// each incoming write with a generated sentence.
func Bot() io.ReadWriteCloser {
	r, out := io.Pipe()
	return bot{r, out}
}

type bot struct {
	io.ReadCloser
	out io.Writer
}

func (b bot) Write(buf []byte) (int, error) {
	go b.speak()
	return len(buf), nil
}

func (b bot) speak() {
	time.Sleep(time.Second)
	msg := chain.Generate(10)
	b.out.Write([]byte(msg))
}


// The Markov chain code is largely copied from golang.com/doc/codewalk/markov/
// Copyright 2011 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Generating random text: a Markov chain algorithm

Based on the program presented in the "Design and Implementation"
chapter of The Practice of Programming (Kernighan and Pike,
Addison-Wesley 1999). See also Computer Recreations, Scientific
American 260, 122 - 125 (1989).

A Markov chain algorithm generates text by creating a statistical
model of potential textual suffixes for a given prefix. Consider this
text:

I am not a number! I am a free man!

Our Markov chain algorithm would arrange this text into this set of
prefixes and suffixes, or "chain": (This table assumes a prefix length
of two words.)

Prefix       Suffix

"" ""        I
"" I         am
I am         a
I am         not
a free       man!
am a         free
am not       a
a number!    I
number! I    am
not a        number!

To generate text using this table we select an initial prefix ("I am",
for example), choose one of the suffixes associated with that prefix
at random with probability determined by the input statistics ("a"),
and then create a new prefix by removing the first word from the
prefix and appending the suffix (making the new prefix is "am
a"). Repeat this process until we can't find any suffixes for the
current prefix or we exceed the word limit. (The word limit is
necessary as the chain table may contain cycles.)
*/

// Prefix is a Markov chain prefix of one or more words.
type Prefix []string

// String returns the Prefix as a string (for use as a map key).
func (p Prefix) String() string {
	return strings.Join(p, " ")
}

// Shift removes the first word from the Prefix and appends the given
// word.
func (p Prefix) Shift(word string) {
	copy(p, p[1:])
	p[len(p)-1] = word
}

// Chain contains a map ("chain") of prefixes to a list of suffixes.
// A prefix is a string of prefixLen words joined with spaces.
// A suffix is a single word. A prefix can have multiple suffixes.
type Chain struct {
	chain     map[string][]string
	prefixLen int
}

// NewChain returns a new Chain with prefixes of prefixLen words.
func NewChain(prefixLen int) *Chain {
	return &Chain{make(map[string][]string), prefixLen}
}

// Write parses the bytes into prefixes and suffixes that are stored
// in Chain.
// Note: added based on http://talks.golang.org/2012/chat.slide#38
func (c *Chain) Write(b []byte) (int, error) {
	p := make(Prefix, c.prefixLen)
	s := strings.Split(string(b[:]), " ")
	for _, word := range s {
		key := p.String()
		c.chain[key] = append(c.chain[key], word)
		p.Shift(word)
	}
	return len(b), nil
}


// Generate returns a string of at most n words generated from Chain.
func (c *Chain) Generate(n int) string {
	p := make(Prefix, c.prefixLen)
	var words []string
	for i := 0; i < n; i++ {
		choices := c.chain[p.String()]
		if len(choices) == 0 {
			break
		}
		next := choices[rand.Intn(len(choices))]
		words = append(words, next)
		p.Shift(next)
	}
	return strings.Join(words, " ")
}
