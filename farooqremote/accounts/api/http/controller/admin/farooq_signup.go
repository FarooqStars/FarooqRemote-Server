package admin

// Farooq Remote (FA-020, owner 1 Nov 2026): anyone may make an account with an e-mail address and a password; the
// account works as soon as the e-mail is proved with a 6-digit code. "Forgot password" works the same way. The
// administrator's own account is never reset by e-mail (he uses FR-09 on the server).
//
// Routes (bound by http/router/farooq.go, before the admin login check):
//
//	GET  /api/admin/fr/options         -> {signup, min_password, max_password}
//	GET  /api/admin/fr/captcha         -> {captcha: {id, b64}}
//	POST /api/admin/fr/signup/code     {email, captcha_id, captcha, lang}
//	POST /api/admin/fr/signup/finish   {email, code, password, lang}  -> logged in, like the upstream register
//	POST /api/admin/fr/password/code   {email, captcha_id, captcha, lang}
//	POST /api/admin/fr/password/reset  {email, code, password, lang}

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/http/response"
	"github.com/lejianwen/rustdesk-api/v2/lib/frsignup"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

type FarooqSignup struct {
}

type frCodeForm struct {
	Email     string `json:"email"`
	CaptchaId string `json:"captcha_id"`
	Captcha   string `json:"captcha"`
	Lang      string `json:"lang"`
}

type frFinishForm struct {
	Email    string `json:"email"`
	Code     string `json:"code"`
	Password string `json:"password"`
	Lang     string `json:"lang"`
}

// frMask keeps the log readable without writing out whole addresses: "fa***@example.com".
func frMask(email string) string {
	at := strings.LastIndex(email, "@")
	if at < 0 {
		return "***"
	}
	name := email[:at]
	if len(name) > 2 {
		name = name[:2]
	}
	return name + "***" + email[at:]
}

// frFindUser finds the account of an address: by user name (accounts made by sign-up) or by the e-mail field.
func frFindUser(email string) *model.User {
	u := service.AllService.UserService.InfoByUsername(email)
	if u != nil && u.Id != 0 {
		return u
	}
	u = service.AllService.UserService.InfoByEmail(email)
	if u == nil {
		return &model.User{}
	}
	return u
}

func frOpen(c *gin.Context) bool {
	if !frsignup.Ready() {
		response.Fail(c, 101, response.TranslateMsg(c, "FrMailNotReady"))
		return false
	}
	if banned, _ := global.LoginLimiter.CheckSecurityStatus(c.ClientIP()); banned {
		response.Fail(c, 101, response.TranslateMsg(c, "LoginBanned"))
		return false
	}
	return true
}

// frCaptchaOK checks the picture code; a wrong one counts as a failed try for this network address.
func frCaptchaOK(c *gin.Context, f *frCodeForm) bool {
	ll := global.LoginLimiter
	if f.CaptchaId == "" || f.Captcha == "" || !ll.VerifyCaptcha(f.CaptchaId, strings.TrimSpace(f.Captcha)) {
		ll.RecordFailedAttempt(c.ClientIP())
		response.Fail(c, 101, response.TranslateMsg(c, "CaptchaError"))
		return false
	}
	return true
}

func (ct *FarooqSignup) Options(c *gin.Context) {
	response.Success(c, gin.H{
		"signup":       frsignup.Ready(),
		"min_password": frsignup.MinPassword,
		"max_password": frsignup.MaxPassword,
	})
}

func (ct *FarooqSignup) Captcha(c *gin.Context) {
	ll := global.LoginLimiter
	if banned, _ := ll.CheckSecurityStatus(c.ClientIP()); banned {
		response.Fail(c, 101, response.TranslateMsg(c, "LoginBanned"))
		return
	}
	err, cp := ll.RequireCaptcha()
	if err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "CaptchaError"))
		return
	}
	err, b64 := ll.DrawCaptcha(cp.Content)
	if err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "CaptchaError"))
		return
	}
	frsignup.WriteTest("captcha-"+cp.Id+".txt", cp.Answer)
	response.Success(c, gin.H{"captcha": gin.H{"id": cp.Id, "b64": b64}})
}

func (ct *FarooqSignup) SignupCode(c *gin.Context) {
	if !frOpen(c) {
		return
	}
	f := &frCodeForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
		return
	}
	if !frCaptchaOK(c, f) {
		return
	}
	email, ok := frsignup.NormalizeEmail(f.Email)
	if !ok {
		response.Fail(c, 101, response.TranslateMsg(c, "FrEmailInvalid"))
		return
	}
	ip := c.ClientIP()
	if !frsignup.Default.Allow(email, ip, time.Now()) {
		response.Fail(c, 101, response.TranslateMsg(c, "FrTooMany"))
		return
	}
	lang := frsignup.Lang(f.Lang, c.GetHeader("Accept-Language"))
	var subject, text, html string
	if frFindUser(email).Id != 0 {
		// the address already has an account: tell its owner instead of making a second account
		subject, text, html = frsignup.Mail("exists", lang, "")
	} else {
		code, err := frsignup.Default.NewCode(frsignup.PurposeSign, email, time.Now())
		if err != nil {
			response.Fail(c, 101, response.TranslateMsg(c, "OperationFailed"))
			return
		}
		subject, text, html = frsignup.Mail(frsignup.PurposeSign, lang, code)
	}
	if err := frsignup.Send(email, subject, text, html); err != nil {
		global.Logger.Error("FR sign-up mail to " + frMask(email) + " failed: " + err.Error())
		response.Fail(c, 101, response.TranslateMsg(c, "FrMailFailed"))
		return
	}
	global.Logger.Info("FR sign-up code sent to " + frMask(email) + " from " + ip)
	response.Success(c, gin.H{"message": response.TranslateMsg(c, "FrCodeSent")})
}

func (ct *FarooqSignup) SignupFinish(c *gin.Context) {
	if !frOpen(c) {
		return
	}
	f := &frFinishForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
		return
	}
	email, ok := frsignup.NormalizeEmail(f.Email)
	if !ok {
		response.Fail(c, 101, response.TranslateMsg(c, "FrEmailInvalid"))
		return
	}
	if !frsignup.PasswordOK(f.Password) {
		response.Fail(c, 101, response.TranslateMsg(c, "FrPasswordRule"))
		return
	}
	ip := c.ClientIP()
	if !frsignup.Default.Check(frsignup.PurposeSign, email, f.Code, time.Now()) {
		global.LoginLimiter.RecordFailedAttempt(ip)
		response.Fail(c, 101, response.TranslateMsg(c, "FrCodeWrong"))
		return
	}
	if frFindUser(email).Id != 0 {
		response.Fail(c, 101, response.TranslateMsg(c, "FrEmailTaken"))
		return
	}
	u := service.AllService.UserService.Register(email, email, f.Password, model.COMMON_STATUS_ENABLE)
	if u == nil || u.Id == 0 {
		response.Fail(c, 101, response.TranslateMsg(c, "OperationFailed"))
		return
	}
	global.Logger.Info("FR sign-up: new account " + frMask(email) + " from " + ip)
	ut := service.AllService.UserService.Login(u, &model.LoginLog{
		UserId: u.Id,
		Client: model.LoginLogClientWebAdmin,
		Uuid:   "",
		Ip:     ip,
		Type:   model.LoginLogTypeAccount,
	})
	responseLoginSuccess(c, u, ut.Token)
}

func (ct *FarooqSignup) PasswordCode(c *gin.Context) {
	if !frOpen(c) {
		return
	}
	f := &frCodeForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
		return
	}
	if !frCaptchaOK(c, f) {
		return
	}
	email, ok := frsignup.NormalizeEmail(f.Email)
	if !ok {
		response.Fail(c, 101, response.TranslateMsg(c, "FrEmailInvalid"))
		return
	}
	ip := c.ClientIP()
	if !frsignup.Default.Allow(email, ip, time.Now()) {
		response.Fail(c, 101, response.TranslateMsg(c, "FrTooMany"))
		return
	}
	lang := frsignup.Lang(f.Lang, c.GetHeader("Accept-Language"))
	us := service.AllService.UserService
	u := frFindUser(email)
	if u.Id != 0 && us.CheckUserEnable(u) && !us.IsAdmin(u) {
		code, err := frsignup.Default.NewCode(frsignup.PurposeReset, email, time.Now())
		if err == nil {
			subject, text, html := frsignup.Mail(frsignup.PurposeReset, lang, code)
			// sent in the background, so the answer is the same whether the address has an account or not
			go func() {
				if err := frsignup.Send(email, subject, text, html); err != nil {
					global.Logger.Error("FR password mail to " + frMask(email) + " failed: " + err.Error())
				}
			}()
			global.Logger.Info("FR password code sent to " + frMask(email) + " from " + ip)
		}
	}
	response.Success(c, gin.H{"message": response.TranslateMsg(c, "FrCodeSentIfAccount")})
}

func (ct *FarooqSignup) PasswordReset(c *gin.Context) {
	if !frOpen(c) {
		return
	}
	f := &frFinishForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
		return
	}
	email, ok := frsignup.NormalizeEmail(f.Email)
	if !ok {
		response.Fail(c, 101, response.TranslateMsg(c, "FrEmailInvalid"))
		return
	}
	if !frsignup.PasswordOK(f.Password) {
		response.Fail(c, 101, response.TranslateMsg(c, "FrPasswordRule"))
		return
	}
	ip := c.ClientIP()
	if !frsignup.Default.Check(frsignup.PurposeReset, email, f.Code, time.Now()) {
		global.LoginLimiter.RecordFailedAttempt(ip)
		response.Fail(c, 101, response.TranslateMsg(c, "FrCodeWrong"))
		return
	}
	us := service.AllService.UserService
	u := frFindUser(email)
	if u.Id == 0 || !us.CheckUserEnable(u) || us.IsAdmin(u) {
		response.Fail(c, 101, response.TranslateMsg(c, "FrCodeWrong"))
		return
	}
	if err := us.UpdatePassword(u, f.Password); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "OperationFailed"))
		return
	}
	global.Logger.Info("FR password changed by e-mail code: " + frMask(email) + " from " + ip)
	response.Success(c, gin.H{"message": response.TranslateMsg(c, "FrPasswordChanged")})
}
