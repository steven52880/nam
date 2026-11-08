package core

import (
	"context"
	"errors"
	"fmt"

	"github.com/steven52880/nam/internal/pricing"
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

// ratioJob syncs tokens + group ratios/prices, then re-derives every auto 上游分组折扣.
// Manual discounts are never changed. Writes nam.db only.
func (c *Core) ratioJob(ctx context.Context) (string, error) {
	msg, failed := c.syncer.SyncAll(ctx, upstream.WhatTokens, upstream.WhatGroups)
	dmsg, err := c.refreshDiscounts(ctx)
	if err != nil {
		return msg, fmt.Errorf("%s；折扣推算失败：%w", msg, err)
	}
	msg = msg + "；" + dmsg
	if failed > 0 {
		return msg, errors.New(msg)
	}
	return msg, nil
}

func (c *Core) refreshDiscounts(ctx context.Context) (string, error) {
	in, err := c.channels.PricingInputs(ctx)
	if err != nil {
		return "", err
	}
	res, _, err := pricing.RefreshAuto(ctx, c.store, in)
	if err != nil {
		return "", err
	}
	// TODO(M6): notify 价格变动 with the returned changes.
	return fmt.Sprintf("折扣更新 %d 个，手动 %d 个，无法推算 %d 个", len(res.Updated), res.Manual, len(res.Underived)), nil
}

func (c *Core) consistencyJob(ctx context.Context) (string, error) {
	rep, err := c.channels.CheckDrift(ctx)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("检查 %d 个，漂移 %d 个，自动禁用 %d 个", rep.Checked, len(rep.Drifted), len(rep.AutoDisabled)), nil
}
