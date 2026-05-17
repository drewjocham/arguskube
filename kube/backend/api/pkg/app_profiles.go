package pkg

import (
	"errors"

	profilespkg "github.com/argues/argus/internal/profiles"
)

// app_profiles.go — Wails-bound CRUD over the per-user workspace
// profiles store. Mirrors the workspace methods' shape (sessionToken
// arg passed through profilesUserID for the auth check) so callers
// have one consistent way to talk to per-user state.

// profilesUserID resolves the caller from the session token. Shares
// the dev-mode bypass with workspaceUserID so the auth-disabled
// dev workflow doesn't have two divergent synthetic user IDs.
func (a *App) profilesUserID(token string) (string, error) {
	if a.auth == nil || a.auth.store == nil {
		return "", errors.New("profiles: auth not configured")
	}
	if a.auth.devMode {
		return devModeUserID, nil
	}
	user, err := a.auth.store.ValidateSession(token)
	if err != nil || user == nil {
		return "", errors.New("profiles: invalid session")
	}
	return user.ID, nil
}

// requireProfileStore returns the store or a descriptive error. Keeps
// every handler from repeating the nil-check.
func (a *App) requireProfileStore() (*profilespkg.Store, error) {
	if a.profiles == nil {
		return nil, errors.New("profiles: store not configured (no DB in this build)")
	}
	return a.profiles, nil
}

// ListProfileGroups returns every group + its variants for the caller.
// Empty slice is the no-profiles-yet response, never nil — keeps JSON
// callers from special-casing null.
func (a *App) ListProfileGroups(sessionToken string) ([]profilespkg.Group, error) {
	store, err := a.requireProfileStore()
	if err != nil {
		return nil, err
	}
	userID, err := a.profilesUserID(sessionToken)
	if err != nil {
		return nil, err
	}
	groups, err := store.ListGroups(a.appCtx(), userID)
	if err != nil {
		return nil, err
	}
	if groups == nil {
		groups = []profilespkg.Group{}
	}
	return groups, nil
}

// SaveProfileGroup upserts a group. The frontend supplies the id
// (UUID generated client-side); the backend treats it as opaque.
func (a *App) SaveProfileGroup(sessionToken string, group profilespkg.Group) (profilespkg.Group, error) {
	store, err := a.requireProfileStore()
	if err != nil {
		return profilespkg.Group{}, err
	}
	userID, err := a.profilesUserID(sessionToken)
	if err != nil {
		return profilespkg.Group{}, err
	}
	return store.SaveGroup(a.appCtx(), userID, group)
}

// DeleteProfileGroup removes the group and cascades to its variants.
// Returns the store's ErrNotFound (as a plain error message over the
// wire) for cross-user or unknown ids.
func (a *App) DeleteProfileGroup(sessionToken, groupID string) error {
	store, err := a.requireProfileStore()
	if err != nil {
		return err
	}
	userID, err := a.profilesUserID(sessionToken)
	if err != nil {
		return err
	}
	return store.DeleteGroup(a.appCtx(), userID, groupID)
}

// SaveProfileVariant upserts a variant within a group. The group
// must already exist and belong to the caller — cross-user attempts
// return ErrNotFound, not a confusing FK error.
func (a *App) SaveProfileVariant(sessionToken, groupID string, variant profilespkg.Variant) (profilespkg.Variant, error) {
	store, err := a.requireProfileStore()
	if err != nil {
		return profilespkg.Variant{}, err
	}
	userID, err := a.profilesUserID(sessionToken)
	if err != nil {
		return profilespkg.Variant{}, err
	}
	return store.SaveVariant(a.appCtx(), userID, groupID, variant)
}

// DeleteProfileVariant removes one variant. If it was the user's
// active selection, the store clears the active pointer as a side
// effect so the frontend doesn't try to restore a gone row.
func (a *App) DeleteProfileVariant(sessionToken, groupID, variantID string) error {
	store, err := a.requireProfileStore()
	if err != nil {
		return err
	}
	userID, err := a.profilesUserID(sessionToken)
	if err != nil {
		return err
	}
	return store.DeleteVariant(a.appCtx(), userID, groupID, variantID)
}

// GetActiveProfile returns the caller's last-applied (group, variant)
// pair. Zero-value Active is normal — it means "never applied
// anything", and the frontend renders the No-Profile state.
func (a *App) GetActiveProfile(sessionToken string) (profilespkg.Active, error) {
	store, err := a.requireProfileStore()
	if err != nil {
		return profilespkg.Active{}, err
	}
	userID, err := a.profilesUserID(sessionToken)
	if err != nil {
		return profilespkg.Active{}, err
	}
	return store.GetActive(a.appCtx(), userID)
}

// SetActiveProfile records which profile the user just applied.
// Passing two empty strings clears the selection — that's how the
// frontend communicates "the last profile was deleted; show
// 'No Profile' from now on".
func (a *App) SetActiveProfile(sessionToken, groupID, variantID string) error {
	store, err := a.requireProfileStore()
	if err != nil {
		return err
	}
	userID, err := a.profilesUserID(sessionToken)
	if err != nil {
		return err
	}
	return store.SetActive(a.appCtx(), userID, groupID, variantID)
}
