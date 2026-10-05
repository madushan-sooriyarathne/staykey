package server

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/oapi"
	"staykey.direct/api/internal/tenant"
)

// ownerRoute is an operation from the contract that acts on an account.
type ownerRoute struct {
	method string
	path   string // as written in the spec, with {params}
	params []string
}

// ownerRoutes lists every operation whose security includes accountHeader, so a new owner route
// is covered by the isolation tests the moment it is added to the spec.
func ownerRoutes(t *testing.T) []ownerRoute {
	t.Helper()
	spec, err := oapi.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	reqs, err := requirementsFrom(spec)
	if err != nil {
		t.Fatal(err)
	}
	var out []ownerRoute
	for path, item := range spec.Paths.Map() {
		for method, op := range item.Operations() {
			if !reqs[method+" "+path].account {
				continue
			}
			r := ownerRoute{method: method, path: path}
			for _, p := range append(append(openapi3.Parameters{}, item.Parameters...), op.Parameters...) {
				if p.Value != nil && p.Value.In == openapi3.ParameterInPath {
					r.params = append(r.params, p.Value.Name)
				}
			}
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].method+out[i].path < out[j].method+out[j].path })
	return out
}

func (r ownerRoute) String() string { return r.method + " " + r.path }

// fill substitutes path parameters from ids, failing when a route uses one the test doesn't know.
func (r ownerRoute) fill(t *testing.T, ids map[string]string) string {
	t.Helper()
	path := r.path
	for _, name := range r.params {
		id, ok := ids[name]
		if !ok {
			t.Fatalf("%s: no isolation fixture for path parameter {%s}; add one to TestTenantIsolation", r, name)
		}
		path = strings.ReplaceAll(path, "{"+name+"}", id)
	}
	return path
}

// TestTenantIsolation checks every owner route in the contract: another account's member gets
// 404, whether they name the other account or point their own account at its records.
func TestTenantIsolation(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	alice, aliceAccount := e.owner("+94773000001", "Alice's villas")
	bob, bobAccount := e.owner("+94773000002", "Bob's guesthouse")
	aliceVilla := e.createProperty(alice, aliceAccount, "Alice Villa")
	bobHouse := e.createProperty(bob, bobAccount, "Bob House")
	aliceStay := e.createBooking(alice, aliceAccount, aliceVilla, "2026-11-02", "2026-11-04")
	rec := e.do(call{method: http.MethodPost, path: "/v1/blocks", as: alice, account: aliceAccount, body: map[string]any{
		"propertyId": aliceVilla.Id, "unitIds": []any{aliceVilla.Units[0].Id}, "from": "2026-12-01", "to": "2026-12-03", "reason": "owner",
	}})
	e.expect(rec, http.StatusCreated)
	aliceBlock := decode[oapi.Block](t, rec)
	aliceTenant := tenant.Tenant{AccountID: uuid.MustParse(aliceAccount), UserID: alice.userID, Role: domain.RoleOwner}
	aliceSlip, err := e.store.AddSlip(context.Background(), aliceTenant, aliceStay.Id, "accounts/slip.jpg", 5000, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	// Alice's records, by the path parameter names routes use for them. Later phases add units,
	// team members and so on here as their routes arrive.
	aliceIDs := map[string]string{
		"id":         aliceVilla.Id.String(),
		"propertyId": aliceVilla.Id.String(),
		"bookingId":  aliceStay.Id.String(),
		"blockId":    aliceBlock.Id.String(),
		"slipId":     aliceSlip.String(),
	}

	routes := ownerRoutes(t)
	if len(routes) == 0 {
		t.Fatal("no owner routes found in the contract")
	}
	for _, r := range routes {
		t.Run(r.String(), func(t *testing.T) {
			path := r.fill(t, aliceIDs)
			body := []byte("{}")

			// Bob names Alice's account.
			rec := e.do(call{method: r.method, path: path, raw: body, as: bob, account: aliceAccount})
			if rec.Code != http.StatusNotFound {
				t.Fatalf("Bob with Alice's account: status %d, want 404 (body %s)", rec.Code, rec.Body)
			}
			if got := decode[oapi.Error](t, rec).Code; got != "account_not_found" {
				t.Errorf("Bob with Alice's account: code %q, want account_not_found", got)
			}

			// Bob uses his own account but points at Alice's records.
			if len(r.params) > 0 {
				rec = e.do(call{method: r.method, path: path, raw: body, as: bob, account: bobAccount})
				if rec.Code != http.StatusNotFound {
					t.Fatalf("Bob's account with Alice's ids: status %d, want 404 (body %s)", rec.Code, rec.Body)
				}
			}

			// No token at all.
			rec = e.do(call{method: r.method, path: path, raw: body, account: aliceAccount})
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("no token: status %d, want 401", rec.Code)
			}
		})
	}

	// And the lists only show each owner their own.
	for _, c := range []struct {
		who     *session
		account string
		want    string
	}{{alice, aliceAccount, aliceVilla.Id.String()}, {bob, bobAccount, bobHouse.Id.String()}} {
		rec := e.do(call{method: http.MethodGet, path: "/v1/properties", as: c.who, account: c.account})
		e.expect(rec, http.StatusOK)
		list := decode[oapi.PropertyList](t, rec)
		if len(list.Items) != 1 || list.Items[0].Id.String() != c.want {
			t.Errorf("list for %s = %v, want only %s", c.account, ids(list.Items), c.want)
		}
	}
}

func ids(items []oapi.Property) []string {
	out := make([]string, len(items))
	for i, p := range items {
		out[i] = fmt.Sprint(p.Id)
	}
	return out
}
