package apis

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/coder/websocket"
	"github.com/idrunk/dce-go/converter"
	"github.com/idrunk/dce-go/proto"
	"github.com/idrunk/dce-go/proto/flex"
	"github.com/idrunk/dce-go/session"
	"github.com/idrunk/dce-go/util"
	"log"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"os"
	"slices"
	"time"
)

func init() {
	proto.CliRouter.Push("websocket/start", FlexWebsocketStart)
}

func FlexWebsocketStart(c *proto.Cli) {
	port := c.Rp.ArgOr("-p", "2047")
	flexWebsocketBind(port)
	fmt.Printf("FlexWebsocket server is starting on port %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, http.HandlerFunc(proto.HttpRouter.Route)))
}

func flexWebsocketBind(port string) {
	wd, _ := os.Getwd()
	converter.TplConfig.SetRoot(wd + "/apis/")

	proto.HttpRouter.Get("", func(h *proto.Http) {
		t := converter.FileTemplate[*proto.HttpProtocol, struct{ ServerAddr string }](h, "flex-websocket.html")
		t.Response(struct{ ServerAddr string }{
			ServerAddr: "ws://127.0.0.1:" + port + "/ws",
		})
	})

	proto.HttpRouter.Get("ws/{sid?}", func(h *proto.Http) {
		c, err := websocket.Accept(h.Rp.Writer, h.Rp.Req, nil)
		if err != nil {
			slog.Warn(err.Error())
			return
		}
		shadowSess, err := session.NewShmSession[*session.SimpleUser]([]string{h.Param("sid")}, session.DefaultTtlMinutes)
		flex.WebsocketRouter.SetMapping(h.Rp.Req.RemoteAddr, c)
		defer func() {
			flex.WebsocketRouter.Unmapping(h.Rp.Req.RemoteAddr)
			// notify others to sync-user-list
			go syncUserList(shadowSess, h, nil)
		}()
		if shadowSess != nil {
			shadowSess.Connect(":2047", h.Rp.Req.RemoteAddr, true)
			defer shadowSess.Disconnect()
			if _, ok := shadowSess.User(); !ok {
				// auto register and login
				_ = shadowSess.Login(genUser(), session.DefaultTtlMinutes)
				// must be generated a new sid when login called
				h.Rp.SetRespSid(shadowSess.Id())
			}
			user, ok := shadowSess.User()
			if ok {
				flex.WebsocketRouter.UidSetMapping(h.Rp.Req.RemoteAddr, user.Uid())
			}
			// notify all to sync-user-list
			go syncUserList(shadowSess, h, user)
		}
		for {
			ctxData := map[string]any{"$shadowSession": shadowSess}
			if !flex.WebsocketRouter.Route(c, h.Rp.Req, ctxData) {
				break
			}
		}
		_ = c.Close(websocket.StatusNormalClosure, "")
	})

	flex.WebsocketRouter.Push("send", func(w *flex.Websocket) {
		msg, err := w.Body()
		if err != nil && w.SetError(err) {
			return
		}
		go func(w *flex.Websocket) {
			sess := w.Rp.Session()
			user, _ := sess.(*session.ShmSession[*session.SimpleUser]).User()
			body, _ := json.Marshal(struct {
				Uid  uint64 `json:"uid,omitempty"`
				Nick string `json:"nick,omitempty"`
				Msg  string `json:"msg,omitempty"`
				Time string `json:"time,omitempty"`
			}{
				user.Uid(),
				user.Nick,
				string(msg),
				time.Now().Format("06-01-02 15:04:05"),
			})
			pkg := flex.NewPackage("sync-new-message", body, "", 0).Serialize()
			for _, conn := range flex.WebsocketRouter.ConnMapping() {
				go conn.Write(w, websocket.MessageBinary, pkg)
			}
		}(w)
		_, _ = w.Write([]byte{'1'})
	})

	flex.WebsocketRouter.SetEventHandler(func(ctx *flex.Websocket) error {
		shadowSession, _ := ctx.Rp.CtxData("$shadowSession")
		rs := shadowSession.(*session.ShmSession[*session.SimpleUser])
		cloned, _ := rs.CloneForRequest(ctx.Rp.Sid())
		sess := cloned.(*session.ShmSession[*session.SimpleUser])
		if _, ok := sess.User(); !ok {
			return util.Openly(401, "Unauthorized")
		}
		ctx.Rp.SetSession(sess)
		auto := session.NewAutoRenew(sess)
		if gotNew, err := auto.TryRenew(); err == nil && gotNew {
			ctx.Rp.SetRespSid(sess.Id())
		}
		return nil
	}, nil)
}

func syncUserList(sess *session.ShmSession[*session.SimpleUser], h *proto.Http, user *session.SimpleUser) {
	var userList []session.SimpleUser
	connList := make(map[string]*websocket.Conn)
	for addr, uid := range flex.WebsocketRouter.UidMapping() {
		if uses, _ := sess.ListByUid(uid); len(uses) > 0 {
			us := uses[0].(*session.ShmSession[*session.SimpleUser])
			if u, o := us.User(); o {
				conn, _ := flex.WebsocketRouter.ConnBy(addr)
				connList[addr] = conn
				if !slices.ContainsFunc(userList, func(ru session.SimpleUser) bool {
					return ru.Id == u.Id
				}) {
					userList = append(userList, *u)
				}
			}
		}
	}
	resp := struct {
		UserList    []session.SimpleUser `json:"userList"`
		SessionUser *session.SimpleUser  `json:"sessionUser,omitempty"`
	}{
		UserList: userList,
	}
	respJson, _ := json.Marshal(resp)
	respPkg := flex.NewPackage("sync-user-list", respJson, "", 0).Serialize()
	var respSessionPkg []byte
	if user != nil {
		respSession := resp
		respSession.SessionUser = user
		respSessionJson, _ := json.Marshal(respSession)
		respSessionPkg = flex.NewPackage("sync-user-list", respSessionJson, h.Rp.RespSid(), 0).Serialize()
	}
	ctx := context.Background()
	for addr, conn := range connList {
		if addr == h.Rp.Req.RemoteAddr {
			_ = conn.Write(ctx, websocket.MessageBinary, respSessionPkg)
		} else {
			_ = conn.Write(ctx, websocket.MessageBinary, respPkg)
		}
	}
}

var incrementUid uint64 = 0

func genUser() *session.SimpleUser {
	namePool := []string{"Drunk", "Golang", "午言"}
	now := time.Now()
	hash := sha256.New()
	hash.Write([]byte(fmt.Sprintf("%d-%d", now.UnixNano(), rand.Uint())))
	incrementUid++
	return &session.SimpleUser{
		Id:   incrementUid,
		Nick: fmt.Sprintf("%s-%x", namePool[rand.IntN(len(namePool))], hash.Sum(nil)[:3]),
	}
}
