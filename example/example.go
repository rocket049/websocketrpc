package main

import (
	_ "embed"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"time"

	"github.com/rocket049/websocketrpc"
)

const Port = 17680

var exit = make(chan int)

type API struct{}

func (a *API) Quit() error {
	exit <- 1
	return nil
}

func main() {
	debug := flag.Bool("debug", false, "debug mode")
	static := flag.String("static", "static", "静态页面目录")
	flag.Parse()
	if !*debug {
		log.SetOutput(io.Discard)
	}

	actions := &API{}
	go serve(actions, *static)

	_ = <-exit
}

func serve(actions *API, static string) {

	l, err := net.Listen("tcp4", fmt.Sprintf("localhost:%v", Port))
	if err != nil {
		panic(err)
	}
	println("serve on:", fmt.Sprintf("http://localhost:%v/?t=%v", Port, time.Now().Unix()))
	//初始化http服务器，得到 rpcClient 指针，指针指向 websocketrpc.MyRpcClient 结构体
	svr := http.NewServeMux()
	httpserver, rpcClient := websocketrpc.CreateServer(svr, "/_myws/_conn/", static)
	//下面用 svr.HandleFunc 添加其他 web api
	//从浏览器调用 /api/calc
	svr.HandleFunc("/api/calc", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
		fmt.Println("call /api/calc")
		x := rand.Uint32() % 100
		y := rand.Uint32() % 100

		//此处调用浏览器中的 eval 函数，返回结果在管道‘ch’中
		ch := rpcClient.Call(r, "eval", fmt.Sprintf("%v+%v", x, y))

		go func() {
			//在线程中，从管道 ch 取出 eval 计算结果到 ret 变量中
			ret := <-ch
			s := fmt.Sprintf("%v + %v = %v\n", x, y, ret)
			//调用浏览器的 show 方法，显示字符串s，Notify 调用无需返回结果
			conn := rpcClient.GetConnection(r)
			rpcClient.NotifyConn(conn, "show", s)
		}()
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	})
	//从浏览器调用 actions.Quit()
	svr.HandleFunc("/api/quit", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
		w.WriteHeader(200)
		w.Write([]byte("ok"))
		fmt.Println("call /api/quit")
		//本地函数 actions.Quit()，将退出本程序
		actions.Quit()
	})

	go func() {
		//启动 http 服务器
		httpserver.Serve(l)
		log.Println("http server stop.")
	}()

}
