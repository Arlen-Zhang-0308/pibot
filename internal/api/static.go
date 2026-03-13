package api

import (
	"io/fs"

	"github.com/pibot/pibot/web"
)

var staticFiles fs.FS

func init() {
	var err error
	staticFiles, err = fs.Sub(web.StaticFiles, "static")
	if err != nil {
		panic(err)
	}
}
