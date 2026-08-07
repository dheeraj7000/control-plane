package gateway

import (
	"net/http"

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
func handleEventsWS(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		executionID := chi.URLParam(r, "id")

		conn, err := websocket.Accept(w, r, nil)
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
