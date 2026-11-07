package nambridge

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/steven52880/nam/ports"
	"gorm.io/gorm"
)

// channelStore implements ports.ChannelStore.
//
// Design choice: writes call the same model functions controller.AddChannel /
// UpdateChannel / DeleteChannel call (Insert/Update/Delete + abilities rebuild),
// replicate controller.validateChannel (unexported, see validate.go) and finish with the
// same cache refreshes (InitChannelCache, ResetProxyClientCache, CancelChannelAffinityForce).
// Only Test drives the real gin handler (controller.TestChannel) because the underlying
// testChannel is unexported and its behaviour must stay identical to the UI button.
type channelStore struct{}

func notFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ports.ErrNotFound
	}
	return err
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func toPort(c *model.Channel) ports.Channel {
	return ports.Channel{
		ID: c.Id, Type: c.Type, Name: c.Name, Key: c.Key, BaseURL: deref(c.BaseURL),
		Models: c.GetModels(), Group: c.Group, Priority: c.GetPriority(), Weight: uint(c.GetWeight()),
		Tag: c.GetTag(), Status: c.Status, TestModel: deref(c.TestModel), ResponseTime: c.ResponseTime,
		TestTime: c.TestTime, Balance: c.Balance, UsedQuota: c.UsedQuota, CreatedTime: c.CreatedTime,
	}
}

func (channelStore) List(ctx context.Context) ([]ports.Channel, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rows, err := model.GetAllChannels(0, 0, true, true)
	if err != nil {
		return nil, err
	}
	out := make([]ports.Channel, 0, len(rows))
	for _, r := range rows {
		out = append(out, toPort(r))
	}
	return out, nil
}

func (channelStore) Get(ctx context.Context, id int) (ports.Channel, error) {
	if err := ctx.Err(); err != nil {
		return ports.Channel{}, err
	}
	c, err := model.GetChannelById(id, true)
	if err != nil {
		return ports.Channel{}, notFound(err)
	}
	return toPort(c), nil
}

func (channelStore) Create(ctx context.Context, in ports.ChannelCreate) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	prio, weight, tag, testModel := in.Priority, in.Weight, in.Tag, in.TestModel
	ch := model.Channel{
		Type: in.Type, Name: in.Name, Key: in.Key, Models: strings.Join(in.Models, ","),
		Group: in.Group, Status: in.Status, Priority: &prio, Weight: &weight, Tag: &tag,
		TestModel: &testModel, CreatedTime: common.GetTimestamp(),
	}
	if in.BaseURL != "" {
		base := in.BaseURL
		ch.BaseURL = &base
	}
	if ch.Group == "" {
		ch.Group = "default"
	}
	if ch.Status == 0 {
		ch.Status = common.ChannelStatusEnabled
	}
	if err := validateChannel(&ch, true); err != nil {
		return 0, err
	}
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&ch).Error; err != nil {
			return err
		}
		return ch.AddAbilities(tx)
	})
	if err != nil {
		return 0, err
	}
	model.InitChannelCache()
	service.ResetProxyClientCache()
	return ch.Id, nil
}

func applyPatch(dst *model.Channel, p ports.ChannelPatch) error {
	if p.Name != nil {
		dst.Name = *p.Name
	}
	if p.Key != nil {
		dst.Key = *p.Key
	}
	if p.BaseURL != nil {
		v := *p.BaseURL
		dst.BaseURL = &v
	}
	if p.Models != nil {
		dst.Models = strings.Join(*p.Models, ",")
	}
	if p.Group != nil {
		dst.Group = *p.Group
	}
	if p.Priority != nil {
		v := *p.Priority
		dst.Priority = &v
	}
	if p.Weight != nil {
		v := *p.Weight
		dst.Weight = &v
	}
	if p.Tag != nil {
		v := *p.Tag
		dst.Tag = &v
	}
	if p.Status != nil {
		if *p.Status == 0 {
			return errors.New("status 0 is never written")
		}
		dst.Status = *p.Status
	}
	if p.TestModel != nil {
		v := *p.TestModel
		dst.TestModel = &v
	}
	return nil
}

func (channelStore) Update(ctx context.Context, id int, p ports.ChannelPatch) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	origin, err := model.GetChannelById(id, true)
	if err != nil {
		return notFound(err)
	}
	merged := *origin
	if err := applyPatch(&merged, p); err != nil {
		return err
	}
	if err := validateChannel(&merged, false); err != nil {
		return err
	}
	// Like controller.UpdateChannel: struct Updates writes only non-zero fields, so a
	// sparse struct carrying just the patched fields (pointers keep explicit zeros) is enough.
	upd := model.Channel{Id: id, ChannelInfo: origin.ChannelInfo}
	if err := applyPatch(&upd, p); err != nil {
		return err
	}
	if err := upd.Update(); err != nil {
		return err
	}
	model.InitChannelCache()
	service.ResetProxyClientCache()
	if _, err := service.CancelChannelAffinityForce(id); err != nil {
		common.SysError("nam: cancel channel affinity failed: " + err.Error())
	}
	return nil
}

func (channelStore) Delete(ctx context.Context, id int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := model.GetChannelById(id, false); err != nil {
		return notFound(err)
	}
	_, _ = service.CancelChannelAffinityForce(id)
	ch := model.Channel{Id: id}
	if err := ch.Delete(); err != nil {
		return err
	}
	model.InitChannelCache()
	return nil
}

type testEnvelope struct {
	Success bool    `json:"success"`
	Message string  `json:"message"`
	Time    float64 `json:"time"`
}

// Test runs controller.TestChannel through a synthetic gin context and parses its envelope.
func (channelStore) Test(ctx context.Context, id int, modelName string) (ports.ChannelTestResult, error) {
	if err := ctx.Err(); err != nil {
		return ports.ChannelTestResult{}, err
	}
	if _, err := model.GetChannelById(id, false); err != nil {
		return ports.ChannelTestResult{}, notFound(err)
	}
	q := url.Values{}
	if modelName != "" {
		q.Set("model", modelName)
	}
	rec := httptest.NewRecorder()
	gc, _ := gin.CreateTestContext(rec)
	gc.Request = httptest.NewRequest(http.MethodGet, "/nam-internal/channel/test?"+q.Encode(), nil).WithContext(ctx)
	gc.Params = gin.Params{{Key: "id", Value: strconv.Itoa(id)}}
	if err := protect("channel-test", func() error { controller.TestChannel(gc); return nil }); err != nil {
		return ports.ChannelTestResult{}, err
	}
	var env testEnvelope
	if err := common.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		return ports.ChannelTestResult{}, fmt.Errorf("parse test response (HTTP %d): %w", rec.Code, err)
	}
	return ports.ChannelTestResult{Success: env.Success, Message: env.Message, Duration: env.Time}, nil
}
