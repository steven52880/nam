package standalone

import (
	"context"
	"net/http"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

const minPasswordLen = 8

// hashLocked returns the stored admin hash, seeding it from NAM_ADMIN_PASSWORD on first
// use. An empty result means no password is configured. Caller holds a.mu.
func (a *Auth) hashLocked(ctx context.Context) (string, error) {
	if a.kv == nil {
		if a.memHash == "" && a.initialPW != "" {
			h, err := hashPassword(a.initialPW)
			if err != nil {
				return "", err
			}
			a.memHash = h
		}
		return a.memHash, nil
	}
	v, ok, err := a.kv.GetKV(ctx, kvHashKey)
	if err != nil {
		return "", err
	}
	if ok && v != "" {
		return v, nil
	}
	if a.initialPW == "" {
		return "", nil
	}
	h, err := hashPassword(a.initialPW)
	if err != nil {
		return "", err
	}
	if err := a.kv.SetKV(ctx, kvHashKey, h); err != nil {
		return "", err
	}
	return h, nil
}

func (a *Auth) storeHashLocked(ctx context.Context, h string) error {
	if a.kv == nil {
		a.memHash = h
		return nil
	}
	return a.kv.SetKV(ctx, kvHashKey, h)
}

func fail(c *gin.Context, status int, msg string, data any) {
	c.AbortWithStatusJSON(status, gin.H{"success": false, "message": msg, "data": data})
}

func (a *Auth) lockInfo() gin.H { return gin.H{"failures": a.failures, "locked_until": a.lockedUntil} }

// RegisterPublicRoutes mounts login and logout (no session required).
func (a *Auth) RegisterPublicRoutes(g *gin.RouterGroup) {
	g.POST("/auth/login", a.login)
	g.POST("/auth/logout", a.logout)
}

// RegisterPrivateRoutes mounts routes that need a session.
func (a *Auth) RegisterPrivateRoutes(g *gin.RouterGroup) {
	g.POST("/auth/password", a.changePassword)
}

func (a *Auth) login(c *gin.Context) {
	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "无效的参数", nil)
		return
	}
	ctx := c.Request.Context()
	a.mu.Lock()
	defer a.mu.Unlock() // held across argon2 so concurrent guesses cannot dodge the lockout
	now := a.now().Unix()
	if a.lockedUntil > now {
		fail(c, http.StatusTooManyRequests, "登录失败次数过多，请稍后再试", a.lockInfo())
		return
	}
	if a.lockedUntil != 0 { // lock expired: start over
		a.failures, a.lockedUntil = 0, 0
	}
	hash, err := a.hashLocked(ctx)
	if err != nil {
		fail(c, http.StatusInternalServerError, "读取管理员密码失败: "+err.Error(), nil)
		return
	}
	if hash == "" {
		fail(c, http.StatusUnauthorized, "未设置管理员密码，请配置 NAM_ADMIN_PASSWORD 后重启", nil)
		return
	}
	match, err := verifyPassword(req.Password, hash)
	if err != nil {
		fail(c, http.StatusInternalServerError, "管理员密码哈希无效: "+err.Error(), nil)
		return
	}
	if !match {
		a.failures++
		if a.failures >= maxFailures {
			a.lockedUntil = now + int64(lockDuration.Seconds())
		}
		fail(c, http.StatusUnauthorized, "密码错误", a.lockInfo())
		return
	}
	a.failures, a.lockedUntil = 0, 0
	token, err := a.newSessionLocked(ctx)
	if err != nil {
		fail(c, http.StatusInternalServerError, "创建会话失败", nil)
		return
	}
	a.setCookie(c, token, int(sessionTTL.Seconds()))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{
		"user_id": a.userID, "username": adminUsername, "role": rootRole,
	}})
}

func (a *Auth) logout(c *gin.Context) {
	if token, err := c.Cookie(cookieName); err == nil && token != "" {
		ctx := c.Request.Context()
		a.mu.Lock()
		a.loadLocked(ctx)
		delete(a.sessions, tokenKey(token))
		a.persistLocked(ctx)
		a.mu.Unlock()
	}
	a.setCookie(c, "", -1)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func (a *Auth) changePassword(c *gin.Context) {
	var req struct {
		Old     string `json:"old_password"`
		New     string `json:"new_password"`
		Confirm string `json:"confirm_password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "无效的参数", nil)
		return
	}
	if utf8.RuneCountInString(req.New) < minPasswordLen {
		fail(c, http.StatusBadRequest, "新密码至少 8 位", nil)
		return
	}
	if req.New != req.Confirm {
		fail(c, http.StatusBadRequest, "两次输入的新密码不一致", nil)
		return
	}
	ctx := c.Request.Context()
	a.mu.Lock()
	defer a.mu.Unlock()
	hash, err := a.hashLocked(ctx)
	if err != nil {
		fail(c, http.StatusInternalServerError, "读取管理员密码失败: "+err.Error(), nil)
		return
	}
	match, err := verifyPassword(req.Old, hash)
	if err != nil || !match {
		fail(c, http.StatusUnauthorized, "原密码错误", nil)
		return
	}
	nh, err := hashPassword(req.New)
	if err == nil {
		err = a.storeHashLocked(ctx, nh)
	}
	if err != nil {
		fail(c, http.StatusInternalServerError, "保存新密码失败: "+err.Error(), nil)
		return
	}
	a.loadLocked(ctx)
	a.sessions = map[string]int64{} // every session, including this one, must log in again
	a.persistLocked(ctx)
	a.setCookie(c, "", -1)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "密码已修改，请重新登录"})
}
