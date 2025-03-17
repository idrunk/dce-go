package converter

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"text/template"

	"go.drunkce.com/dce/router"
	"go.drunkce.com/dce/util"
)

func TemplateResponser[Rp router.RoutableProtocol, D any](ctx *router.Context[Rp], tmpl *template.Template) *router.Responser[Rp, D, D] {
	return &router.Responser[Rp, D, D]{ Context: ctx, Serializer: TemplateEngine[D]{tmpl}}
}

func FileTemplate[Rp router.RoutableProtocol, D any](c *router.Context[Rp], tplPath string) *router.Responser[Rp, D, D] {
	return TemplateResponser[Rp, D](c, fileTemplate(tplPath, ""))
}

func TextTemplate[Rp router.RoutableProtocol, D any](c *router.Context[Rp], text string) *router.Responser[Rp, D, D] {
	return TemplateResponser[Rp, D](c, textTemplate(text, ""))
}

func StatusTemplate[Rp router.RoutableProtocol](c *router.Context[Rp]) *router.Responser[Rp, *router.Status, *router.Status] {
	return TemplateResponser[Rp, *router.Status](c, nil)
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

type TemplateEngine[D any] struct {
	*template.Template
}

func (t TemplateEngine[D]) Serialize(resp D) ([]byte, error) {
	tpl := t.Template
	if status, ok := any(resp).(*router.Status); ok {
		if status.Code == 0 {
			status.Code = util.ServiceUnavailable
		}
		if status.Code == 404 {
			tpl = statusTemplate(NotfoundTplId)
		} else {
			tpl = statusTemplate(StatusTplId)
		}
	}
	buff := new(bytes.Buffer)
	if err := tpl.Execute(buff, resp); err != nil {
		return nil, err
	}
	return buff.Bytes(), nil
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
