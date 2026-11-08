package core

import (
	"context"
	"errors"
	"fmt"

	"github.com/steven52880/nam/internal/upstream"
)

func syncJob(c *Core, what ...string) func(context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		msg, failed := c.syncer.SyncAll(ctx, what...)
		if failed > 0 {
			return msg, errors.New(msg)
		}
		return msg, nil
	}
}

func (c *Core) balanceJob(ctx context.Context) (string, error) {
	return syncJob(c, upstream.WhatBalance)(ctx)
}

func (c *Core) ratioJob(ctx context.Context) (string, error) {
	return syncJob(c, upstream.WhatTokens, upstream.WhatGroups)(ctx)
}

func (c *Core) consistencyJob(ctx context.Context) (string, error) {
	rep, err := c.channels.CheckDrift(ctx)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("检查 %d 个，漂移 %d 个，自动禁用 %d 个", rep.Checked, len(rep.Drifted), len(rep.AutoDisabled)), nil
}
