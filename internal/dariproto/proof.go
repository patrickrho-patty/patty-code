package dariproto

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
)

// ObjectType identifies a registered DARI object type for domain-separated
// content addressing (DARI §32, internal/paper/crypto.go). The connector
// only recognizes the PeerCredential type today; other types are rejected
// to keep the digest domain narrow.
type ObjectType uint16

// Object types used by the connector's AUTH_PROOF path.
const (
	ObjTypePeerCredential ObjectType = 0x0100
)

// Digest is a SHA-256 content digest (DARI §37.1).
type Digest [32]byte

// Bytes returns the raw digest bytes.
func (d Digest) Bytes() []byte { return d[:] }

// String returns the hex encoding of the digest.
func (d Digest) String() string {
	return fmt.Sprintf("sha256:%x", d[:])
}

// ComputeObjectDigest computes the content-addressed digest of a registered
// DARI object per DARI §32. The encoding MUST match the relay's
// `internal/paper/crypto.go::ComputeObjectDigest` byte-for-byte:
//
//	digest = SHA256("DARI-OBJ-v1\0" || 0x00 || uint16BE(T) || canonical_cbor(O))
//
// The relay writes an explicit zero byte between the domain prefix and
// the object-type uint16 (a reserved flags byte); the connector mirrors
// it exactly. The COSE-Sign1 credential bytes (NOT the parsed payload)
// MUST be the input; the relay hashes the same byte string when it
// verifies the proof.
func ComputeObjectDigest(objType ObjectType, canonicalCBOR []byte) Digest {
	h := sha256.New()
	h.Write([]byte("DARI-OBJ-v1\x00"))
	h.Write([]byte{0})
	var typeBytes [2]byte
	binary.BigEndian.PutUint16(typeBytes[:], uint16(objType))
	h.Write(typeBytes[:])
	h.Write(canonicalCBOR)
	var d Digest
	copy(d[:], h.Sum(nil))
	return d
}

// AuthContextDomain is the domain-separation prefix used by the relay's
// transcript hash (DARI §18.2). The connector and the relay MUST agree
// on this constant EXACTLY — the relay's `paper.AuthContext` writes
// "DARI-AUTH-v1" with NO trailing NUL (verified against the live
// verifier; the e2e suite exercises the real bytes).
const AuthContextDomain = "DARI-AUTH-v1"

// AuthProofDomain is the domain-separation prefix used by the relay's
// proof-of-possession signing (DARI §18.2). Matches the relay's
// `internal/paper/peer.go::PeerProofSigningBytes` domain constant.
const AuthProofDomain = "DARI-AUTH-PROOF-v1\x00"

// BuildAuthContext computes the DARI authentication context hash used by both
// peers to bind the proof-of-possession to the negotiated transcript. The
// inputs are the canonical CBOR encodings of the HELLO and HELLO_ACK already
// exchanged on the wire, the two nonces, the channel binding identifier
// (e.g. "tcp-exporter"), and the credential digest.
//
// The encoding MUST match the relay's
// `internal/paper/crypto.go::AuthContext` byte-for-byte; otherwise the relay
// silently rejects the proof.
func BuildAuthContext(helloCBOR, helloAckCBOR, clientNonce, serverNonce, channelBinding, peerCredDigest []byte) Digest {
	h := sha256.New()
	h.Write([]byte(AuthContextDomain))
	h.Write(helloCBOR)
	h.Write(helloAckCBOR)
	h.Write(clientNonce)
	h.Write(serverNonce)
	h.Write(channelBinding)
	h.Write(peerCredDigest)
	var d Digest
	copy(d[:], h.Sum(nil))
	return d
}

// SignAuthProof returns the Ed25519 signature the connector places in
// `AuthProofMessage.Signature`. The signing bytes are the SHA-256 of the
// domain-separation prefix plus length-prefixed transcript, challenge ID,
// and big-endian revocation epoch.
//
// The encoding MUST match the relay's
// `internal/paper/peer.go::PeerProofSigningBytes` byte-for-byte.
func SignAuthProof(priv ed25519.PrivateKey, transcript, challengeID []byte, revocationEpoch uint64) []byte {
	return ed25519.Sign(priv, SignAuthProofInputs(transcript, challengeID, revocationEpoch))
}

// SignAuthProofInputs returns the exact signing bytes produced by
// SignAuthProof; tests use it to verify against the same byte string the
// relay recomputes.
func SignAuthProofInputs(transcript, challengeID []byte, revocationEpoch uint64) []byte {
	h := sha256.New()
	h.Write([]byte(AuthProofDomain))
	writeLengthPrefixed(h, transcript)
	writeLengthPrefixed(h, challengeID)
	var epochBuf [8]byte
	binary.BigEndian.PutUint64(epochBuf[:], revocationEpoch)
	h.Write(epochBuf[:])
	return h.Sum(nil)
}

// EncodeRevocationEpoch encodes the revocation checkpoint carried in the
// AUTH_PROOF revocation-evidence field as a big-endian uint64.
func EncodeRevocationEpoch(epoch uint64) []byte {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], epoch)
	return buf[:]
}

// AuthProofInput groups the inputs needed to build a complete AUTH_PROOF
// message. The connector collects these once after the HELLO/HELLO_ACK
// exchange and the AUTH_CHALLENGE reception.
type AuthProofInput struct {
	PrivateKey      ed25519.PrivateKey
	Credential      []byte // raw COSE-Sign1 CBOR of the enrolled peer credential
	Hello           *HelloMessage
	HelloAck        *HelloAckMessage
	ChallengeID     []byte
	RevocationEpoch uint64
	ChannelBinding  []byte // e.g. []byte("tcp-exporter")
}

// BuildAuthProof assembles the connector-side AUTH_PROOF using the supplied
// transcript context. The relay independently recomputes the same hash and
// verifies the signature under the issuer-defined subject public key embedded
// in the credential body.
func BuildAuthProof(in AuthProofInput) (*AuthProofMessage, error) {
	if len(in.PrivateKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("dari: invalid private key size %d", len(in.PrivateKey))
	}
	if len(in.Credential) == 0 {
		return nil, errors.New("dari: empty peer credential")
	}
	if in.Hello == nil || in.HelloAck == nil {
		return nil, errors.New("dari: missing HELLO/HELLO_ACK for proof context")
	}
	if len(in.ChallengeID) == 0 {
		return nil, errors.New("dari: empty challenge ID")
	}
	if len(in.ChannelBinding) == 0 {
		return nil, errors.New("dari: empty channel binding")
	}

	helloCBOR, err := CanonicalHelloCBOR(in.Hello)
	if err != nil {
		return nil, fmt.Errorf("dari: marshal HELLO: %w", err)
	}
	ackCBOR, err := CanonicalAckCBOR(in.HelloAck)
	if err != nil {
		return nil, fmt.Errorf("dari: marshal HELLO_ACK: %w", err)
	}
	credDigest := ComputeObjectDigest(ObjTypePeerCredential, in.Credential)
	transcript := BuildAuthContext(
		helloCBOR, ackCBOR,
		in.Hello.ClientNonce, in.HelloAck.ServerNonce,
		in.ChannelBinding,
		credDigest[:],
	)

	proof := &AuthProofMessage{
		Credential:         append([]byte(nil), in.Credential...),
		Signature:          SignAuthProof(in.PrivateKey, transcript[:], in.ChallengeID, in.RevocationEpoch),
		KeyAlgorithm:       COSEAlgEdDSA,
		ChallengeID:        append([]byte(nil), in.ChallengeID...),
		RevocationEvidence: EncodeRevocationEpoch(in.RevocationEpoch),
	}
	debugAuthTranscript = append([]byte(nil), transcript[:]...)
	return proof, nil
}

// debugAuthTranscript holds the last proof's transcript bytes for
// cross-repo debugging (DARI_DEBUG_AUTH). Never serialized.
var debugAuthTranscript []byte

// DebugLastAuthTranscript returns the transcript bytes of the most
// recent BuildAuthProof call (development aid).
func DebugLastAuthTranscript() []byte { return append([]byte(nil), debugAuthTranscript...) }

// Identity is the loaded enrolled-credential pair the connector uses to
// authenticate against a DARI relay. The credential bytes are the raw
// COSE-Sign1 CBOR returned by the issuer's issuance step; the private key is
// the Ed25519 subject key referenced inside the credential body.
type Identity struct {
	PrivateKey ed25519.PrivateKey
	Credential []byte
}

// peerCredentialBody mirrors the relay's `paper.PeerCredential` body.
// The issuer encodes with snake_case NAMED cbor keys (verified against
// the live issuer's bytes; see the lease conformance suite). Only the
// fields the connector needs are decoded; the signature stays opaque.
type peerCredentialBody struct {
	CredentialVersion uint16 `cbor:"credential_version"`
	Issuer            string `cbor:"issuer"`
	SubjectPeerID     string `cbor:"subject_peer_id"`
	Organization      string `cbor:"organization"`
}

// PeerID decodes the enrolled credential's SubjectPeerID. The lease the
// relay issues binds this value; LeaseClient.Acquire verifies it.
func (i *Identity) PeerID() (string, error) {
	if i == nil || len(i.Credential) == 0 {
		return "", errors.New("dari: no enrolled credential")
	}
	sign1, err := DecodeCOSESign1(i.Credential)
	if err != nil {
		return "", fmt.Errorf("dari: decode credential: %w", err)
	}
	var body peerCredentialBody
	if err := UnmarshalCBOR(sign1.Payload, &body); err != nil {
		return "", fmt.Errorf("dari: decode credential body: %w", err)
	}
	if body.SubjectPeerID == "" {
		return "", errors.New("dari: credential carries no subject peer id")
	}
	return body.SubjectPeerID, nil
}

// Organization decodes the enrolled credential's organization binding.
func (i *Identity) Organization() (string, error) {
	if i == nil || len(i.Credential) == 0 {
		return "", errors.New("dari: no enrolled credential")
	}
	sign1, err := DecodeCOSESign1(i.Credential)
	if err != nil {
		return "", fmt.Errorf("dari: decode credential: %w", err)
	}
	var body peerCredentialBody
	if err := UnmarshalCBOR(sign1.Payload, &body); err != nil {
		return "", fmt.Errorf("dari: decode credential body: %w", err)
	}
	return body.Organization, nil
}

// LoadIdentityFromDisk reads the connector's enrolled identity from a pair of
// files. The private key file holds the raw 32-byte Ed25519 seed (not a PEM
// envelope; the connector is a minimal client and the relay only needs the
// raw bytes). The credential file holds the raw COSE-Sign1 CBOR returned by
// the issuer. Both files MUST be readable by the connector's runtime user
// but not world-readable; the loader creates them with 0600 permissions in
// the setup helper.
//
// Reasonable error returns let the caller decide whether to fail fast on
// missing files (no enrollment yet) or recover via a setup flow.
func LoadIdentityFromDisk(credentialPath, privateKeyPath string) (*Identity, error) {
	if credentialPath == "" || privateKeyPath == "" {
		return nil, errors.New("dari: credential and private-key paths are required")
	}
	cred, err := os.ReadFile(credentialPath)
	if err != nil {
		return nil, fmt.Errorf("dari: read credential %s: %w", credentialPath, err)
	}
	if len(cred) == 0 {
		return nil, fmt.Errorf("dari: credential file %s is empty", credentialPath)
	}
	key, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("dari: read private key %s: %w", privateKeyPath, err)
	}
	if len(key) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("dari: private key %s has %d bytes, want %d",
			privateKeyPath, len(key), ed25519.PrivateKeySize)
	}
	return &Identity{
		PrivateKey: ed25519.PrivateKey(append([]byte(nil), key...)),
		Credential: append([]byte(nil), cred...),
	}, nil
}

// writeLengthPrefixed writes a uint32 big-endian length followed by the value
// to h. Trailing-NULL domain separation is provided by the caller.
func writeLengthPrefixed(h interface{ Write([]byte) (int, error) }, value []byte) {
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(value)))
	h.Write(length[:])
	h.Write(value)
}

