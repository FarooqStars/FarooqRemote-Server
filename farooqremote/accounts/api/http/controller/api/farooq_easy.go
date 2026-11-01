package api

// Farooq Remote (FA-020, owner 1 Nov 2026): «easy access» like TeamViewer's - see lib/freasy for the whole picture.
//
//	POST /api/fr/easy/ticket  (signed in)  {"id": target, "from": my id, "pk": target key}
//	                                        -> {"easy": true, "ticket": "…"} | {"easy": false}
//	POST /api/fr/easy/check   (no sign-in) {"ticket": "…", "id": my id}
//	                                        -> {"ok": true, "user": "…", "from": "…", "pk": "…"} | {"ok": false}
//
// The check needs no sign-in on purpose: the computer being connected to runs it from its service, which does not hold
// the account's token; the ticket itself (32 random bytes, 60 seconds, one use) is the secret, and that computer accepts
// only its own stored owner, the computer that connected and its own key.
//
// 2.7-10: a ticket only for a computer signed in to the same account (logging out on it ends easy access until the next
// login), the target's key travels with the ticket, and every ticket request counts in the account's limit.

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
	Pk   string `json:"pk"`
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
	f.Id, f.From, f.Pk = strings.TrimSpace(f.Id), strings.TrimSpace(f.From), strings.TrimSpace(f.Pk)
	u := service.AllService.UserService.CurUser(c)
	if u == nil || u.Id == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	no := func() { c.JSON(http.StatusOK, gin.H{"easy": false}) }
	now := time.Now()
	// every request counts, whatever the answer (the same answer for every refusal)
	if !freasy.Default.Spend(u.Id, now) || !freasy.Default.Announced(f.Id, now) {
		no()
		return
	}
	// only a computer signed in to this same account (the computer itself also checks its stored owner)
	if p := service.AllService.PeerService.FindById(f.Id); p == nil || p.RowId == 0 || p.UserId != u.Id {
		no()
		return
	}
	t, err := freasy.Default.Issue(u.Id, u.Username, f.From, f.Id, f.Pk, now)
	if err != nil {
		no()
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
	user, from, pk, ok := freasy.Default.Check(strings.TrimSpace(f.Ticket), strings.TrimSpace(f.Id), time.Now())
	if !ok {
		c.JSON(http.StatusOK, gin.H{"ok": false})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "user": user, "from": from, "pk": pk})
}
