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

// Typ: call/result/notify
// Id: unique (call/result)
// Action: function name (call/notify)
// Args: arguments or result (call/result/notify)
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
