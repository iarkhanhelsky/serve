package server

import (
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type browseEntry struct {
	Name    string
	Href    string
	IsDir   bool
	Size    int64
	ModTime string
	Type    string
}

type browsePage struct {
	Path    string
	Entries []browseEntry
}

var browseTemplate = template.Must(template.New("browse").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Index of {{.Path}}</title>
  <style>
    body { font-family: system-ui, -apple-system, sans-serif; margin: 2rem; }
    table { border-collapse: collapse; width: 100%; }
    th, td { text-align: left; padding: 0.5rem; border-bottom: 1px solid #ddd; }
    .type { width: 4rem; }
    .size { width: 8rem; text-align: right; }
    .mod { width: 14rem; white-space: nowrap; }
  </style>
</head>
<body>
  <h1>Index of {{.Path}}</h1>
  <table>
    <thead>
      <tr><th>Name</th><th class="type">Type</th><th class="size">Size</th><th class="mod">Modified</th></tr>
    </thead>
    <tbody>
      {{range .Entries}}
      <tr>
        <td><a href="{{.Href}}">{{.Name}}</a></td>
        <td class="type">{{.Type}}</td>
        <td class="size">{{if .IsDir}}-{{else}}{{.Size}}{{end}}</td>
        <td class="mod">{{.ModTime}}</td>
      </tr>
      {{end}}
    </tbody>
  </table>
</body>
</html>
`))

func newStaticHandler(root string) http.Handler {
	fileServer := http.FileServer(http.Dir(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cleanPath := path.Clean("/" + strings.TrimPrefix(r.URL.Path, "/"))
		fullPath := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(cleanPath, "/")))

		info, err := os.Stat(fullPath)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		if info.IsDir() {
			renderBrowse(w, r, root, cleanPath, fullPath)
			return
		}

		fileServer.ServeHTTP(w, r)
	})
}

func renderBrowse(w http.ResponseWriter, r *http.Request, root, reqPath, fullPath string) {
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		http.Error(w, "read directory failed", http.StatusInternalServerError)
		return
	}

	items := make([]browseEntry, 0, len(entries)+1)
	if reqPath != "/" {
		parent := path.Dir(strings.TrimSuffix(reqPath, "/"))
		if parent == "." {
			parent = "/"
		}
		if !strings.HasSuffix(parent, "/") {
			parent += "/"
		}
		items = append(items, browseEntry{Name: "..", Href: parent, IsDir: true, Type: "dir"})
	}

	for _, ent := range entries {
		info, err := ent.Info()
		if err != nil {
			continue
		}
		name := ent.Name()
		hrefPath := strings.TrimSuffix(reqPath, "/")
		if hrefPath == "" {
			hrefPath = "/"
		}
		if hrefPath == "/" {
			hrefPath += url.PathEscape(name)
		} else {
			hrefPath += "/" + url.PathEscape(name)
		}
		isDir := ent.IsDir()
		if isDir {
			hrefPath += "/"
		}
		items = append(items, browseEntry{
			Name:    name,
			Href:    hrefPath,
			IsDir:   isDir,
			Size:    info.Size(),
			ModTime: info.ModTime().Format(time.RFC3339),
			Type:    fileType(isDir),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Name == ".." {
			return true
		}
		if items[j].Name == ".." {
			return false
		}
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = browseTemplate.Execute(w, browsePage{
		Path:    reqPath,
		Entries: items,
	})
}

func fileType(isDir bool) string {
	if isDir {
		return "dir"
	}
	return "file"
}
