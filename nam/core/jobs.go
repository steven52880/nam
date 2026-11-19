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

// ratioJob syncs tokens + group ratios/prices into nam.db. Channel discounts are computed
// from that cache whenever they are read, so nothing is derived or stored here.
func (c *Core) ratioJob(ctx context.Context) (string, error) {
	return syncJob(c, upstream.WhatTokens, upstream.WhatGroups)(ctx)
}

// consistencyJob is the rule check: validateRows over every channel (drafts overlaid) and the
// channels NewAPI auto-disabled. Read only.
func (c *Core) consistencyJob(ctx context.Context) (string, error) {
	rep, err := c.channels.Check(ctx)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("检查 %d 个渠道，规则问题 %d 条，自动禁用 %d 个", rep.Checked, len(rep.Issues), len(rep.AutoDisabled)), nil
}
