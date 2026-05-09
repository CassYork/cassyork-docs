package adminui

import "net/http"

const (
	cookieOrgIDKey  = "cassyork_admin_org_id"
	cookieProjIDKey = "cassyork_admin_project_id"
)

func cookieString(r *http.Request, name string) string {
	c, err := r.Cookie(name)
	if err != nil || c == nil || c.Value == "" {
		return ""
	}
	return c.Value
}
