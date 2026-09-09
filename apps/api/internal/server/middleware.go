package server

import (
	"net/http"
	"runtime/debug"
	"slices"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

const headerRequestID = "X-Request-ID"

// requestID assigns or propagates a request id.
func requestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(headerRequestID)
		if id == "" {
			id = uuid.NewString()
		}
		httpx.SetRequestID(c, id)
		c.Header(headerRequestID, id)
		c.Next()
	}
}

// accessLog writes one structured line per request.
func accessLog(l zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		ev := l.Info()
		status := c.Writer.Status()
		switch {
		case status >= 500:
			ev = l.Error()
		case status >= 400:
			ev = l.Warn()
		}
		ev = ev.
			Str("request_id", httpx.RequestID(c)).
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Int("status", status).
			Dur("latency", time.Since(start)).
			Str("ip", c.ClientIP())
		if len(c.Errors) > 0 {
			ev = ev.Str("error", c.Errors.String())
		}
		ev.Msg("http")
	}
}

// recovery converts panics into a 500 envelope and logs the stack.
func recovery(l zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				l.Error().
					Str("request_id", httpx.RequestID(c)).
					Interface("panic", r).
					Bytes("stack", debug.Stack()).
					Msg("panic recovered")
				c.AbortWithStatusJSON(http.StatusInternalServerError, httpx.Envelope{
					Code: httpx.CodeInternal, Message: "internal error", RequestID: httpx.RequestID(c),
				})
			}
		}()
		c.Next()
	}
}

// cors allows the configured origins. "*" allows any origin (dev only).
func cors(origins []string) gin.HandlerFunc {
	allowAll := slices.Contains(origins, "*")
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && (allowAll || slices.Contains(origins, origin)) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
			c.Header("Access-Control-Expose-Headers", headerRequestID)
			c.Header("Vary", "Origin")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
