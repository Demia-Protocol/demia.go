package iotago

// An Ed25519 Address Block Issuer Key.
type BlockIssuerKeyEd25519Address struct {
	Address *Ed25519Address `serix:"0"`
}

// BlockIssuerKeyEd25519FromPublicKey creates a block issuer key from an Ed25519 public key.
func BlockIssuerKeyEd25519AddressFromAddress(address *Ed25519Address) BlockIssuerKeyEd25519Address {
	return BlockIssuerKeyEd25519Address{Address: address}
}

// BlockIssuerKeyBytes returns a byte slice consisting of the type prefix and the raw address.
func (key BlockIssuerKeyEd25519Address) BlockIssuerKeyBytes() []byte {
	blockIssuerKeyBytes := make([]byte, 0, key.Size())
	blockIssuerKeyBytes = append(blockIssuerKeyBytes, byte(Ed25519BlockIssuerKeyAddress))
	return append(blockIssuerKeyBytes, key.Address[:]...)
}

// Type returns the BlockIssuerKeyType.
func (key BlockIssuerKeyEd25519Address) Type() BlockIssuerKeyType {
	return Ed25519BlockIssuerKeyAddress
}

// Size returns the size of the block issuer key when serialized.
func (key BlockIssuerKeyEd25519Address) Size() int {
	return key.Address.Size()
}

func (key BlockIssuerKeyEd25519Address) VBytes(rentStructure *RentStructure, vbyteFunc VBytesFunc) VBytes {
	return key.Address.VBytes(rentStructure, vbyteFunc)
}