package adapt

// error type
const (
	Success = iota
	SrcChain_Unavailable
	TargetChain_Unavailable
	SrcChainService_Unavailable
	TargetChainService_Unavailable
	ValidationRules_Unregister
	Proof_Invalid
	Index_Wrong
)

type SendIbtpError struct {
	Err    string
	Status int
}

func (e *SendIbtpError) Error() string {
	return e.Err
}

func (e *SendIbtpError) NeedRetry() bool {
	switch e.Status {
	case SrcChain_Unavailable, SrcChainService_Unavailable, ValidationRules_Unregister, Proof_Invalid:
		return true
	case TargetChain_Unavailable, TargetChainService_Unavailable, Index_Wrong:
		return false
	default:
		return false
	}
}
