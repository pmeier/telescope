package ui

import (
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"path/filepath"
	"regexp"
	"slices"

	"github.com/Masterminds/sprig/v3"
)

type TemplateGroupFS interface {
	fs.FS
	fs.ReadFileFS
}

type TemplateGroup struct {
	tpls map[string]*template.Template
}

func NewTemplateGroup() *TemplateGroup {
	return &TemplateGroup{tpls: map[string]*template.Template{}}
}

func (tg *TemplateGroup) ParseFS(fsys TemplateGroupFS, root string) (*TemplateGroup, error) {
	texts, err := readFiles(fsys, root)
	if err != nil {
		return nil, err
	}

	tpls := make(map[string]*template.Template, len(texts))
	for groupName, memberNames := range resolveDependencies(texts) {
		tpl := template.New(groupName)
		for _, n := range memberNames {
			var t *template.Template
			if n == groupName {
				t = tpl
			} else {
				t = tpl.New(n)
			}
			template.Must(t.Funcs(sprig.FuncMap()).Parse(texts[n]))
		}

		tpls[groupName] = tpl
	}

	tg.tpls = tpls
	return tg, nil
}

func readFiles(fsys TemplateGroupFS, root string) (map[string]string, error) {
	texts := map[string]string{}

	err := fs.WalkDir(fsys, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		} else if d.IsDir() {
			return nil
		}

		c, err := fsys.ReadFile(path)
		if err != nil {
			return err
		}
		t := string(c)

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		texts[filepath.ToSlash(rel)] = t

		return nil
	})

	if err != nil {
		return nil, err
	} else {
		return texts, nil
	}
}

func resolveDependencies(texts map[string]string) map[string][]string {
	r := regexp.MustCompile(`{{.*?template\s+"(?P<dependency>[^\"]+)".*?}}`)
	i := r.SubexpIndex("dependency")
	deps := make(map[string][]string, len(texts))
	for name, t := range texts {
		ds := []string{}
		for _, match := range r.FindAllStringSubmatch(t, -1) {
			ds = append(ds, match[i])
		}
		deps[name] = ds
	}

	resolveCache := make(map[string][]string, len(texts))
	var resolve func(string) []string
	resolve = func(name string) []string {
		ds := []string{name}
		for _, d := range deps[name] {
			sds, ok := resolveCache[d]
			if !ok {
				sds = resolve(d)
			}
			ds = append(ds, sds...)
		}

		resolveCache[name] = ds
		return ds
	}

	resolvedDeps := make(map[string][]string, len(texts))
	for name := range deps {
		resolvedDeps[name] = slices.Compact(resolve(name))
	}

	return resolvedDeps
}

func (tg *TemplateGroup) ExecuteTemplate(wr io.Writer, name string, data any) error {
	tpl, ok := tg.tpls[name]
	if !ok {
		return fmt.Errorf("unknown template %s", name)
	}
	return tpl.Execute(wr, data)
}
