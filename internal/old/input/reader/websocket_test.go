package reader

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/benthosdev/benthos/v4/internal/component"
	"github.com/benthosdev/benthos/v4/internal/component/metrics"
	"github.com/benthosdev/benthos/v4/internal/log"
	"github.com/benthosdev/benthos/v4/internal/message"
)

func TestWebsocketBasic(t *testing.T) {
	expMsgs := []string{
		"foo",
		"bar",
		"baz",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}

		var ws *websocket.Conn
		var err error
		if ws, err = upgrader.Upgrade(w, r, nil); err != nil {
			return
		}

		defer ws.Close()

		for _, msg := range expMsgs {
			if err = ws.WriteMessage(websocket.BinaryMessage, []byte(msg)); err != nil {
				t.Error(err)
			}
		}
	}))

	conf := NewWebsocketConfig()
	if wsURL, err := url.Parse(server.URL); err != nil {
		t.Fatal(err)
	} else {
		wsURL.Scheme = "ws"
		conf.URL = wsURL.String()
	}

	m, err := NewWebsocket(conf, log.Noop(), metrics.Noop())
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	if err = m.ConnectWithContext(ctx); err != nil {
		t.Fatal(err)
	}

	for _, exp := range expMsgs {
		var actMsg *message.Batch
		if actMsg, _, err = m.ReadWithContext(ctx); err != nil {
			t.Error(err)
		} else if act := string(actMsg.Get(0).Get()); act != exp {
			t.Errorf("Wrong result: %v != %v", act, exp)
		}
	}

	m.CloseAsync()
	if err = m.WaitForClose(time.Second); err != nil {
		t.Error(err)
	}
}

func TestWebsocketOpenMsg(t *testing.T) {
	expMsgs := []string{
		"foo",
		"bar",
		"baz",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}

		var ws *websocket.Conn
		var err error
		if ws, err = upgrader.Upgrade(w, r, nil); err != nil {
			return
		}

		defer ws.Close()

		_, data, err := ws.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		if exp, act := "hello world", string(data); exp != act {
			t.Errorf("Wrong open message: %v != %v", act, exp)
		}

		for _, msg := range expMsgs {
			if err = ws.WriteMessage(websocket.BinaryMessage, []byte(msg)); err != nil {
				t.Error(err)
			}
		}
	}))

	conf := NewWebsocketConfig()
	conf.OpenMsg = "hello world"
	if wsURL, err := url.Parse(server.URL); err != nil {
		t.Fatal(err)
	} else {
		wsURL.Scheme = "ws"
		conf.URL = wsURL.String()
	}

	m, err := NewWebsocket(conf, log.Noop(), metrics.Noop())
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	if err = m.ConnectWithContext(ctx); err != nil {
		t.Fatal(err)
	}

	for _, exp := range expMsgs {
		var actMsg *message.Batch
		if actMsg, _, err = m.ReadWithContext(ctx); err != nil {
			t.Error(err)
		} else if act := string(actMsg.Get(0).Get()); act != exp {
			t.Errorf("Wrong result: %v != %v", act, exp)
		}
	}

	m.CloseAsync()
	if err = m.WaitForClose(time.Second); err != nil {
		t.Error(err)
	}
}

func TestWebsocketClose(t *testing.T) {
	closeChan := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}

		var ws *websocket.Conn
		var err error
		if ws, err = upgrader.Upgrade(w, r, nil); err != nil {
			return
		}

		defer ws.Close()
		<-closeChan
	}))

	conf := NewWebsocketConfig()
	if wsURL, err := url.Parse(server.URL); err != nil {
		t.Fatal(err)
	} else {
		wsURL.Scheme = "ws"
		conf.URL = wsURL.String()
	}

	m, err := NewWebsocket(conf, log.Noop(), metrics.Noop())
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	if err = m.ConnectWithContext(ctx); err != nil {
		t.Fatal(err)
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		m.CloseAsync()
		if cErr := m.WaitForClose(time.Second); cErr != nil {
			t.Error(cErr)
		}
		wg.Done()
	}()

	if _, _, err = m.ReadWithContext(ctx); err != component.ErrTypeClosed && err != component.ErrNotConnected {
		t.Errorf("Wrong error: %v != %v", err, component.ErrTypeClosed)
	}

	wg.Wait()
	close(closeChan)
}
