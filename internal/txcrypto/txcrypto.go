package txcrypto

//go:generate mockgen -destination mock_txcrypto/mock_txcrypto.go -package mock_txcrypto -source txcrypto.go
type Cryptor interface {
	// encrypt can encrypt the content in IBTP
	Encrypt(content []byte, address string) ([]byte, error)

	// decrypt can decrypt the content in IBTP
	Decrypt(content []byte, address string) ([]byte, error)
}
