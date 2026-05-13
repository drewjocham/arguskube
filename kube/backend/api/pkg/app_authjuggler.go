package pkg

import (
	"context"

	"github.com/argues/argus/internal/k8s"
)

// Wails v2 does NOT auto-inject context.Context into bound methods that
// declare it as their first parameter — the binding count check sees ctx
// as a required arg and the frontend (which only sends business args)
// gets "received N, expected N+1". Fix: drop ctx from the binding
// signature and use a.ctx (the app-scoped context wired up in Startup).
// If a.ctx is nil (early Startup), fall back to context.Background so
// the call still completes instead of panicking.

func (a *App) appCtx() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

func (a *App) CheckAuthStatus() (*k8s.AuthCheckResult, error) {
	return a.k8s.CheckAuthStatus(a.appCtx())
}

func (a *App) GetBlastRadiusInfo() (*k8s.BlastRadiusInfo, error) {
	return a.k8s.GetBlastRadiusInfo(a.appCtx())
}

func (a *App) InjectDebugContainer(namespace, podName, debugImage string) (*k8s.DebugSession, error) {
	return a.k8s.InjectDebugContainer(a.appCtx(), namespace, podName, debugImage)
}

func (a *App) CheckCanI(namespace, verb, resource, apiGroup string) (*k8s.CanIResult, error) {
	return a.k8s.CheckCanI(a.appCtx(), namespace, verb, resource, apiGroup)
}

func (a *App) BatchCheckCanI(namespace string, checks []k8s.CanIResult) ([]k8s.CanIResult, error) {
	return a.k8s.BatchCheckCanI(a.appCtx(), namespace, checks)
}

func (a *App) GetCommonPermissions(namespace string) ([]k8s.CanIResult, error) {
	return a.k8s.GetCommonPermissions(a.appCtx(), namespace)
}

func (a *App) ImpersonateUser(user string, groups []string) (*k8s.ImpersonationView, error) {
	return a.k8s.ImpersonateUser(a.appCtx(), user, groups)
}

func (a *App) ProfileWaste(namespace string) (*k8s.WasteProfile, error) {
	return a.k8s.ProfileWaste(a.appCtx(), namespace)
}

func (a *App) CorrelatePodEvents(namespace, podName string, tailLines int64) (*k8s.CorrelationResult, error) {
	return a.k8s.CorrelatePodEvents(a.appCtx(), namespace, podName, tailLines)
}
