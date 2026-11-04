package terminal

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestRunnerOutputCreditDoesNotBlockKeyboardOrLeakIntoPTY(t *testing.T) {
	id := "flow_" + generateSessionID()
	path := SocketPath(id)
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	defer os.Remove(path)
	input := make(chan TerminalMessage, 8)
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		c, err := listener.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		go func() {
			scanner := bufio.NewScanner(c)
			for scanner.Scan() {
				var msg TerminalMessage
				if json.Unmarshal(scanner.Bytes(), &msg) == nil {
					input <- msg
				}
			}
		}()
		c.Write(bytes.Repeat([]byte("a"), 4096))
		c.Write(bytes.Repeat([]byte("b"), 4096))
		<-stop
	}()
	h := NewHandler(HandlerConfig{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { h.proxySocketToWebSocket(w, r, id, nil) }))
	defer server.Close()
	client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"?output_ack=1", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	_, hello, err := client.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	var handshake struct {
		OutputAck bool `json:"output_ack"`
	}
	json.Unmarshal(hello, &handshake)
	if !handshake.OutputAck {
		t.Fatal("output credit not negotiated")
	}
	_, first, err := client.ReadMessage()
	if err != nil || !bytes.Equal(first, bytes.Repeat([]byte("a"), 4096)) {
		t.Fatal("first frame changed", err)
	}
	second := make(chan []byte, 1)
	go func() { _, data, _ := client.ReadMessage(); second <- data }()
	client.WriteJSON(TerminalMessage{Type: "output_ack", Sequence: 2})
	client.WriteJSON(TerminalMessage{Type: "stdin", Data: "typing while rendering"})
	select {
	case msg := <-input:
		if msg.Type != "resize" {
			t.Fatal("expected initial resize")
		}
	case <-time.After(time.Second):
		t.Fatal("resize missing")
	}
	select {
	case msg := <-input:
		if msg.Type != "stdin" || msg.Data != "typing while rendering" {
			t.Fatal("paint credit leaked into PTY", msg.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("keyboard blocked behind output credit")
	}
	select {
	case <-second:
		t.Fatal("future credit released output")
	case <-time.After(20 * time.Millisecond):
	}
	client.WriteJSON(TerminalMessage{Type: "output_ack", Sequence: 1})
	select {
	case data := <-second:
		if !bytes.Equal(data, bytes.Repeat([]byte("b"), 4096)) {
			t.Fatal("second frame changed")
		}
	case <-time.After(time.Second):
		t.Fatal("parsed frame did not release output")
	}
	client.WriteJSON(TerminalMessage{Type: "output_ack", Sequence: 2})
}
