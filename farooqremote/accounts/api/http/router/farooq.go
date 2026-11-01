package router

import (
	"github.com/gin-gonic/gin"
	"github.com/lejianwen/rustdesk-api/v2/http/controller/admin"
)

// FarooqSignupBind - Farooq Remote (FA-020): sign-up and "forgot password" by an e-mail code. These routes are open
// (bound before the admin login check), like the upstream /user/register.
func FarooqSignupBind(rg *gin.RouterGroup) {
	cont := &admin.FarooqSignup{}
	g := rg.Group("/fr")
	g.GET("/options", cont.Options)
	g.GET("/captcha", cont.Captcha)
	g.POST("/signup/code", cont.SignupCode)
	g.POST("/signup/finish", cont.SignupFinish)
	g.POST("/password/code", cont.PasswordCode)
	g.POST("/password/reset", cont.PasswordReset)
}
