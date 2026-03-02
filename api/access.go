package api

import (
	"context"
	"fmt"
)

// ── Users ──────────────────────────────────────────────────────────────────

// User represents a Proxmox user from /access/users.
type User struct {
	UserID    string   `json:"userid"`
	Enable    int      `json:"enable"`
	Expire    int64    `json:"expire"`
	FirstName string   `json:"firstname,omitempty"`
	LastName  string   `json:"lastname,omitempty"`
	Email     string   `json:"email,omitempty"`
	Comment   string   `json:"comment,omitempty"`
	Groups    []string `json:"groups,omitempty"`
	RealmType string   `json:"realm-type,omitempty"`
}

// GetUsers returns all users.
func (c *Client) GetUsers(ctx context.Context) ([]User, error) {
	var out APIResponse[[]User]
	if err := c.get(ctx, "/access/users", &out); err != nil {
		return nil, fmt.Errorf("get users: %w", ClassifyError(err))
	}
	return out.Data, nil
}

// CreateUser creates a new user.
func (c *Client) CreateUser(ctx context.Context, userid, password, email, comment string, enable bool) error {
	en := "0"
	if enable {
		en = "1"
	}
	body := map[string]string{"userid": userid, "enable": en}
	if password != "" {
		body["password"] = password
	}
	if email != "" {
		body["email"] = email
	}
	if comment != "" {
		body["comment"] = comment
	}
	var out APIResponse[any]
	if err := c.post(ctx, "/access/users", body, &out); err != nil {
		return fmt.Errorf("create user %s: %w", userid, ClassifyError(err))
	}
	return nil
}

// DeleteUser deletes a user by userid (e.g. "user@pve").
func (c *Client) DeleteUser(ctx context.Context, userid string) error {
	var out APIResponse[any]
	if err := c.del(ctx, fmt.Sprintf("/access/users/%s", userid), &out); err != nil {
		return fmt.Errorf("delete user %s: %w", userid, ClassifyError(err))
	}
	return nil
}

// ── Groups ──────────────────────────────────────────────────────────────────

// Group represents a Proxmox user group.
type Group struct {
	GroupID string   `json:"groupid"`
	Comment string   `json:"comment,omitempty"`
	Members []string `json:"members,omitempty"`
}

// GetGroups returns all groups.
func (c *Client) GetGroups(ctx context.Context) ([]Group, error) {
	var out APIResponse[[]Group]
	if err := c.get(ctx, "/access/groups", &out); err != nil {
		return nil, fmt.Errorf("get groups: %w", ClassifyError(err))
	}
	return out.Data, nil
}

// CreateGroup creates a new group.
func (c *Client) CreateGroup(ctx context.Context, groupid, comment string) error {
	body := map[string]string{"groupid": groupid}
	if comment != "" {
		body["comment"] = comment
	}
	var out APIResponse[any]
	if err := c.post(ctx, "/access/groups", body, &out); err != nil {
		return fmt.Errorf("create group %s: %w", groupid, ClassifyError(err))
	}
	return nil
}

// DeleteGroup deletes a group.
func (c *Client) DeleteGroup(ctx context.Context, groupid string) error {
	var out APIResponse[any]
	if err := c.del(ctx, fmt.Sprintf("/access/groups/%s", groupid), &out); err != nil {
		return fmt.Errorf("delete group %s: %w", groupid, ClassifyError(err))
	}
	return nil
}

// ── Roles ──────────────────────────────────────────────────────────────────

// Role represents a Proxmox role with its privileges.
type Role struct {
	RoleID  string `json:"roleid"`
	Privs   string `json:"privs,omitempty"`
	Special int    `json:"special,omitempty"` // 1 = built-in
}

// GetRoles returns all roles.
func (c *Client) GetRoles(ctx context.Context) ([]Role, error) {
	var out APIResponse[[]Role]
	if err := c.get(ctx, "/access/roles", &out); err != nil {
		return nil, fmt.Errorf("get roles: %w", ClassifyError(err))
	}
	return out.Data, nil
}

// CreateRole creates a custom role.
func (c *Client) CreateRole(ctx context.Context, roleid, privs string) error {
	body := map[string]string{"roleid": roleid}
	if privs != "" {
		body["privs"] = privs
	}
	var out APIResponse[any]
	if err := c.post(ctx, "/access/roles", body, &out); err != nil {
		return fmt.Errorf("create role %s: %w", roleid, ClassifyError(err))
	}
	return nil
}

// DeleteRole deletes a custom role.
func (c *Client) DeleteRole(ctx context.Context, roleid string) error {
	var out APIResponse[any]
	if err := c.del(ctx, fmt.Sprintf("/access/roles/%s", roleid), &out); err != nil {
		return fmt.Errorf("delete role %s: %w", roleid, ClassifyError(err))
	}
	return nil
}

// ── ACLs ──────────────────────────────────────────────────────────────────

// ACLEntry represents a permission assignment in /access/acl.
type ACLEntry struct {
	Path      string `json:"path"`
	UGI       string `json:"ugid"`      // user, group, or token id
	Type      string `json:"ugid-type"` // user|group|token
	RoleID    string `json:"roleid"`
	Propagate int    `json:"propagate"`
}

// GetACL returns all ACL entries for the cluster.
func (c *Client) GetACL(ctx context.Context) ([]ACLEntry, error) {
	var out APIResponse[[]ACLEntry]
	if err := c.get(ctx, "/access/acl", &out); err != nil {
		return nil, fmt.Errorf("get acl: %w", ClassifyError(err))
	}
	return out.Data, nil
}

// UpdateACL adds or removes an ACL entry.
// ugidType: "user"|"group"|"token". delete=true removes it.
func (c *Client) UpdateACL(ctx context.Context, path, ugid, ugidType, roleid string, propagate, delete bool) error {
	prop := "0"
	if propagate {
		prop = "1"
	}
	del := "0"
	if delete {
		del = "1"
	}
	body := map[string]string{
		"path":      path,
		"roles":     roleid,
		"propagate": prop,
		"delete":    del,
	}
	switch ugidType {
	case "group":
		body["groups"] = ugid
	case "token":
		body["tokens"] = ugid
	default:
		body["users"] = ugid
	}
	var out APIResponse[any]
	if err := c.put(ctx, "/access/acl", body, &out); err != nil {
		return fmt.Errorf("update acl: %w", ClassifyError(err))
	}
	return nil
}

// ── Tokens ──────────────────────────────────────────────────────────────────

// Token represents an API token entry.
type Token struct {
	TokenID string `json:"tokenid"`
	Comment string `json:"comment,omitempty"`
	Expire  int64  `json:"expire,omitempty"`
	Privsep int    `json:"privsep,omitempty"`
}

// GetTokens returns all API tokens for a user.
func (c *Client) GetTokens(ctx context.Context, userid string) ([]Token, error) {
	var out APIResponse[[]Token]
	if err := c.get(ctx, fmt.Sprintf("/access/users/%s/token", userid), &out); err != nil {
		return nil, fmt.Errorf("get tokens for %s: %w", userid, ClassifyError(err))
	}
	return out.Data, nil
}
