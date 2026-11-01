package api

// Farooq Remote (FA-020, owner 1 Nov 2026): «easy access» like TeamViewer's - see lib/freasy for the whole picture.
//
//	POST /api/fr/easy/ticket  (signed in)  {"id": target, "from": my id}   -> {"easy": true, "ticket": "…"} | {"easy": false}
//	POST /api/fr/easy/check   (no sign-in) {"ticket": "…", "id": my id}    -> {"ok": true, "user": "…", "from": "…"} | {"ok": false}
//
// The check needs no sign-in on purpose: the computer being connected to runs it from its service, which does not hold
// the account's token; the ticket itself (32 random bytes, 60 seconds, one use) is the secret, and that computer accepts
// only its own stored owner and the computer that connected.

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lejianwen/rustdesk-api/v2/lib/freasy"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

type FarooqEasy struct{}

type frEasyTicketForm struct {
	Id   string `json:"id"`
	From string `json:"from"`
}

type frEasyCheckForm struct {
	Ticket string `json:"ticket"`
	Id     string `json:"id"`
}

// Ticket gives a one-time ticket to the signed-in account, for one of its computers that has easy access on.
func (ct *FarooqEasy) Ticket(c *gin.Context) {
	f := &frEasyTicketForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
		return
	}
	f.Id, f.From = strings.TrimSpace(f.Id), strings.TrimSpace(f.From)
	u := service.AllService.UserService.CurUser(c)
	if u == nil || u.Id == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	now := time.Now()
	if !freasy.Default.Announced(f.Id, now) {
		c.JSON(http.StatusOK, gin.H{"easy": false})
		return
	}
	// a computer bound to another account is never offered (the computer itself also checks its stored owner)
	if p := service.AllService.PeerService.FindById(f.Id); p != nil && p.RowId > 0 && p.UserId != 0 && p.UserId != u.Id {
		c.JSON(http.StatusOK, gin.H{"easy": false})
		return
	}
	t, err := freasy.Default.Issue(u.Id, u.Username, f.From, f.Id, now)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"easy": false})
		return
	}
	c.JSON(http.StatusOK, gin.H{"easy": true, "ticket": t})
}

// Check answers once about a ticket presented to computer id.
func (ct *FarooqEasy) Check(c *gin.Context) {
	f := &frEasyCheckForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
		return
	}
	user, from, ok := freasy.Default.Check(strings.TrimSpace(f.Ticket), strings.TrimSpace(f.Id), time.Now())
	if !ok {
		c.JSON(http.StatusOK, gin.H{"ok": false})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "user": user, "from": from})
}
