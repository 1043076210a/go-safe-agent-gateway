package web

import (
	"embed"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed static/*
var staticFiles embed.FS

func Register(r *gin.Engine) {
	r.GET("/", index)
	r.GET("/demo", index)
	r.GET("/demo/", index)
	r.GET("/demo/app.js", asset("app.js", "application/javascript; charset=utf-8"))
	r.GET("/demo/styles.css", asset("styles.css", "text/css; charset=utf-8"))
}

func index(c *gin.Context) {
	b, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", b)
}

func asset(name string, contentType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.Contains(name, "/") || strings.Contains(name, "\\") {
			c.Status(http.StatusNotFound)
			return
		}
		b, err := staticFiles.ReadFile("static/" + name)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		c.Data(http.StatusOK, contentType, b)
	}
}
