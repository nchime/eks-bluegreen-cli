package errors

import (
	"errors"
	"fmt"
)

var (
	ErrJobNotFound             = errors.New("upgrade job not found")
	ErrJobAlreadyExists        = errors.New("upgrade job already exists")
	ErrInvalidPhaseTransition  = errors.New("invalid job phase transition")
	ErrApprovalRequired        = errors.New("explicit approval is required before cutover")
	ErrApprovalInvalidated     = errors.New("approval invalidated: plan or validation state has changed since approval")
	ErrValidationFailed        = errors.New("validation failed: critical checks have not passed")
	ErrBlueClusterProtected    = errors.New("safety guardrail: operation rejected because target equals blue (source) cluster")
	ErrBlueClusterDeletion     = errors.New("safety guardrail: blue cluster deletion or destructive modification is prohibited")
	ErrInvalidPlan             = errors.New("invalid upgrade plan")
	ErrResourceConflict        = errors.New("resource conflict in green environment")
	ErrToolMissing             = errors.New("required external tool is missing")
	ErrAWSAuthFailed           = errors.New("aws authentication or permission check failed")
)

type GuardrailError struct {
	ClusterName string
	Operation   string
	Reason      string
}

func (e *GuardrailError) Error() string {
	return fmt.Sprintf("guardrail violation for cluster %q during %s: %s", e.ClusterName, e.Operation, e.Reason)
}

type PhaseTransitionError struct {
	CurrentPhase string
	TargetPhase  string
	Reason       string
}

func (e *PhaseTransitionError) Error() string {
	return fmt.Sprintf("cannot transition from phase %s to %s: %s", e.CurrentPhase, e.TargetPhase, e.Reason)
}
