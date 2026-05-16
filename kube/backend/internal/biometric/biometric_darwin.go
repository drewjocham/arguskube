//go:build darwin

// Package biometric wraps the macOS LocalAuthentication framework so the
// Argus desktop app can unlock a Keychain-stored session with Touch ID
// instead of asking the user to re-type a password every launch.
//
// Why Cgo despite the project's "no-CGO" preference for secretstore:
// LocalAuthentication has no shell-out equivalent. The `security`
// CLI doesn't expose `LAContext evaluatePolicy`. The only way to
// trigger the system Touch ID prompt is to link the framework
// directly. Build tags keep the Cgo dependency macOS-only so Linux
// and Windows builds still compile cleanly without CGO.
package biometric

/*
#cgo darwin CFLAGS: -x objective-c -fobjc-arc
#cgo darwin LDFLAGS: -framework LocalAuthentication -framework Foundation

#import <Foundation/Foundation.h>
#import <LocalAuthentication/LocalAuthentication.h>
#include <stdlib.h>

// argus_biometric_available reports whether the current device has
// Touch ID (or Face ID) configured and enrolled. Returns 1 on yes,
// 0 on no. We use LAPolicyDeviceOwnerAuthenticationWithBiometrics
// specifically — NOT the broader "...WithAuthentication" policy that
// also accepts the user's login password — because the product goal
// is "tap your finger, you're in"; falling back to a password prompt
// here would defeat the whole UX win.
static int argus_biometric_available(void) {
    LAContext *ctx = [[LAContext alloc] init];
    NSError *err = nil;
    BOOL ok = [ctx canEvaluatePolicy:LAPolicyDeviceOwnerAuthenticationWithBiometrics
                               error:&err];
    return ok ? 1 : 0;
}

// argus_biometric_authenticate prompts the user. Blocks until the
// system dialog resolves. The reply block fires on a private LA
// queue, so we synchronize back to the caller with a semaphore —
// this is the documented pattern from Apple's sample code.
//
// errOut: caller-owned buffer (errBufLen bytes). On failure we copy
// the NSError's localizedDescription into it (UTF-8, NUL-terminated,
// truncated to fit). On success we leave it empty.
//
// Returns 1 on success, 0 on failure.
static int argus_biometric_authenticate(const char *reasonC,
                                        char *errOut,
                                        int errBufLen) {
    if (errBufLen > 0 && errOut != NULL) errOut[0] = '\0';

    LAContext *ctx = [[LAContext alloc] init];
    NSError *preErr = nil;
    if (![ctx canEvaluatePolicy:LAPolicyDeviceOwnerAuthenticationWithBiometrics
                          error:&preErr]) {
        if (preErr && errOut && errBufLen > 0) {
            const char *m = [[preErr localizedDescription] UTF8String];
            if (m) {
                strncpy(errOut, m, (size_t)(errBufLen - 1));
                errOut[errBufLen - 1] = '\0';
            }
        }
        return 0;
    }

    NSString *reason = reasonC ? [NSString stringWithUTF8String:reasonC]
                               : @"Authenticate";

    __block BOOL ok = NO;
    __block NSString *errMsg = nil;
    dispatch_semaphore_t sem = dispatch_semaphore_create(0);

    [ctx evaluatePolicy:LAPolicyDeviceOwnerAuthenticationWithBiometrics
        localizedReason:reason
                  reply:^(BOOL success, NSError *e) {
        ok = success;
        if (e) errMsg = [e localizedDescription];
        dispatch_semaphore_signal(sem);
    }];
    dispatch_semaphore_wait(sem, DISPATCH_TIME_FOREVER);

    if (!ok && errMsg && errOut && errBufLen > 0) {
        const char *m = [errMsg UTF8String];
        if (m) {
            strncpy(errOut, m, (size_t)(errBufLen - 1));
            errOut[errBufLen - 1] = '\0';
        }
    }
    return ok ? 1 : 0;
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

// Available reports whether biometric auth is configured + usable.
// False if the Mac has no Touch ID hardware, no fingerprint enrolled,
// or the framework refuses the policy for any other reason.
func Available() bool {
	return C.argus_biometric_available() == 1
}

// Authenticate displays the system Touch ID / Face ID prompt with the
// given reason string. Returns nil on success.
//
// BLOCKS until the user authenticates or dismisses the prompt. The
// underlying semaphore wait means callers MUST NOT invoke this from
// the goroutine that pumps the Wails main/event loop — Wails routes
// each method on a fresh goroutine so the normal binding call is
// safe, but anything that synchronizes back to the runtime thread
// would deadlock here. Wrap in a goroutine when in doubt.
func Authenticate(reason string) error {
	if reason == "" {
		reason = "Authenticate"
	}
	cReason := C.CString(reason)
	defer C.free(unsafe.Pointer(cReason))

	const bufLen = 512
	errBuf := (*C.char)(C.malloc(bufLen))
	defer C.free(unsafe.Pointer(errBuf))

	ok := C.argus_biometric_authenticate(cReason, errBuf, C.int(bufLen))
	if ok == 1 {
		return nil
	}
	msg := C.GoString(errBuf)
	if msg == "" {
		msg = "biometric authentication failed"
	}
	return errors.New("biometric: " + msg)
}
