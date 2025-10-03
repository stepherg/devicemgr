package devicemgr

import "errors"

var (
	ErrDeviceNotFound          = errors.New("device not found")
	ErrDeviceOffline           = errors.New("device offline")
	ErrTimeout                 = errors.New("timeout")
	ErrAccessDenied            = errors.New("access denied")
	ErrInvalidParameter        = errors.New("invalid parameter")
	ErrConflict                = errors.New("conflict")
	ErrBackendUnavailable      = errors.New("backend unavailable")
	ErrPolicyNotFound          = errors.New("policy not found")
	ErrRuleConflict            = errors.New("rule conflict")
	ErrUnsupportedStage        = errors.New("unsupported stage")
	ErrChangeNotApproved       = errors.New("change not approved")
	ErrInvalidTargetExpression = errors.New("invalid target expression")
)
