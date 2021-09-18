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
	err    string
	status int
}

func (e *SendIbtpError) Error() string {
	return e.err
}

func (e *SendIbtpError) NeedRetry() bool {
	switch e.status {
	case SrcChain_Unavailable, SrcChainService_Unavailable, ValidationRules_Unregister, Proof_Invalid:
		return true
	case Index_Gt_Exp, Index_Lt_Exp:
		return false
	default:
		return false
	}
}
