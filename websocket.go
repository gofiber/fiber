// 🚀 Fiber is an Express.js inspired web framework written in Go with 💖
// 📌 Please open an issue if you got suggestions or found a bug!
// 🖥 Links: https://github.com/gofiber/fiber, https://fiber.wiki

// 🦸 Not all heroes wear capes, thank you to some amazing people
// 💖 @valyala, @erikdubbelboer, @savsgio, @julienschmidt, @koddr

package fiber

import (
	"sync"

	websocket "github.com/fasthttp/websocket"
)

// Conn ...
type Conn struct {
	*websocket.Conn
}

// Conn pool
var poolConn = sync.Pool{
	New: func() interface{} {
		return new(Conn)
	},
}

// Get new Conn from pool
func acquireConn(socket *websocket.Conn) *Conn {
	conn := poolConn.Get().(*Conn)
	conn.Conn = socket
	return conn
}

// Return Conn to pool
func releaseConn(conn *Conn) {
	poolConn.Put(conn)
}

// Socket ...
func (app *Application) Socket(args ...interface{}) *Application {
	app.register("SOCKET", args...)
	return app
}

// On ...
func (c *Conn) On(event string, cb func(string)) error {
	if event == "message" {
		for {
			_, p, err := c.ReadMessage()
			if err != nil {
				return err
			}
			// msgType 1: websocket.BinaryMessage
			// msgType 2: websocket.TextMessage
			cb(getString(p))
		}
	} else if event == "close" {

	}
	return nil
}

// Send ...
func (c *Conn) Send(data string) error {
	if err := c.WriteMessage(1, getBytes(data)); err != nil {
		return err
	}
	return nil
}
