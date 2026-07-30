package app

import (
	"context"
	"time"
)

// apiContext holds the startup context that bound methods derive their deadlines
// from. Wails v2 cannot inject a context.Context into a bound method without
// exposing it as a JavaScript argument, so every API embeds this instead.
type apiContext struct {
	ctx context.Context
}

func newAPIContext() apiContext {
	return apiContext{ctx: context.Background()}
}

func (a *apiContext) useContext(ctx context.Context) {
	a.ctx = ctx
}

func (a *apiContext) withTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(a.ctx, timeout)
}
