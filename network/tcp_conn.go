package network

import (
	"net"
	"sync"

	"github.com/name5566/leaf/log"
)

type ConnSet map[net.Conn]struct{}

// 封装底层net.Conn，提供经过msgParser处理过的数据读写方法ReadMsg()，WriteMsg()
// 向底层连接net.Conn写数据额外提供了一层缓冲写通道
type TCPConn struct {
	sync.Mutex             // 锁
	conn       net.Conn    // 底层连接
	writeChan  chan []byte // 服务器向客户端发送消息时的缓冲写通道,缓冲区大小为*TCPServer.pendingWriteNum
	closeFlag  bool        // 关闭标签
	msgParser  *MsgParser  // 消息解析器，继承自*TCPServer.msgParser
}

func newTCPConn(conn net.Conn, pendingWriteNum int, msgParser *MsgParser) *TCPConn {
	tcpConn := new(TCPConn)
	tcpConn.conn = conn
	tcpConn.writeChan = make(chan []byte, pendingWriteNum) // 缓冲通道,缓冲区大小为pendingWriteNum
	tcpConn.msgParser = msgParser

	// 循环从writeChan中接收数据，并将接收到的数据写入conn
	go func() {
		// 循环从缓冲通道writeChan中接收数据，直到writeChan关闭，否则range一直等待
		for b := range tcpConn.writeChan {
			if b == nil {
				break // 退出整个range循环
			}

			_, err := conn.Write(b)
			if err != nil {
				break
			}
		}

		// 当for...range退出后，即writeChan关闭后，关闭连接
		conn.Close()
		tcpConn.Lock()
		tcpConn.closeFlag = true
		tcpConn.Unlock()
	}()

	return tcpConn
}

// 当服务器向客户端发送数据的缓冲通道已满时，关闭当前连接
func (tcpConn *TCPConn) doDestroy() {
	tcpConn.conn.(*net.TCPConn).SetLinger(0) // 设置关闭连接时忽略未发送数据
	tcpConn.conn.Close()

	if !tcpConn.closeFlag {
		close(tcpConn.writeChan)
		tcpConn.closeFlag = true
	}
}

func (tcpConn *TCPConn) Destroy() {
	tcpConn.Lock()
	defer tcpConn.Unlock()

	tcpConn.doDestroy()
}

func (tcpConn *TCPConn) Close() {
	tcpConn.Lock()
	defer tcpConn.Unlock()
	if tcpConn.closeFlag {
		return
	}

	tcpConn.doWrite(nil)
	tcpConn.closeFlag = true
}

// 服务器向客户端发送消息时，最终会调用到该方法，将消息数据写入缓冲通道tcpConn.writeChan
func (tcpConn *TCPConn) doWrite(b []byte) {
	if len(tcpConn.writeChan) == cap(tcpConn.writeChan) {
		log.Debug("close conn: channel full")
		tcpConn.doDestroy()
		return
	}

	tcpConn.writeChan <- b
}

// b must not be modified by the others goroutines
// tcpConn.msgParser.Write()最终会调回这个方法
func (tcpConn *TCPConn) Write(b []byte) {
	tcpConn.Lock()
	defer tcpConn.Unlock()
	if tcpConn.closeFlag || b == nil {
		return
	}

	tcpConn.doWrite(b)
}

func (tcpConn *TCPConn) Read(b []byte) (int, error) {
	return tcpConn.conn.Read(b)
}

func (tcpConn *TCPConn) LocalAddr() net.Addr {
	return tcpConn.conn.LocalAddr()
}

func (tcpConn *TCPConn) RemoteAddr() net.Addr {
	return tcpConn.conn.RemoteAddr()
}

func (tcpConn *TCPConn) ReadMsg() ([]byte, error) {
	return tcpConn.msgParser.Read(tcpConn)
}

func (tcpConn *TCPConn) WriteMsg(args ...[]byte) error {
	return tcpConn.msgParser.Write(tcpConn, args...)
}
