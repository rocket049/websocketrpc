package websocketrpc

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/gorilla/websocket"
)

type MyRpcClient struct {
	Id            *atomic.Uint64
	ChannelMap    *sync.Map
	ResultChannel chan []byte
	Conn          *websocket.Conn
	Ws            *WsServer
}

// RpcCmd : data struct for transport remote calls.
//
//	Typ: call/result/notify
//	Id: unique (call/result)
//	Action: function name (call/notify)
//	Args: arguments or result (call/result/notify)
type RpcCmd struct {
	Typ    string      `json:"typ"`
	Id     uint64      `json:"id"`
	Action string      `json:"action"`
	Data   interface{} `json:"data"`
}

func NewMyRpcClient() *MyRpcClient {
	id := &atomic.Uint64{}
	id.Store(0)
	m := &sync.Map{}
	rch := make(chan []byte, 1)
	ret := &MyRpcClient{
		Id:            id,
		ChannelMap:    m,
		ResultChannel: rch,
	}
	go ret.ResultRecv()
	return ret
}

// Do not call ResultRecv, it is always running backend.
func (p *MyRpcClient) ResultRecv() {
	for data := range p.ResultChannel {
		res := RpcCmd{}
		err := json.Unmarshal(data, &res)
		if err != nil {
			log.Println(err)
			log.Println(string(data))
			continue
		}
		if res.Typ != "result" {
			continue
		}
		ch, ok := p.ChannelMap.Load(res.Id)
		if !ok {
			continue
		}
		channel := ch.(chan interface{})
		channel <- res.Data
		close(channel)
		p.ChannelMap.Delete(res.Id)
	}
}

// Call : remote call function "fn" in web browser. Call will return a channel for result.
//
//	r : this is used to find the connection for the correct client.
//	fn : name of the remote function in web browser.
//	args : arguments of the function, they will be sent as JSON.
func (p *MyRpcClient) Call(r *http.Request, fn string, args interface{}) <-chan interface{} {
	c, err := r.Cookie("websocketid")
	if err != nil {
		log.Println(err)
		return nil
	}
	p.Ws.Lock()
	conn, ok := p.Ws.connMap[c.Value]
	p.Ws.Unlock()
	if ok {
		p.Conn = conn
	} else {
		log.Println("no websocket connection:", c.Value)
		return nil
	}
	id := p.Id.Add(1)
	cmd := &RpcCmd{
		Typ:    "call",
		Id:     id,
		Action: fn,
		Data:   args,
	}

	callData, err := json.Marshal(cmd)
	if err != nil {
		log.Println(err)

		return nil
	}

	res := make(chan interface{}, 1)
	p.ChannelMap.Store(id, res)

	err = p.Conn.WriteMessage(1, callData)
	if err != nil {
		log.Println(err)
		close(res)
		p.ChannelMap.Delete(id)
		return nil
	}
	return res
}

// Notify : remote call function "fn" in web browser. It will not return any results.
//
//	r : this is used to find the connection for the correct client.
//	fn : name of the remote function in web browser.
//	args : arguments of the function, they will be sent as JSON.
func (p *MyRpcClient) Notify(r *http.Request, fn string, args interface{}) {
	c, err := r.Cookie("websocketid")
	if err != nil {
		log.Println(err)
		return
	}
	p.Ws.Lock()
	conn, ok := p.Ws.connMap[c.Value]
	p.Ws.Unlock()
	if ok {
		p.Conn = conn
	} else {
		log.Println("no websocket connection:", c.Value)
		return
	}

	cmd := &RpcCmd{
		Typ:    "notify",
		Id:     0,
		Action: fn,
		Data:   args,
	}

	callData, err := json.Marshal(cmd)
	if err != nil {
		log.Println(err)

		return
	}

	err = p.Conn.WriteMessage(1, callData)
	if err != nil {
		log.Println(err)
	}

}

// GetConnection will return `*websocket.Conn` pointer, index by `r *http.Request`
func (p *MyRpcClient) GetConnection(r *http.Request) *websocket.Conn {
	c, err := r.Cookie("websocketid")
	if err != nil {
		log.Println(err)
		return nil
	}
	p.Ws.Lock()
	conn, ok := p.Ws.connMap[c.Value]
	p.Ws.Unlock()
	if ok {
		p.Conn = conn
		return p.Conn
	} else {
		log.Println("no websocket connection:", c.Value)
		return nil
	}
}

// CallConn : remote call function "fn" in web browser. Call will return a channel for result.
//
//	conn : `*websocket.Conn` pointer.
//	fn : name of the remote function in web browser.
//	args : arguments of the function, they will be sent as JSON.
//
// This function works same as `Call`, but will be more quickly when run very much times.
func (p *MyRpcClient) CallConn(conn *websocket.Conn, fn string, args interface{}) <-chan interface{} {

	p.Conn = conn

	id := p.Id.Add(1)
	cmd := &RpcCmd{
		Typ:    "call",
		Id:     id,
		Action: fn,
		Data:   args,
	}

	callData, err := json.Marshal(cmd)
	if err != nil {
		log.Println(err)

		return nil
	}

	res := make(chan interface{}, 1)
	p.ChannelMap.Store(id, res)

	err = p.Conn.WriteMessage(1, callData)
	if err != nil {
		log.Println(err)
		close(res)
		p.ChannelMap.Delete(id)
		return nil
	}
	return res
}

// NotifyConn : remote call function "fn" in web browser. It will not return any results.
//
//	conn : `*websocket.Conn` pointer.
//	fn : name of the remote function in web browser.
//	args : arguments of the function, they will be sent as JSON.
//
// This function works same as `Notify`, but will be more quickly when run very much times.
func (p *MyRpcClient) NotifyConn(conn *websocket.Conn, fn string, args interface{}) {

	p.Conn = conn

	cmd := &RpcCmd{
		Typ:    "notify",
		Id:     0,
		Action: fn,
		Data:   args,
	}

	callData, err := json.Marshal(cmd)
	if err != nil {
		log.Println(err)

		return
	}

	err = p.Conn.WriteMessage(1, callData)
	if err != nil {
		log.Println(err)
	}

}
