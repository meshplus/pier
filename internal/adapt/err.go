package adapt

// error type
const (
	Success = iota
	SrcChain_Unavailable
	SrcChainService_Unavailable
	ValidationRules_Unregister
	Proof_Invalid
	Index_Gt_Exp
	Index_Lt_Exp
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
	case Index_Gt_Exp, Index_Lt_Exp:
		return false
	default:
		return false
	}
}
