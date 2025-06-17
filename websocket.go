// User only need to use 3 functions:
//
//	websocketrpc.CreateServer
//	websocketrpc.MyRpcClient.Call
//	websocketrpc.MyRpcClient.Notify
//
// Do not use other functions.
//
// Read "example/example.go" and "example/static/main.js" to learn how to use.
package websocketrpc

import (
	"log"
	"net/http"

	"gitee.com/rocket049/syncmap"

	"github.com/gorilla/websocket"

	"time"
)

// User just need to use the function "CreateServer", use it like follow:
//
//	svr := http.NewServeMux()
//	httpserver, rpcClient := websocketrpc.CreateServer(svr, "/_myws/_conn/", static)
//	// add other web api
//	// user can use rpcClient.Call and rpcClient.Notify call javascript functions in browser
//	httpserver.Serve( listener )
//
// If websocketPath=="", it will use default path: "/_myws/_conn/" .
// static is the dir of static files, the home page is index.html .
func CreateServer(svr *http.ServeMux, websocketPath, rootStatic string) (*http.Server, *MyRpcClient) {
	fshandler := http.FileServer(http.Dir(rootStatic))
	svr.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost*")
		cookie := http.Cookie{Name: "websocketid", Value: r.RemoteAddr}
		w.Header().Add("Set-Cookie", cookie.String())
		fshandler.ServeHTTP(w, r)
	})

	wsPath := "/_myws/_conn/"
	if websocketPath != "" {
		wsPath = websocketPath
	}
	rpcClient := NewMyRpcClient()
	ws := &WsServer{rpcClient: rpcClient, connMap: syncmap.NewSyncMap(make(map[string]*websocket.Conn))}
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
	connMap   *syncmap.SyncMap[string, *websocket.Conn]
	rpcClient *MyRpcClient
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
		log.Println(err, ": Try refresh the page.")
		return
	}

	s.connMap.Put(c.Value, conn)

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
	s.connMap.Delete(c.Value)

}
