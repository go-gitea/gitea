package googletpm

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"
)

// DecodeAttestationData decode a TPMS_ATTEST message. No error is returned if
// the input has extra trailing data.
func DecodeAttestationData(in []byte) (*AttestationData, error) {
	buf := bytes.NewBuffer(in)

	var ad AttestationData
	if err := UnpackBuf(buf, &ad.Magic, &ad.Type); err != nil {
		return nil, fmt.Errorf("decoding Magic/Type: %v", err)
	}
	n, err := decodeName(buf)
	if err != nil {
		return nil, fmt.Errorf("decoding QualifiedSigner: %v", err)
	}
	ad.QualifiedSigner = *n
	if err := UnpackBuf(buf, &ad.ExtraData, &ad.ClockInfo, &ad.FirmwareVersion); err != nil {
		return nil, fmt.Errorf("decoding ExtraData/ClockInfo/FirmwareVersion: %v", err)
	}

	// The spec specifies several other types of attestation data. We only need
	// parsing of Certify & Creation attestation data for now. If you need
	// support for other attestation types, add them here.
	switch ad.Type {
	case TagAttestCertify:
		if ad.AttestedCertifyInfo, err = decodeCertifyInfo(buf); err != nil {
			return nil, fmt.Errorf("decoding AttestedCertifyInfo: %v", err)
		}
	case TagAttestCreation:
		if ad.AttestedCreationInfo, err = decodeCreationInfo(buf); err != nil {
			return nil, fmt.Errorf("decoding AttestedCreationInfo: %v", err)
		}
	case TagAttestQuote:
		if ad.AttestedQuoteInfo, err = decodeQuoteInfo(buf); err != nil {
			return nil, fmt.Errorf("decoding AttestedQuoteInfo: %v", err)
		}
	default:
		return nil, fmt.Errorf("only Certify & Creation attestation structures are supported, got type 0x%x", ad.Type)
	}

	return &ad, nil
}

// AttestationData contains data attested by TPM commands (like Certify).
type AttestationData struct {
	Magic                uint32
	Type                 Tag
	QualifiedSigner      Name
	ExtraData            []byte
	ClockInfo            ClockInfo
	FirmwareVersion      uint64
	AttestedCertifyInfo  *CertifyInfo
	AttestedQuoteInfo    *QuoteInfo
	AttestedCreationInfo *CreationInfo
}

// Tag is a command tag.
type Tag uint16

type Name struct {
	Handle *Handle
	Digest *HashValue
}

// A Handle is a reference to a TPM object.
type Handle uint32
type HashValue struct {
	Alg   Algorithm
	Value []byte
}

// ClockInfo contains TPM state info included in AttestationData.
type ClockInfo struct {
	Clock        uint64
	ResetCount   uint32
	RestartCount uint32
	Safe         byte
}

// CertifyInfo contains Certify-specific data for TPMS_ATTEST.
type CertifyInfo struct {
	Name          Name
	QualifiedName Name
}

// QuoteInfo represents a TPMS_QUOTE_INFO structure.
type QuoteInfo struct {
	PCRSelection PCRSelection
	PCRDigest    []byte
}

// PCRSelection contains a slice of PCR indexes and a hash algorithm used in
// them.
type PCRSelection struct {
	Hash Algorithm
	PCRs []int
}

// CreationInfo contains Creation-specific data for TPMS_ATTEST.
type CreationInfo struct {
	Name Name
	// Most TPM2B_Digest structures contain a TPMU_HA structure
	// and get parsed to HashValue. This is never the case for the
	// digest in TPMS_CREATION_INFO.
	OpaqueDigest []byte
}

func decodeName(in *bytes.Buffer) (*Name, error) {
	var nameBuf []byte
	if err := UnpackBuf(in, &nameBuf); err != nil {
		return nil, err
	}

	name := new(Name)
	switch len(nameBuf) {
	case 0:
		// No name is present.
	case 4:
		name.Handle = new(Handle)
		if err := UnpackBuf(bytes.NewBuffer(nameBuf), name.Handle); err != nil {
			return nil, fmt.Errorf("decoding Handle: %v", err)
		}
	default:
		var err error
		name.Digest, err = decodeHashValue(bytes.NewBuffer(nameBuf))
		if err != nil {
			return nil, fmt.Errorf("decoding Digest: %v", err)
		}
	}
	return name, nil
}

func decodeHashValue(in *bytes.Buffer) (*HashValue, error) {
	var hv HashValue
	if err := UnpackBuf(in, &hv.Alg); err != nil {
		return nil, fmt.Errorf("decoding Alg: %v", err)
	}
	hfn, ok := hashConstructors[hv.Alg]
	if !ok {
		return nil, fmt.Errorf("unsupported hash algorithm type 0x%x", hv.Alg)
	}
	hv.Value = make([]byte, hfn().Size())
	if _, err := in.Read(hv.Value); err != nil {
		return nil, fmt.Errorf("decoding Value: %v", err)
	}
	return &hv, nil
}

// HashConstructor returns a function that can be used to make a
// hash.Hash using the specified algorithm. An error is returned
// if the algorithm is not a hash algorithm.
func (a Algorithm) HashConstructor() (func() hash.Hash, error) {
	c, ok := hashConstructors[a]
	if !ok {
		return nil, fmt.Errorf("algorithm not supported: 0x%x", a)
	}
	return c, nil
}

var hashConstructors = map[Algorithm]func() hash.Hash{
	AlgSHA1:   sha1.New,
	AlgSHA256: sha256.New,
	AlgSHA384: sha512.New384,
	AlgSHA512: sha512.New,
}

// TPM Structure Tags. Tags are used to disambiguate structures, similar to Alg
// values: tag value defines what kind of data lives in a nested field.
const (
	TagNull           Tag = 0x8000
	TagNoSessions     Tag = 0x8001
	TagSessions       Tag = 0x8002
	TagAttestCertify  Tag = 0x8017
	TagAttestQuote    Tag = 0x8018
	TagAttestCreation Tag = 0x801a
	TagHashCheck      Tag = 0x8024
)

func decodeCertifyInfo(in *bytes.Buffer) (*CertifyInfo, error) {
	var ci CertifyInfo

	n, err := decodeName(in)
	if err != nil {
		return nil, fmt.Errorf("decoding Name: %v", err)
	}
	ci.Name = *n

	n, err = decodeName(in)
	if err != nil {
		return nil, fmt.Errorf("decoding QualifiedName: %v", err)
	}
	ci.QualifiedName = *n

	return &ci, nil
}

func decodeCreationInfo(in *bytes.Buffer) (*CreationInfo, error) {
	var ci CreationInfo

	n, err := decodeName(in)
	if err != nil {
		return nil, fmt.Errorf("decoding Name: %v", err)
	}
	ci.Name = *n

	if err := UnpackBuf(in, &ci.OpaqueDigest); err != nil {
		return nil, fmt.Errorf("decoding Digest: %v", err)
	}

	return &ci, nil
}

func decodeQuoteInfo(in *bytes.Buffer) (*QuoteInfo, error) {
	var out QuoteInfo
	sel, err := decodeTPMLPCRSelection(in)
	if err != nil {
		return nil, fmt.Errorf("decoding PCRSelection: %v", err)
	}
	out.PCRSelection = sel
	if err := UnpackBuf(in, &out.PCRDigest); err != nil {
		return nil, fmt.Errorf("decoding PCRDigest: %v", err)
	}
	return &out, nil
}

func decodeTPMLPCRSelection(buf *bytes.Buffer) (PCRSelection, error) {
	var count uint32
	var sel PCRSelection
	if err := UnpackBuf(buf, &count); err != nil {
		return sel, err
	}
	switch count {
	case 0:
		sel.Hash = AlgUnknown
		return sel, nil
	case 1: // We only support decoding of a single PCRSelection.
	default:
		return sel, fmt.Errorf("decoding TPML_PCR_SELECTION list longer than 1 is not supported (got length %d)", count)
	}

	// See comment in encodeTPMLPCRSelection for details on this format.
	var ts tpmsPCRSelection
	if err := UnpackBuf(buf, &ts.Hash, &ts.Size); err != nil {
		return sel, err
	}
	ts.PCRs = make([]byte, ts.Size)
	if _, err := buf.Read(ts.PCRs); err != nil {
		return sel, err
	}

	sel.Hash = ts.Hash
	for i := 0; i < int(ts.Size); i++ {
		for j := 0; j < 8; j++ {
			set := ts.PCRs[i] & byte(1<<byte(j))
			if set == 0 {
				continue
			}
			sel.PCRs = append(sel.PCRs, 8*i+j)
		}
	}
	return sel, nil
}

type tpmsPCRSelection struct {
	Hash Algorithm
	Size byte
	PCRs RawBytes
}

// RawBytes is for Pack and RunCommand arguments that are already encoded.
// Compared to []byte, RawBytes will not be prepended with slice length during
// encoding.
type RawBytes []byte
