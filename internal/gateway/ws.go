package gateway

import (
	"net/http"
	"slices"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/go-chi/chi/v5"
)

// handleEventsWS streams executionID's live events as they're
// recorded. Simplifications, deliberate for this milestone: no ping/
// pong keepalive tuning beyond the library's defaults, no replay of
// events that happened before the socket connected (a client that
// wants full history should GET .../events first, then open this
// socket for what happens next — merging the two into one gapless
// stream is a real problem, left as a known limitation rather than
// solved partially here).
//
// allowedOrigins configures coder/websocket's own Origin check
// (distinct from, and in addition to, the CORS middleware every HTTP
// route gets — CORS doesn't apply to the WebSocket handshake, but
// coder/websocket.Accept enforces a same-origin policy by default and
// rejects a cross-origin handshake with 403 otherwise). Found via a
// real browser hitting this endpoint from the dashboard's origin
// during Milestone 6 verification — no Go-based test caught it,
// because net/http/httptest clients never send an Origin header the
// way a browser's WebSocket API does. Reuses
// config.Config.CORSAllowedOrigins rather than adding a second origin
// list to configure.
func handleEventsWS(svc *Service, allowedOrigins []string) http.HandlerFunc {
	acceptOpts := &websocket.AcceptOptions{OriginPatterns: allowedOrigins}
	if slices.Contains(allowedOrigins, "*") {
		// coder/websocket's own docs ask callers to prefer
		// InsecureSkipVerify over an OriginPatterns entry of "*" for
		// this case, rather than relying on "*" happening to satisfy
		// its glob matcher.
		acceptOpts = &websocket.AcceptOptions{InsecureSkipVerify: true}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		executionID := chi.URLParam(r, "id")

		conn, err := websocket.Accept(w, r, acceptOpts)
		if err != nil {
			return // Accept already wrote an appropriate HTTP error response.
		}
		defer func() { _ = conn.CloseNow() }()

		ctx := conn.CloseRead(r.Context()) // this socket is send-only; discard/react to client frames as connection-close signals

		ch, err := svc.SubscribeEvents(ctx, executionID)
		if err != nil {
			_ = conn.Close(websocket.StatusInternalError, "subscribe failed")
			return
		}

		for {
			select {
			case e, ok := <-ch:
				if !ok {
					_ = conn.Close(websocket.StatusNormalClosure, "execution stream ended")
					return
				}
				if err := wsjson.Write(ctx, conn, e); err != nil {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}
}
