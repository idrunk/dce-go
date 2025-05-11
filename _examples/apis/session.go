package apis

import (
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/redis/go-redis/v9"
	"go.drunkce.com/dce/converter"
	"go.drunkce.com/dce/proto"
	"go.drunkce.com/dce/router"
	"go.drunkce.com/dce/session"
	"go.drunkce.com/dce/session/redises"
	"go.drunkce.com/dce/util"
)

func init() {
	proto.CliRouter.Push("http/start/session", HttpStartSession)
}

func HttpStartSession(c *proto.Cli) {
	bind()
	_ = PrepareRedis(c.Rp.ArgOr("redis", ":6379"))
	port := c.Rp.ArgOr("port", "2050")
	fmt.Printf("http server is starting at :%s\n", port)
	if err := http.ListenAndServe(":"+port, http.HandlerFunc(proto.HttpRouter.Route)); err != nil {
		panic(err)
	}
}

func bind() {
	// curl http://127.0.0.1:2050/
	proto.HttpRouter.Get("", func(h *proto.Http) {
		_, _ = h.WriteString("This is a public page, you can access without a token")
	})

	// curl http://127.0.0.1:2050/login -d "{""name"":""Drunk""}" // role 1
	// curl http://127.0.0.1:2050/login -d "{""name"":""Dce""}" // role 2
	proto.HttpRouter.Post("login", func(h *proto.Http) {
		jc := converter.JsonMapResponser(h)
		u, ok := converter.JsonMapRequester(h).Parse()
		if !ok {
			return
		}
		name, ok := u["name"].(string)
		if !ok && jc.Fail("Name required", 1000) {
			return
		}
		member, ok := util.SeqFrom(members()).Find(func(m Member) bool {
			return strings.EqualFold(m.Name, name)
		})
		if !ok && jc.Fail("Wrong name", 1001) {
			return
		}
		se := h.Rp.Session()
		if se == nil && jc.Fail("Invalid session", 999) {
			return
		}
		sess := se.(*redises.Session[*Member])
		if err := sess.Login(&member, session.DefaultTtlMinutes); err != nil && h.SetError(err) && jc.Fail("Failed to login", 1002) {
			return
		}
		h.Rp.SetRespSid(sess.Id())
		_, _ = h.WriteString(fmt.Sprintf("Succeed login with:\n%v", member))
	})

	// curl http://127.0.0.1:2050/manage/profile //  without sid, cannot access got 401
	// curl http://127.0.0.1:2050/manage/profile -H "X-Session-Id: $session_id" //  pass sid on header, can access if sid is valid
	// curl http://127.0.0.1:2050/manage/profile -b "session_id=$session_id" //  pass sid in cookies, can access if sid is valid
	// curl http://127.0.0.1:2050/manage/profile?autologin=1 -H "X-Session-Id: $session_id" //  use long life sid to do auto login, will get new sid and the old will destroy
	proto.HttpRouter.PushApi(router.Path("manage/profile").ByMethod(proto.HttpGet).Append("roles", 1, 2).BindHosts("2050"), func(h *proto.Http) {
		se := h.Rp.Session()
		if se == nil && h.SetError(util.Openly0("Invalid session")) {
			return
		}
		sess := se.(*redises.Session[*Member])
		member, _ := sess.User()
		_, _ = h.WriteString(fmt.Sprintf("Your profile:\n%v", member))
	})

	// curl -X PATCH http://127.0.0.1:2050/manage/profile -H "X-Session-Id: $session_id" -d "{}" // none required fields, got openly err response
	// curl -X PATCH http://127.0.0.1:2050/manage/profile -H "X-Session-Id: $session_id" -d "{""name"":""Foo"",""role_id"":2}" // with required, curren session user will update to role 2
	proto.HttpRouter.PushApi(router.Path("manage/profile").ByMethod(proto.HttpPatch).Append("roles", 1), func(h *proto.Http) {
		jc := converter.JsonMapResponser(h)
		u, ok := converter.JsonMapRequester(h).Parse()
		if !ok {
			return
		}

		var newName string
		var newRoleId uint16
		if tNewName, ok := u["name"]; ok {
			newName = tNewName.(string)
		}
		if tNewRoleId, ok := u["role_id"]; ok {
			newRoleId = uint16(tNewRoleId.(float64))
		}
		if newName == "" && newRoleId == 0 && jc.Fail("Must specified something to modify", 1010) {
			return
		}
		se := h.Rp.Session()
		if se == nil && h.SetError(util.Openly0("Invalid session")) {
			return
		}
		sess := se.(*redises.Session[*Member])
		member, _ := sess.User()
		member.Name = newName
		member.RoleId = newRoleId
		if err := sess.Sync(&member); err != nil && h.SetError(err) {
			return
		}
		_, _ = h.WriteString(fmt.Sprintf("You have succeed to modified profile to:\n%v", member))
	})

	// curl -I http://127.0.0.1:2050/manage/user -H "X-Session-Id: $session_id" // got 403 if the session user role is 1, you can use role 2 user login to access
	// curl http://127.0.0.1:2050/manage/user -H "X-Session-Id: $session_id"
	proto.HttpRouter.PushApi(
		router.Path("manage/user").ByMethod(proto.HttpGet|proto.HttpHead).Append("roles", 2),
		func(h *proto.Http) {
			se := h.Rp.Session()
			if se == nil && h.SetError(util.Openly0("Invalid session")) {
				return
			}
			sess := se.(*redises.Session[*Member])
			member, _ := sess.User()
			_, _ = h.WriteString(fmt.Sprintf("You are role %d, so you can access, your profile:\n%v", (*member).RoleId, member))
		},
	)

	proto.HttpRouter.Raw().
	SetBefore("*", func(ctx *router.Context[*proto.HttpProtocol]) error {
		sess, err := redises.NewSession[*Member](Rdb, []string{ctx.Rp.Sid()}, session.DefaultTtlMinutes)
		if err != nil {
			return err
		}
		auto := session.NewAutoRenew(sess).Config(240, 0, 0)
		auth := NewAppAuth(ctx, auto)
		if err = auth.valid(); err != nil {
			return err
		} else if strings.Contains(ctx.Rp.Req.URL.RequestURI(), "autologin") {
			if err = auth.autoLogin(); err != nil {
				return err
			}
		} else if !auth.isLogin() {
			if err = auth.tryRenew(); err != nil {
				return err
			}
		}
		ctx.Rp.SetSession(sess)
		return nil
	}).
	SetAfter("*", func(ctx *router.Context[*proto.HttpProtocol]) error {
		if newSid := ctx.Rp.RespSid(); newSid != "" {
			_, _ = ctx.Rp.WriteString(fmt.Sprintf("\n\nGot new sid, you can use it to access private page:\n%s", newSid))
		}
		return nil
	})
}

type AppAuth[Rp router.RoutableProtocol] struct {
	ctx        *router.Context[Rp]
	session    *session.AutoRenew[*redises.Session[*Member]]
	rolesNeeds []uint16
}

func NewAppAuth[Rp router.RoutableProtocol](ctx *router.Context[Rp], session *session.AutoRenew[*redises.Session[*Member]]) *AppAuth[Rp] {
	var roles []uint16
	if rls := ctx.Api.ExtrasBy("roles"); rls != nil {
		roles = util.MapSeqFrom[any, uint16](rls).Map(func(a any) uint16 {
			return uint16(a.(int))
		}).Collect()
	}
	return &AppAuth[Rp]{ctx, session, roles}
}

func (a *AppAuth[Rp]) isPrivate() bool {
	return len(a.rolesNeeds) > 0
}

func (a *AppAuth[Rp]) isLogin() bool {
	return strings.HasSuffix(a.ctx.Api.Path, "login")
}

func (a *AppAuth[Rp]) autoLogin() error {
	if err := a.session.S.AutoLogin(); err != nil {
		return err
	}
	a.ctx.Rp.SetRespSid(a.session.S.Id())
	return nil
}

func (a *AppAuth[Rp]) tryRenew() error {
	if ok, err := a.session.TryRenew(); err != nil {
		return err
	} else if ok {
		a.ctx.Rp.SetRespSid(a.session.S.Id())
	}
	return nil
}

func (a *AppAuth[Rp]) valid() error {
	if a.isPrivate() {
		if user, ok := a.session.S.User(); !ok {
			return util.Openly(401, "Unauthorized")
		} else if !slices.Contains(a.rolesNeeds, user.RoleId) {
			return util.Openly(403, "Forbidden")
		}
	}
	return nil
}

func members() []Member {
	return []Member{
		{Id: 1000, Name: "Drunk", RoleId: 1},
		{Id: 1001, Name: "Dce", RoleId: 2},
		{Id: 1002, Name: "Golang", RoleId: 2},
	}
}

type Member struct {
	Id     uint64
	Name   string
	RoleId uint16
}

func (m Member) Uid() uint64 {
	return m.Id
}

var Rdb *redis.Client

func PrepareRedis(addr string) *redis.Client {
	if Rdb != nil {
		return Rdb
	}
	Rdb = redis.NewClient(&redis.Options{Addr: addr})
	return Rdb
}
