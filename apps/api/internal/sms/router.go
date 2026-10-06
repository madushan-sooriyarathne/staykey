// Package sms delivers text messages through the gateway that reaches each number.
package sms

import (
	"context"
	"strings"

	"staykey.direct/api/internal/auth"
	"staykey.direct/api/internal/domain"
)

// Router sends each message through the gateway for the number's country. Only Sri Lankan
// numbers have a gateway so far; an international sender gets its own branch here once one
// exists.
type Router struct {
	LK auth.SMSSender
}

// Send passes Sri Lankan numbers (E.164, +94) to LK and refuses every other country.
//
// ponytail: auth.RequestCode stores the challenge before Send runs, so a refused number still
// counts towards the resend and rate limits, and a repeat within 30 seconds gets "try again"
// instead of this message. Add a Reaches(phone) check before RequestCode's transaction if that
// starts to confuse people.
func (r Router) Send(ctx context.Context, to, body string) error {
	if !strings.HasPrefix(to, "+94") {
		return &domain.ValidationError{Field: "phone", Message: "international numbers are not supported yet"}
	}
	return r.LK.Send(ctx, to, body)
}
