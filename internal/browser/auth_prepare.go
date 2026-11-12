package browser

import (
	"context"

	"ghostfish/internal/auth"
)

// prepareAuth 在后台探测并查找凭据，不在验证成功之前保存。
func (a *Browser) prepareAuth(ctx context.Context, rawURL string) *preparedAuth {
	if rawURL == "" || ctx.Err() != nil {
		return nil
	}
	need := auth.Probe(ctx, rawURL)
	if need == nil || ctx.Err() != nil {
		return nil
	}
	cred := a.resolveAuth(ctx, need)
	if cred == nil || ctx.Err() != nil {
		return &preparedAuth{need: *need, url: rawURL}
	}
	ok, _, err := a.verifyAuth(ctx, rawURL, cred)
	if ctx.Err() != nil {
		return nil
	}
	return &preparedAuth{need: *need, url: rawURL, credential: cred, verified: err == nil && ok}
}
