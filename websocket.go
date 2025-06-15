package websocketrpc

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"

	"time"
)

func CreateServer(svr *http.ServeMux, websocketPath, rootStatic string) (*http.Server, *MyRpcClient) {
	fshandler := http.FileServer(http.Dir(rootStatic))
	svr.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
		cookie := http.Cookie{Name: "websocketid", Value: r.RemoteAddr}
		w.Header().Add("Set-Cookie", cookie.String())
		fshandler.ServeHTTP(w, r)
	})

	wsPath := "/_myws/_conn/"
	if websocketPath == "" {
		wsPath = websocketPath
	}
	rpcClient := NewMyRpcClient()
	ws := &WsServer{rpcClient: rpcClient, connMap: make(map[string]*websocket.Conn)}
	rpcClient.Ws = ws
	svr.HandleFunc(wsPath, func(w http.ResponseWriter, r *http.Request) {
		log.Println("my websocket connect", r.Proto)
		ws.ServeHTTP(w, r)
	})

	httpserver := &http.Server{Handler: svr}
	httpserver.SetKeepAlivesEnabled(true)

	return httpserver, rpcClient
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// 可以添加更多的配置，例如校验请求的Origin等
	CheckOrigin: func(r *http.Request) bool {
		// 这里可以添加更多的逻辑来决定是否接受这个连接，例如校验Origin等
		return true
	},
}

type WsServer struct {
	locker    sync.Mutex
	connMap   map[string]*websocket.Conn
	rpcClient *MyRpcClient
}

func (s *WsServer) Lock() {
	s.locker.Lock()
}

func (s *WsServer) Unlock() {
	s.locker.Unlock()
}

func (s *WsServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	c, err := r.Cookie("websocketid")
	if err != nil {
		log.Println(err)
		return
	}

	s.locker.Lock()
	s.connMap[c.Value] = conn
	s.locker.Unlock()
	log.Println("websocket upgraded:", c.Value)

	//msg := make(map[string]string, 2)
	var clientCorrect bool = false
	var n uint64 = 0
	// 读取和写入数据到WebSocket连接
loop1:
	for {
		if n > 0 && !clientCorrect {
			break loop1
		}
		conn.SetReadDeadline(time.Now().Add(time.Second * 30))
		mt, message, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			break loop1
		}
		n++
		log.Printf("收到消息:%v: %s: from %v", mt, message, conn.RemoteAddr().String())
		switch string(message) {
		case "myws,connected!":
			clientCorrect = true
			s.locker.Lock()
			s.rpcClient.Conn = s.connMap[r.RemoteAddr]
			s.locker.Unlock()
			log.Println("connected", conn.RemoteAddr().String())
		case "ping-pong":
			err = conn.WriteMessage(mt, message)
			if err != nil {
				log.Println(err)
				break loop1
			}
		default:
			s.rpcClient.ResultChannel <- message

		}
	}
	s.locker.Lock()
	delete(s.connMap, c.Value)
	s.locker.Unlock()
}
