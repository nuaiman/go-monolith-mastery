// Package constants centralizes shared vocabulary — role names, size
// limits, pagination defaults, and other magic values.
//
// It has no imports so it can be freely used by any layer (models,
// handlers, middlewares, bootstrap, CLI tools) without creating import
// cycles.
package constants

// Role values, in ascending order of privilege:
//
//	RoleUser < RoleAdmin < RoleSuperAdmin
//
// Use CanModerate to decide whether one role may act on another.
const (
	RoleUser       = "user"
	RoleAdmin      = "admin"
	RoleSuperAdmin = "superadmin"
)

// AllRoles returns every valid role.
func AllRoles() []string {
	return []string{RoleUser, RoleAdmin, RoleSuperAdmin}
}

// IsValidRole reports whether role is one of the defined roles.
func IsValidRole(role string) bool {
	switch role {
	case RoleUser, RoleAdmin, RoleSuperAdmin:
		return true
	}
	return false
}

// RoleRank returns a sortable power level. Higher = more powerful.
// Unknown roles return 0.
func RoleRank(role string) int {
	switch role {
	case RoleSuperAdmin:
		return 3
	case RoleAdmin:
		return 2
	case RoleUser:
		return 1
	}
	return 0
}

// CanModerate reports whether the caller's rank is strictly greater
// than the target's rank. Admins cannot moderate admins; superadmins
// cannot moderate superadmins.
func CanModerate(callerRole, targetRole string) bool {
	return RoleRank(callerRole) > RoleRank(targetRole)
}
