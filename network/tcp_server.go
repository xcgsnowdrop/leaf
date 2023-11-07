package network

import (
	"net"
	"sync"
	"time"

	"github.com/name5566/leaf/log"
)

// 负责监听本地网络地址，并管理底层连接net.Conn，为每一个连接创建一个agent，并在go协程中运行agent.Run()
type TCPServer struct {
	Addr            string               // 连接地址
	MaxConnNum      int                  // 允许最大连接数
	PendingWriteNum int                  // 传递给*network.TCPConn用
	NewAgent        func(*TCPConn) Agent // 用来新建Agent的方法
	ln              net.Listener         // 监听器对象
	conns           ConnSet              // 连接池，map结构：ConnSet map[net.Conn]struct{}，存放*TCPServer.ln接受到的连接
	mutexConns      sync.Mutex           // 连接池锁
	wgLn            sync.WaitGroup
	wgConns         sync.WaitGroup

	// msg parser
	LenMsgLen    int    // MsgParser属性
	MinMsgLen    uint32 // MsgParser属性
	MaxMsgLen    uint32 // MsgParser属性
	LittleEndian bool   // MsgParser属性
	msgParser    *MsgParser
}

func (server *TCPServer) Start() {
	server.init()
	go server.run()
}

func (server *TCPServer) init() {
	ln, err := net.Listen("tcp", server.Addr)
	if err != nil {
		log.Fatal("%v", err)
	}

	// 设置默认MaxConnNum
	if server.MaxConnNum <= 0 {
		server.MaxConnNum = 100
		log.Release("invalid MaxConnNum, reset to %v", server.MaxConnNum)
	}
	// 设置默认PendingWriteNum
	if server.PendingWriteNum <= 0 {
		server.PendingWriteNum = 100
		log.Release("invalid PendingWriteNum, reset to %v", server.PendingWriteNum)
	}
	// *TCPServer必须要有定义NewAgent方法
	if server.NewAgent == nil {
		log.Fatal("NewAgent must not be nil")
	}

	server.ln = ln
	server.conns = make(ConnSet)

	// msg parser
	msgParser := NewMsgParser()
	msgParser.SetMsgLen(server.LenMsgLen, server.MinMsgLen, server.MaxMsgLen)
	msgParser.SetByteOrder(server.LittleEndian)
	server.msgParser = msgParser
}

// 管理底层连接net.Conn，为每一个连接创建一个agent，并在go协程中运行agent.Run()
func (server *TCPServer) run() {
	server.wgLn.Add(1)
	defer server.wgLn.Done()

	var tempDelay time.Duration
	for {
		conn, err := server.ln.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				log.Release("accept error: %v; retrying in %v", err, tempDelay)
				time.Sleep(tempDelay)
				continue
			}
			return
		}
		tempDelay = 0

		server.mutexConns.Lock()
		if len(server.conns) >= server.MaxConnNum {
			server.mutexConns.Unlock()
			conn.Close()
			log.Debug("too many connections")
			continue
		}
		server.conns[conn] = struct{}{}
		server.mutexConns.Unlock()

		server.wgConns.Add(1)

		tcpConn := newTCPConn(conn, server.PendingWriteNum, server.msgParser)
		agent := server.NewAgent(tcpConn)
		go func() {
			agent.Run()

			// cleanup
			tcpConn.Close()
			server.mutexConns.Lock()
			delete(server.conns, conn)
			server.mutexConns.Unlock()
			agent.OnClose()

			server.wgConns.Done()
		}()
	}
}

func (server *TCPServer) Close() {
	server.ln.Close()
	server.wgLn.Wait()

	server.mutexConns.Lock()
	for conn := range server.conns {
		conn.Close()
	}
	server.conns = nil
	server.mutexConns.Unlock()
	server.wgConns.Wait()
}
