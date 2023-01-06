package adapt

// error type
const (
	Success = iota
	SrcChainServiceUnavailable
	ValidationRulesUnregister
	ProofInvalid
	IndexWrong
	InvalidIBTP
	OtherError
	PierConnectError
	ExecuteRevert
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
	case SrcChainServiceUnavailable, ValidationRulesUnregister, ProofInvalid, PierConnectError, InvalidIBTP:
		return true
	case IndexWrong:
		return false
	default:
		return false
	}
}

func (e *SendIbtpError) NeedRetryWithLimit() bool {
	switch e.Status {
	case ExecuteRevert:
		return true
	default:
		return false
	}
}
