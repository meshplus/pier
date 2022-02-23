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
	InvalidIBTP
	Other_Error
	PierConnect_Error
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
	case SrcChainService_Unavailable, ValidationRules_Unregister, Proof_Invalid, PierConnect_Error, InvalidIBTP:
		return true
	case Index_Wrong:
		return false
	default:
		return false
	}
}
