package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/microsoft/kiota-abstractions-go/authentication"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
	"github.com/upbound/function-msgraph/input/v1beta1"
	"k8s.io/utils/ptr"

	"github.com/crossplane/function-sdk-go/logging"
)

// Exercise the actual SDK request and response path without Azure credentials or
// calls to Microsoft Graph. The returned guest's mail deliberately differs
// from its resource-tenant UPN.
const testGuestMail = "guest@example.com"

func TestValidateUsersLookupField(t *testing.T) {
	guest := `{"id":"guest-id","displayName":"Guest","mail":"guest@example.com","userPrincipalName":"guest_example.com#EXT#@tenant.onmicrosoft.com","accountEnabled":true}`
	cases := map[string]struct {
		field      string
		identifier string
		filter     string
		body       string
		nextPage   bool
		wantCount  int
		wantError  string
	}{
		"DefaultUPN": {
			identifier: testGuestMail, filter: "userPrincipalName eq 'guest@example.com'", body: guest, wantCount: 1,
		},
		"ExplicitUPN": {
			field: fieldUserPrincipalName, identifier: testGuestMail, filter: "userPrincipalName eq 'guest@example.com'", body: guest, wantCount: 1,
		},
		"MailForB2BGuest": {
			field: fieldMail, identifier: testGuestMail, filter: "mail eq 'guest@example.com'", body: guest, wantCount: 1,
		},
		"EscapesMailLiteral": {
			field: fieldMail, identifier: "o'connor@example.com", filter: "mail eq 'o''connor@example.com'", body: guest, wantCount: 1,
		},
		"EscapesUPNLiteral": {
			identifier: "o'connor@example.com", filter: "userPrincipalName eq 'o''connor@example.com'", body: guest, wantCount: 1,
		},
		"NoMatchPreservesEmptyResult": {
			field: fieldMail, identifier: "missing@example.com", filter: "mail eq 'missing@example.com'",
		},
		"AmbiguousMail": {
			field: fieldMail, identifier: testGuestMail, filter: "mail eq 'guest@example.com'", body: guest + "," + guest, wantError: "matched multiple users",
		},
		"PaginatedMailIsNotProvenUnique": {
			field: fieldMail, identifier: testGuestMail, filter: "mail eq 'guest@example.com'", body: guest, nextPage: true, wantError: "matched multiple users",
		},
		"InvalidField": {
			field: "displayName", identifier: "Guest", wantError: "unsupported userLookupField",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if got := r.URL.Query().Get("$filter"); got != tc.filter {
					t.Errorf("filter: got %q, want %q", got, tc.filter)
				}
				if r.Method != http.MethodGet || r.URL.Path != "/users" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				body := `{"value":[` + tc.body + `]`
				if tc.nextPage {
					body += `,"@odata.nextLink":"https://graph.microsoft.com/v1.0/users?$skiptoken=next"`
				}
				if _, err := io.WriteString(w, body+`}`); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			adapter, err := msgraphsdk.NewGraphRequestAdapter(&authentication.AnonymousAuthenticationProvider{})
			if err != nil {
				t.Fatal(err)
			}
			adapter.SetBaseUrl(server.URL)
			client := msgraphsdk.NewGraphServiceClient(adapter)
			g := &GraphQuery{log: logging.NewNopLogger()}
			result, err := g.validateUsers(context.Background(), client, &v1beta1.Input{
				Users: []*string{ptr.To(tc.identifier)}, UserLookupField: tc.field,
			})
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("got error %v, want %q", err, tc.wantError)
				}
				if tc.field == "displayName" && calls != 0 {
					t.Fatal("invalid lookup field must fail before any Graph request")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			rows, ok := result.([]interface{})
			if !ok || len(rows) != tc.wantCount {
				t.Fatalf("got %#v, want %d users", result, tc.wantCount)
			}
			if calls != 1 {
				t.Fatalf("got %d Graph requests, want 1", calls)
			}
			if tc.wantCount == 1 {
				want := map[string]interface{}{
					"id": "guest-id", "displayName": "Guest", fieldMail: testGuestMail,
					"userPrincipalName": "guest_example.com#EXT#@tenant.onmicrosoft.com", "accountEnabled": true,
				}
				if diff := cmp.Diff(want, rows[0]); diff != "" {
					t.Errorf("result mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}
