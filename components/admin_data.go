package components

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
)

// UserAccount is a user account as the API returns it.
type UserAccount struct {
	UserID   string   `json:"user_id"`
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
}

// LinkedAccount is a platform linked to an account.
type LinkedAccount struct {
	Platform         string `json:"platform"`
	PlatformUsername string `json:"platform_username"`
	PlatformID       string `json:"platform_id"`
	Verified         bool   `json:"verified"`
	LoginEnabled     bool   `json:"login_enabled"`
}

// Permission is a permission node roles can grant.
type Permission struct {
	ID          string `json:"id"`
	Node        string `json:"node"`
	Description string `json:"description"`
	ValueType   string `json:"value_type"`
	Merge       string `json:"merge"`
}

// RolePermission is a permission as a role grants it.
type RolePermission struct {
	Permission
	Value json.RawMessage `json:"value"`
}

// Role is a named set of permissions.
type Role struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Permissions []RolePermission `json:"permissions"`
}

// ValueText returns the granted value as the text of its input: the number, the text, or one list item per line.
func (p RolePermission) ValueText() string {
	switch p.ValueType {
	case "string":
		var text string
		_ = json.Unmarshal(p.Value, &text)
		return text
	case "string_list":
		var items []string
		_ = json.Unmarshal(p.Value, &items)
		return strings.Join(items, "\n")
	case "int":
		return strings.TrimSpace(string(p.Value))
	default:
		return ""
	}
}

// Label returns the node, followed by its value when the role grants one.
func (p RolePermission) Label() string {
	if p.ValueType == "" {
		return p.Node
	}
	return p.Node + ": " + strings.ReplaceAll(p.ValueText(), "\n", ", ")
}

// AdminDashboardData is what the admin dashboard shows.
type AdminDashboardData struct {
	Users bool
	Roles bool
}

// AdminUsersData is a page of the user list.
type AdminUsersData struct {
	Users      []UserAccount
	RoleNames  map[string]string
	Search     string
	Start      int
	NextOffset int
	More       bool
	FocusFirst bool
}

// AdminUserData is the user editor.
type AdminUserData struct {
	User           UserAccount
	Roles          []Role
	RolesReadable  bool
	Links          []LinkedAccount
	LinksError     string
	Permissions    []string
	PermissionsErr string
}

// AdminRolesData is the role list.
type AdminRolesData struct {
	Roles []Role
}

// AdminRoleData is the role editor.
type AdminRoleData struct {
	Role            Role
	Catalogue       []Permission
	Drafts          map[string]string
	GrantPermission string
	GrantValue      string
	FocusList       bool
}

// AdminPermissionsData is the permission list.
type AdminPermissionsData struct {
	Permissions []Permission
	FocusList   bool
}

// HeldRole reports whether the user holds the role.
func (d AdminUserData) HeldRole(id string) bool {
	for _, held := range d.User.Roles {
		if held == id {
			return true
		}
	}
	return false
}

// UnlistedRoles returns the roles the user holds that are not in the catalogue, which have no checkbox.
func (d AdminUserData) UnlistedRoles() []string {
	listed := make(map[string]bool, len(d.Roles))
	for _, role := range d.Roles {
		listed[role.ID] = true
	}
	var unlisted []string
	for _, held := range d.User.Roles {
		if !listed[held] {
			unlisted = append(unlisted, held)
		}
	}
	return unlisted
}

// Available returns the permissions the role does not grant yet.
func (d AdminRoleData) Available() []Permission {
	granted := make(map[string]bool, len(d.Role.Permissions))
	for _, p := range d.Role.Permissions {
		granted[p.ID] = true
	}
	var available []Permission
	for _, p := range d.Catalogue {
		if !granted[p.ID] {
			available = append(available, p)
		}
	}
	return available
}

// Selected returns the permission chosen in the grant form, or the first available one.
func (d AdminRoleData) Selected() (Permission, bool) {
	available := d.Available()
	for _, p := range available {
		if p.ID == d.GrantPermission {
			return p, true
		}
	}
	if len(available) > 0 {
		return available[0], true
	}
	return Permission{}, false
}

// Draft returns the text for a granted permission's value field: what was typed, else the granted value.
func (d AdminRoleData) Draft(p RolePermission) string {
	if text, ok := d.Drafts[p.ID]; ok {
		return text
	}
	return p.ValueText()
}

func adminUserPath(id string) string {
	return "/admin/users/" + url.PathEscape(id)
}

func adminRolePath(id string) string {
	return "/admin/roles/" + url.PathEscape(id)
}

func adminRolePermissionPath(roleID string, permissionID string) string {
	return adminRolePath(roleID) + "/permissions/" + url.PathEscape(permissionID)
}

func adminUsername(username string) string {
	if username == "" {
		return "No username"
	}
	return username
}

func usersSummary(d AdminUsersData) string {
	count := len(d.Users)
	switch {
	case count == 0 && d.Search != "" && d.More:
		return "No matches in the first " + strconv.Itoa(d.NextOffset) + " users"
	case count == 0 && d.Start > 0:
		return "No more users"
	case count == 0 && d.Search != "":
		return "No matches"
	case count == 0:
		return "No users found"
	}
	text := strconv.Itoa(count) + " users"
	if count == 1 {
		text = "1 user"
	}
	if d.Start > 0 {
		text = strconv.Itoa(count) + " more users"
		if count == 1 {
			text = "1 more user"
		}
	}
	if d.Search != "" && d.More {
		text += ". Searched the first " + strconv.Itoa(d.NextOffset) + " users"
	}
	return text
}
