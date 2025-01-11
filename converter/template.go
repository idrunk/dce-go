package converter

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"github.com/idrunk/dce-go/router"
	"github.com/idrunk/dce-go/util"
	"io/fs"
	"os"
	"path/filepath"
	"text/template"
)

type TemplateEngine[Rp router.RoutableProtocol, Resp any] struct {
	*router.Context[Rp]
	tpl *template.Template
}

func FileTemplate[Rp router.RoutableProtocol, Resp any](c *router.Context[Rp], tplPath string) TemplateEngine[Rp, Resp] {
	return TemplateEngine[Rp, Resp]{c, fileTemplate(tplPath, "")}
}

func TextTemplate[Rp router.RoutableProtocol, Resp any](c *router.Context[Rp], text string) TemplateEngine[Rp, Resp] {
	return TemplateEngine[Rp, Resp]{c, textTemplate(text, "")}
}

func EmptyTemplate[Rp router.RoutableProtocol](c *router.Context[Rp]) TemplateEngine[Rp, router.DoNotConvert] {
	return TemplateEngine[Rp, router.DoNotConvert]{Context: c}
}

func fileTemplate(tplPath string, key string) *template.Template {
	if len(key) == 0 {
		key = tplPath
	}
	return TplConfig.templateOrGen(key, func() *template.Template {
		tplPath = TplConfig.root() + tplPath
		if !fs.ValidPath(tplPath) {
			panic("invalid template path: " + tplPath)
		}
		return template.Must(template.ParseFiles(tplPath))
	})
}

func textMd5(text string) string {
	hash := md5.New()
	hash.Write([]byte(text))
	return fmt.Sprintf("%x", hash.Sum(nil))
}

func textTemplate(text string, key string) *template.Template {
	if len(key) == 0 {
		key = textMd5(text)
	}
	return TplConfig.templateOrGen(key, func() *template.Template {
		tpl, err := template.New(key).Parse(text)
		if err != nil {
			panic(err.Error())
		}
		return tpl
	})
}

func (t *TemplateEngine[Rp, Resp]) Serialize(resp Resp) ([]byte, error) {
	buff := new(bytes.Buffer)
	if err := t.tpl.Execute(buff, resp); err != nil {
		return nil, err
	}
	return buff.Bytes(), nil
}

func (t *TemplateEngine[Rp, Resp]) Response(resp Resp) bool {
	if bs, err := t.Serialize(resp); err != nil {
		t.Rp.SetError(err)
	} else if _, err := t.Rp.Write(bs); err != nil {
		t.Rp.SetError(err)
	}
	return true
}

func (t *TemplateEngine[Rp, Resp]) Error(err error) bool {
	code, msg := util.ResponseUnits(err)
	return t.Status(false, msg, code, nil)
}

func (t *TemplateEngine[Rp, Resp]) Success(data any) bool {
	return t.Status(true, "", 0, data)
}

func (t *TemplateEngine[Rp, Resp]) Fail(msg string, code int) bool {
	return t.Status(false, msg, code, nil)
}

func (t *TemplateEngine[Rp, Resp]) Status(status bool, msg string, code int, data any) bool {
	if code == 0 {
		code = util.ServiceUnavailable
	}
	s := router.Status{Status: status, Msg: msg, Code: code, Data: data}
	var tpl *template.Template
	if code == 404 {
		tpl = statusTemplate(NotfoundTplId)
	} else {
		tpl = statusTemplate(StatusTplId)
	}
	buff := new(bytes.Buffer)
	if err := tpl.Execute(buff, s); err != nil {
		t.Rp.SetError(err)
	} else if _, err := t.Write(buff.Bytes()); err != nil {
		t.Rp.SetError(err)
	}
	return true
}

func statusTemplate(tplId string) *template.Template {
	if tplId == StatusTplId {
		if path, text := TplConfig.statusTpl(); len(path) > 0 {
			return fileTemplate(path, StatusTplId)
		} else {
			return textTemplate(text, StatusTplId)
		}
	} else if path, text := TplConfig.notfoundTpl(); len(path) > 0 {
		return fileTemplate(path, NotfoundTplId)
	} else {
		return textTemplate(text, NotfoundTplId)
	}
}

type TemplateConfig struct{ *util.Config }

func (t *TemplateConfig) templateOrGen(key string, supplier func() *template.Template) *template.Template {
	if tpl, ok := t.Scalar(key); ok {
		return tpl.(*template.Template)
	}
	tpl := supplier()
	t.SetScalar(key, tpl)
	return tpl
}

func (t *TemplateConfig) root() string {
	root, _ := t.Scalar("root_dir")
	return root.(string)
}

func (t *TemplateConfig) SetRoot(root string) *TemplateConfig {
	t.SetScalar("root_dir", root)
	return t
}

func (t *TemplateConfig) statusTpl() (path string, text string) {
	if path, _ := t.Scalar("status_path"); len(path.(string)) > 0 {
		return path.(string), ""
	} else {
		text, _ := t.Scalar("status_text")
		return "", text.(string)
	}
}

func (t *TemplateConfig) SetStatusTpl(path string, text string) *TemplateConfig {
	t.SetScalar("status_path", path)
	t.SetScalar("status_text", text)
	return t
}

func (t *TemplateConfig) notfoundTpl() (path string, text string) {
	if path, _ := t.Scalar("notfound_path"); len(path.(string)) > 0 {
		return path.(string), ""
	} else {
		text, _ := t.Scalar("notfound_text")
		return "", text.(string)
	}
}

func (t *TemplateConfig) SetNotfoundTpl(path string, text string) *TemplateConfig {
	t.SetScalar("notfound_path", path)
	t.SetScalar("notfound_text", text)
	return t
}

var TplConfig = TemplateConfig{util.NewConfig()}

const (
	StatusTplId   = "status_tmpl_id_status"
	NotfoundTplId = "status_tmpl_id_notfound"
)

func init() {
	exe, err := os.Executable()
	if err != nil {
		panic(err)
	}
	root := filepath.Dir(exe)
	TplConfig.
		SetRoot(root+"/assets/templates/").
		SetNotfoundTpl("", `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Not found</title>
</head>
<body>
Not found
</body>
</html>`).
		SetStatusTpl("", `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Exception ({{.Code}})</title>
</head>
<body>
<h1>Exception</h1>
<p>{{.Code}}: {{.Msg}}</p>
</body>
</html>`)
}
